package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const (
	pluginID     = "com.mattermost.calls"
	displayID    = 45
	readyTimeout = 15 * time.Second
	stopTimeout  = 10 * time.Second
)

type Recorder struct {
	cfg RecorderConfig

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
		chromedp.NoSandbox,

		// puppeteer default behavior
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("enable-automation", true),
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
		opts = append(opts, chromedp.Flag("unsafely-treat-insecure-origin-as-secure", "http://172.17.0.1:8065"))
		contextOpts = append(contextOpts, chromedp.WithLogf(log.Printf))
		contextOpts = append(contextOpts, chromedp.WithDebugf(log.Printf))
	}

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, _ := chromedp.NewContext(allocCtx, contextOpts...)

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventExceptionThrown:
			log.Printf("%s", ev.ExceptionDetails.Text)
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
			// TODO: improve this check
			if strings.Contains(str, "rtc connected") {
				close(rec.readyCh)
			}
			log.Printf(str)
		}
	})

	if err := chromedp.Run(ctx,
		chromedp.Navigate(recURL),
	); err != nil {
		return fmt.Errorf("failed to run chromedp: %w", err)
	}

	<-rec.stopCh

	log.Printf("stop received, shutting down browser")

	if err := chromedp.Run(ctx,
		chromedp.EvaluateAsDevTools("window.callsClient.disconnect();", nil),
	); err != nil {
		log.Printf("failed to run chromedp: %s", err)
	}

	tctx, cancelCtx := context.WithTimeout(ctx, 10*time.Second)
	// graceful cancel
	if err := chromedp.Cancel(tctx); err != nil {
		log.Printf("failed to cancel context: %s", err)
	}
	cancelCtx()

	close(rec.stoppedCh)

	return nil
}

func (rec *Recorder) runTranscoder(dst string) error {
	args := fmt.Sprintf(`-y -thread_queue_size 1024 -f pulse -i default -r %d -thread_queue_size 1024 -f x11grab -draw_mouse 0 -s %dx%d -i :%d -c:v h264 -preset fast -vf format=yuv420p -b:v %dk -b:a %dk -movflags +faststart %s`, rec.cfg.FrameRate, rec.cfg.Width, rec.cfg.Height, displayID, rec.cfg.VideoRate, rec.cfg.AudioRate, dst)
	log.Printf("running transcoder: %q", args)
	cmd := exec.Command("ffmpeg", strings.Split(args, " ")...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to pipe stderr: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start transcoder: %w", err)
	}

	rec.transcoder = cmd

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("transcoder: %s\n", scanner.Text())
		}
	}()

	return nil
}

func runDisplayServer(width, height int) (*exec.Cmd, error) {
	args := fmt.Sprintf(`:%d -screen 0 %dx%dx24 -dpi 96`, displayID, width, height)
	log.Printf("running display server: %q", args)
	cmd := exec.Command("Xvfb", strings.Split(args, " ")...)
	return cmd, cmd.Start()
}

func NewRecorder(cfg RecorderConfig) (*Recorder, error) {
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

	recURL := fmt.Sprintf("%s/plugins/%s/standalone/recording.html?call_id=%s&token=%s",
		rec.cfg.SiteURL, pluginID, rec.cfg.CallID, rec.cfg.AuthToken)

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

	// TODO (MM-48546): implement retry logic.

	if err := rec.uploadRecording(); err != nil {
		log.Printf("failed to upload recording: %s", err)
	}

	if err := os.Remove(rec.outPath); err != nil {
		log.Printf("failed to remove recording: %s", err)
	}

	return nil
}
