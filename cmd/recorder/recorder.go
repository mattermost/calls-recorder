package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mattermost/calls-recorder/cmd/recorder/config"

	"github.com/mattermost/mattermost/server/public/model"

	cruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const (
	pluginID                   = "com.mattermost.calls"
	displayID                  = 45
	readyTimeout               = 15 * time.Second
	stopTimeout                = 10 * time.Second
	maxUploadRetryAttempts     = 5
	uploadRetryAttemptWaitTime = 5 * time.Second
	connCheckInterval          = 1 * time.Second
)

type Recorder struct {
	cfg config.RecorderConfig

	readyCh   chan struct{}
	stopCh    chan struct{}
	stoppedCh chan error

	displayServer *exec.Cmd
	transcoder    *exec.Cmd

	client *model.Client4

	outPath string
}

func (rec *Recorder) runBrowser(recURL string) (rerr error) {
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
		chromedp.WithErrorf(slogDebugF),
	}
	if devMode := os.Getenv("DEV_MODE"); devMode == "true" {
		opts = append(opts, chromedp.Flag("unsafely-treat-insecure-origin-as-secure",
			"http://172.17.0.1:8065,http://host.docker.internal:8065,http://mm-server:8065,http://host.minikube.internal:8065"))
		opts = append(opts, chromedp.NoSandbox)
		contextOpts = append(contextOpts, chromedp.WithLogf(slogDebugF))
		contextOpts = append(contextOpts, chromedp.WithDebugf(slogDebugF))
	}

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, _ := chromedp.NewContext(allocCtx, contextOpts...)

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *cruntime.EventExceptionThrown:
			slog.Error("chrome exception", slog.String("err", ev.ExceptionDetails.Text))
			if ev.ExceptionDetails.Exception != nil {
				slog.Error("chrome exception", slog.String("err", ev.ExceptionDetails.Exception.Description))
			}
		case *cruntime.EventConsoleAPICalled:
			args := make([]string, 0, len(ev.Args))
			for _, arg := range ev.Args {
				var val interface{}
				var str string
				if len(arg.Value) > 0 {
					err := json.Unmarshal(arg.Value, &val)
					if err != nil {
						slog.Error("failed to unmarshal", slog.String("err", err.Error()))
						continue
					}
					str = fmt.Sprintf("%+v", val)
				} else {
					str = arg.Description
				}
				args = append(args, str)
			}

			str := fmt.Sprintf("chrome console %s %s", ev.Type.String(), strings.Join(args, " "))

			slog.Debug(sanitizeConsoleLog(str))
		}
	})

	defer func() {
		tctx, cancelCtx := context.WithTimeout(ctx, stopTimeout)
		// graceful cancel
		if err := chromedp.Cancel(tctx); err != nil {
			slog.Error("failed to cancel context", slog.String("err", err.Error()))
		}
		cancelCtx()
		rec.stoppedCh <- rerr
	}()

	if err := chromedp.Run(ctx, chromedp.Navigate(recURL)); err != nil {
		return fmt.Errorf("failed to run chromedp: %w", err)
	}

	ticker := time.NewTicker(connCheckInterval)
	defer ticker.Stop()

	var connected bool
	connectCheckExpr := "window.callsClient && window.callsClient.connected && !window.callsClient.closed"
	for {
		select {
		case <-rec.stopCh:
			slog.Info("stop signal received, shutting down browser")
			return nil
		case <-ticker.C:
			if err := chromedp.Run(ctx,
				chromedp.Evaluate(connectCheckExpr, &connected),
			); err != nil {
				slog.Error("failed to run chromedp", slog.String("err", err.Error()))
				continue
			}
			if !connected {
				slog.Debug("not connected to call yet")
				continue
			}

			slog.Debug("connected to call")
			close(rec.readyCh)
		}
		break
	}

	var disconnected bool
	disconnectCheckExpr := "!window.callsClient || window.callsClient.closed"
	for {
		select {
		case <-rec.stopCh:
			slog.Info("stop signal received, shutting down browser")
		case <-ticker.C:
			if err := chromedp.Run(ctx,
				chromedp.Evaluate(disconnectCheckExpr, &disconnected),
			); err != nil {
				slog.Error("failed to run chromedp", slog.String("err", err.Error()))
			}
			if disconnected {
				slog.Info("disconnected from call, shutting down")
				if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
					slog.Error("failed to send SIGTERM signal", slog.String("err", err.Error()))
				}
				return nil
			}
			continue
		}
		break
	}

	disconnectExpr := "window.callsClient.disconnect();"
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(disconnectExpr+disconnectCheckExpr, &disconnected),
	); err != nil {
		slog.Error("failed to run chromedp", slog.String("err", err.Error()))
	}
	if disconnected {
		slog.Info("disconnected from call successfully")
	} else {
		slog.Error("failed to disconnect")
	}

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
		return fmt.Errorf("failed to run transcoder command: %w", err)
	}

	rec.transcoder = cmd

	return nil
}

func runDisplayServer(width, height int) (*exec.Cmd, error) {
	args := fmt.Sprintf(`:%d -screen 0 %dx%dx24 -dpi 96 -nolisten tcp -nolisten unix`, displayID, width, height)
	return runCmd("Xvfb", args)
}

func NewRecorder(cfg config.RecorderConfig) (*Recorder, error) {
	if err := cfg.IsValid(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	client := model.NewAPIv4Client(cfg.SiteURL)
	client.SetToken(cfg.AuthToken)

	return &Recorder{
		cfg:       cfg,
		readyCh:   make(chan struct{}),
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan error),
		client:    client,
	}, nil
}

func (rec *Recorder) Start() error {
	if err := checkOSRequirements(); err != nil {
		return err
	}

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

	recURL := fmt.Sprintf("%s/plugins/%s/standalone/recording.html?call_id=%s&job_id=%s#%s",
		rec.cfg.SiteURL, pluginID, rec.cfg.CallID, rec.cfg.RecordingID, base64.URLEncoding.EncodeToString(data))

	go func() {
		if err := rec.runBrowser(recURL); err != nil {
			slog.Error("failed to run browser", slog.String("err", err.Error()))
		}
	}()

	select {
	case <-rec.readyCh:
	case <-time.After(readyTimeout):
		return fmt.Errorf("timed out waiting for ready event")
	}

	slog.Info("browser connected, ready to record")

	filename := fmt.Sprintf("%s-%s.mp4", rec.cfg.CallID, time.Now().UTC().Format("2006-01-02-15_04_05"))
	rec.outPath = filepath.Join("/recs", filename)
	err = rec.runTranscoder(rec.outPath)
	if err != nil {
		return fmt.Errorf("failed to run transcoder: %s", err)
	}

	if err := rec.ReportJobStarted(); err != nil {
		return fmt.Errorf("failed to report job started status: %w", err)
	}

	return nil
}

func (rec *Recorder) Stop() error {
	if rec.transcoder != nil {
		if err := rec.transcoder.Process.Signal(syscall.SIGTERM); err != nil {
			slog.Error("failed to send signal", slog.String("err", err.Error()))
		}
		if err := rec.transcoder.Wait(); err != nil {
			slog.Error("failed waiting for transcoder to exit", slog.String("err", err.Error()))
		}
		rec.transcoder = nil
	}

	close(rec.stopCh)

	var exitErr error
	select {
	case exitErr = <-rec.stoppedCh:
	case <-time.After(stopTimeout):
		exitErr = fmt.Errorf("timed out waiting for stopped event")
	}

	if rec.displayServer != nil {
		if err := rec.displayServer.Process.Signal(syscall.SIGTERM); err != nil {
			slog.Error("failed to stop display process", slog.String("err", err.Error()))
		}
		if err := rec.displayServer.Wait(); err != nil {
			slog.Error("failed waiting for display server to exit", slog.String("err", err.Error()))
		}
		rec.displayServer = nil
	}

	if exitErr != nil {
		return exitErr
	}

	// TODO (MM-48546): implement better retry logic.
	var attempt int
	for {
		err := rec.uploadRecording()
		if err == nil {
			slog.Info("recording uploaded successfully")
			break
		}

		if attempt == maxUploadRetryAttempts {
			return fmt.Errorf("max retry attempts reached, exiting")
		}

		attempt++
		slog.Info("failed to upload recording", slog.String("err", err.Error()))
		slog.Info("retrying", slog.Duration("wait_time", uploadRetryAttemptWaitTime))
		time.Sleep(uploadRetryAttemptWaitTime)
	}

	if err := os.Remove(rec.outPath); err != nil {
		slog.Error("failed to remove recording", slog.String("err", err.Error()))
	}

	return nil
}
