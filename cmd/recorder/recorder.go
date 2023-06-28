package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mattermost/calls-recorder/cmd/recorder/config"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const (
	pluginID                   = "com.mattermost.calls"
	displayID                  = 45
	readyTimeout               = 15 * time.Second
	stopTimeout                = 10 * time.Second
	maxUploadRetryAttempts     = 5
	uploadRetryAttemptWaitTime = 5 * time.Second
	initPollInterval           = 250 * time.Millisecond
)

type Recorder struct {
	cfg config.RecorderConfig

	readyCh   chan struct{}
	stopCh    chan struct{}
	stoppedCh chan struct{}

	displayServer *exec.Cmd
	transcoder    *exec.Cmd

	outPath string
}

func (rec *Recorder) runBrowser(recURL string) error {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.DisableGPU,

		// puppeteer default behavior
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("disable-client-side-phishing-detection", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-features", "site-per-process,TranslateUI,BlinkGenPropertyTrees"),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-prompt-on-repost", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),
		chromedp.Flag("password-store", "basic"),
		chromedp.Flag("use-mock-keychain", true),
		chromedp.Flag("use-fake-ui-for-media-stream", true),
		chromedp.Flag("use-fake-device-for-media-stream", true),

		// custom args
		chromedp.Flag("incognito", true),
		chromedp.Flag("kiosk", true),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("autoplay-policy", "no-user-gesture-required"),
		chromedp.Flag("window-position", "0,0"),
		chromedp.Flag("window-size", fmt.Sprintf("%d,%d", rec.cfg.Width, rec.cfg.Height)),
		chromedp.Flag("display", fmt.Sprintf(":%d", displayID)),
	}

	contextOpts := []chromedp.ContextOption{
		chromedp.WithErrorf(log.Printf),
	}
	if devMode := os.Getenv("DEV_MODE"); devMode == "true" {
		opts = append(opts, chromedp.Flag("unsafely-treat-insecure-origin-as-secure",
			"http://172.17.0.1:8065,http://host.docker.internal:8065,http://mm-server:8065"))
		opts = append(opts, chromedp.NoSandbox)
		contextOpts = append(contextOpts, chromedp.WithLogf(log.Printf))
		contextOpts = append(contextOpts, chromedp.WithDebugf(log.Printf))
	}

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, _ := chromedp.NewContext(allocCtx, contextOpts...)

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventExceptionThrown:
			log.Printf("chrome exception: %s", ev.ExceptionDetails.Text)
			if ev.ExceptionDetails.Exception != nil {
				log.Printf("chrome exception: %s", ev.ExceptionDetails.Exception.Description)
			}
		case *runtime.EventConsoleAPICalled:
			args := make([]string, 0, len(ev.Args))
			for _, arg := range ev.Args {
				var val interface{}
				var str string
				if len(arg.Value) > 0 {
					err := json.Unmarshal(arg.Value, &val)
					if err != nil {
						log.Printf("failed to unmarshal: %s", err)
						continue
					}
					str = fmt.Sprintf("%+v", val)
				} else {
					str = arg.Description
				}
				args = append(args, str)
			}

			str := fmt.Sprintf("chrome console %s %s", ev.Type.String(), strings.Join(args, " "))
			log.Printf(str)
		}
	})

	var connected bool
	connectCheckExpr := "window.callsClient && window.callsClient.connected && !window.callsClient.closed"
	if err := chromedp.Run(ctx,
		chromedp.Navigate(recURL),
		chromedp.Poll(connectCheckExpr, &connected, chromedp.WithPollingInterval(initPollInterval)),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if connected {
				log.Printf("connected to call")
				close(rec.readyCh)
				return nil
			}
			return fmt.Errorf("connectivity check failed")
		}),
	); err != nil {
		return fmt.Errorf("failed to run chromedp: %w", err)
	}

	<-rec.stopCh

	log.Printf("stop received, shutting down browser")

	var disconnected bool
	disconnectCheckExpr := "window.callsClient.disconnect(); !window.callsClient || window.callsClient.closed"
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(disconnectCheckExpr, &disconnected),
	); err != nil {
		log.Printf("failed to run chromedp: %s", err)
	}

	if disconnected {
		log.Printf("disconnected from call successfully")
	} else {
		log.Printf("failed to disconnect")
	}

	tctx, cancelCtx := context.WithTimeout(ctx, stopTimeout)
	// graceful cancel
	if err := chromedp.Cancel(tctx); err != nil {
		log.Printf("failed to cancel context: %s", err)
	}
	cancelCtx()

	close(rec.stoppedCh)

	return nil
}

func (rec *Recorder) runTranscoder(dst string) error {
	args := fmt.Sprintf(`-y -thread_queue_size 4096 -f alsa -i default -r %d -thread_queue_size 4096 -f x11grab -draw_mouse 0 -s %dx%d -i :%d -c:v h264 -preset %s -vf format=yuv420p -b:v %dk -b:a %dk -movflags +faststart %s`,
		rec.cfg.FrameRate,
		rec.cfg.Width,
		rec.cfg.Height,
		displayID,
		rec.cfg.VideoPreset,
		rec.cfg.VideoRate,
		rec.cfg.AudioRate,
		dst,
	)

	cmd, err := runCmd("ffmpeg", args)
	if err != nil {
		log.Fatal(err)
	}

	rec.transcoder = cmd

	return nil
}

func runDisplayServer(width, height int) (*exec.Cmd, error) {
	args := fmt.Sprintf(`:%d -screen 0 %dx%dx24 -dpi 96`, displayID, width, height)
	return runCmd("Xvfb", args)
}

func NewRecorder(cfg config.RecorderConfig) (*Recorder, error) {
	if err := cfg.IsValid(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &Recorder{
		cfg:       cfg,
		readyCh:   make(chan struct{}),
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}, nil
}

func (rec *Recorder) Start() error {
	var err error
	rec.displayServer, err = runDisplayServer(rec.cfg.Width, rec.cfg.Height)
	if err != nil {
		return fmt.Errorf("failed to run display server: %s", err)
	}

	data, err := json.Marshal(map[string]string{
		"token": rec.cfg.AuthToken,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal data: %s", err)
	}

	recURL := fmt.Sprintf("%s/plugins/%s/standalone/recording.html?call_id=%s&context_id=%s#%s",
		rec.cfg.SiteURL, pluginID, rec.cfg.CallID, rec.cfg.RecordingID, base64.URLEncoding.EncodeToString(data))

	go func() {
		if err := rec.runBrowser(recURL); err != nil {
			log.Printf("failed to run browser: %s", err)
		}
	}()

	select {
	case <-rec.readyCh:
	case <-time.After(readyTimeout):
		return fmt.Errorf("timed out waiting for ready event")
	}

	log.Printf("browser connected, ready to record")

	filename := fmt.Sprintf("%s-%s.mp4", rec.cfg.CallID, time.Now().UTC().Format("2006-01-02-15_04_05"))
	rec.outPath = filepath.Join("/recs", filename)
	err = rec.runTranscoder(rec.outPath)
	if err != nil {
		return fmt.Errorf("failed to run transcoder: %s", err)
	}

	return nil
}

func (rec *Recorder) Stop() error {
	if err := rec.transcoder.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("failed to send signal: %s", err.Error())
	}
	if err := rec.transcoder.Wait(); err != nil {
		log.Printf("failed waiting for transcoder to exit: %s", err)
	}

	close(rec.stopCh)

	select {
	case <-rec.stoppedCh:
	case <-time.After(stopTimeout):
		return fmt.Errorf("timed out waiting for stopped event")
	}

	if err := rec.displayServer.Process.Kill(); err != nil {
		log.Printf("failed to stop display process: %s", err)
	}

	// TODO (MM-48546): implement better retry logic.
	var attempt int
	for {
		err := rec.uploadRecording()
		if err == nil {
			log.Printf("recording uploaded successfully")
			break
		}

		if attempt == maxUploadRetryAttempts {
			return fmt.Errorf("max retry attempts reached, exiting")
		}

		attempt++
		log.Printf("failed to upload recording: %s", err)
		log.Printf("retrying in %s", uploadRetryAttemptWaitTime)
		time.Sleep(uploadRetryAttemptWaitTime)
	}

	if err := os.Remove(rec.outPath); err != nil {
		log.Printf("failed to remove recording: %s", err)
	}

	return nil
}
