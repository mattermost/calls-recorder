package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/time/rate"

	"github.com/mattermost/calls-recorder/cmd/recorder/config"

	"github.com/mattermost/mattermost/server/public/model"

	cruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const (
	pluginID                     = "com.mattermost.calls"
	displayID                    = 45
	readyTimeout                 = 20 * time.Second
	stopTimeout                  = 10 * time.Second
	connCheckInterval            = 1 * time.Second
	initCheckInterval            = 1 * time.Second
	initCheckTimeout             = 5 * time.Second
	dataDir                      = "/data"
	transcoderStartTimeout       = 5 * time.Second
	transcoderStatsPeriod        = 100 * time.Millisecond
	transcoderProgressSocketPath = "/tmp/progress.sock"
	transcoderProgressBufferSize = 8192
	transcoderProgressLogFreq    = 2 * time.Second
)

type Recorder struct {
	cfg config.RecorderConfig

	// browser
	readyCh   chan struct{}
	stopCh    chan struct{}
	stoppedCh chan error

	// display server
	displayServer *exec.Cmd

	// transcoder
	transcoder          *exec.Cmd
	transcoderStoppedCh chan struct{}

	client *model.Client4

	outPath string
}

func (rec *Recorder) runBrowser(recURL string) (rerr error) {
	opts, contextOpts, err := genChromiumOptions(rec.cfg)
	if err != nil {
		return fmt.Errorf("failed to generate Chromium options: %w", err)
	}

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)

	var ctx context.Context
	cleanup := func() {
		tctx, cancelCtx := context.WithTimeout(ctx, stopTimeout)
		defer cancelCtx()
		// graceful cancel
		if err := chromedp.Cancel(tctx); err != nil {
			slog.Error("failed to cancel context", slog.String("err", err.Error()))
		}
	}

	defer func() {
		cleanup()
		rec.stoppedCh <- rerr
	}()

	for {
		select {
		case <-rec.stopCh:
			return fmt.Errorf("stop signal received while initializing client")
		default:
		}

		var cancel func()

		ctx, cancel = chromedp.NewContext(allocCtx, contextOpts...)
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

		if err := chromedp.Run(ctx, chromedp.Navigate(recURL)); err != nil {
			slog.Error("failed to run chromedp", slog.String("err", err.Error()))
			cancel()
			// If we don't event get to navigate to the URL then there's no point in
			// evaluating expressions. We simply wait for a second and try from
			// scratch.
			time.Sleep(time.Second)
			continue
		}

		// We poll until the client is initialized. In case of timeout we
		// re-initialize the browser again.
		if err := pollBrowserEvaluateExpr(ctx, `Boolean(window.callsClient)`, initCheckInterval, initCheckTimeout, rec.stopCh); err != nil {
			slog.Error("failed to poll for client initialization", slog.String("err", err.Error()))
			cancel()
		} else {
			// Client initialized, exiting the loop.
			break
		}
	}

	// Client has been initialized at this point, we move on to waiting until connected.
	connectCheckExpr := "Boolean(window.callsClient) && Boolean(window.callsClient.connected) && Boolean(!window.callsClient.closed)"
	if err := pollBrowserEvaluateExpr(ctx, connectCheckExpr, connCheckInterval, 0, rec.stopCh); err != nil {
		return fmt.Errorf("connectivity check failed: %w", err)
	}

	slog.Info("client connected to call")
	close(rec.readyCh)

	// Client connected, we poll until either we get the stop signal or client
	// disconnects on its own.
	disconnectCheckExpr := "Boolean(!window.callsClient) || Boolean(window.callsClient.closed)"
	if err := pollBrowserEvaluateExpr(ctx, disconnectCheckExpr, connCheckInterval*2, 0, rec.stopCh); err != nil {
		slog.Error("disconnect check failed", slog.String("err", err.Error()))

		// We must have received the stop signal so we attempt a clean disconnect.
		var disconnected bool
		disconnectExpr := "window.callsClient.disconnect();"
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(disconnectExpr+disconnectCheckExpr, &disconnected),
		); err != nil {
			slog.Error("failed to run chromedp", slog.String("err", err.Error()))
		} else if disconnected {
			slog.Info("disconnected from call successfully")
		} else {
			slog.Error("failed to disconnect")
		}

		return nil
	}

	// Client disconnected on its own so we self shutdown.
	slog.Info("disconnected from call, shutting down")
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		slog.Error("failed to send SIGTERM signal", slog.String("err", err.Error()))
	}

	return nil
}

func (rec *Recorder) runTranscoder(dst string) error {
	ln, err := net.Listen("unix", transcoderProgressSocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on progress socket: %w", err)
	}

	slog.Debug("listening on progress socket", slog.String("addr", ln.Addr().String()))

	startedCh := make(chan struct{})
	go func() {
		defer func() {
			if err := ln.Close(); err != nil {
				slog.Error("failed to close listener", slog.String("err", err.Error()))
			}
			close(rec.transcoderStoppedCh)
		}()

		conn, err := ln.Accept()
		if err != nil {
			slog.Error("failed to accept connection on progress socket", slog.String("err", err.Error()))
			return
		}

		slog.Debug("accepted connection on progress socket")

		var once sync.Once
		limiter := rate.NewLimiter(rate.Every(transcoderProgressLogFreq), 1)
		buf := make([]byte, transcoderProgressBufferSize)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					slog.Error("failed to read from conn", slog.String("err", err.Error()))
				}
				break
			}

			once.Do(func() {
				close(startedCh)
			})

			if limiter.Allow() {
				slog.Debug(fmt.Sprintf("ffmpeg progress:\n%s\n", string(buf[:n])))
			}
		}
	}()

	args := fmt.Sprintf(`-nostats -stats_period %0.2f -progress unix://%s -y -thread_queue_size 4096 -f alsa -i default -r %d -thread_queue_size 4096 -f x11grab -draw_mouse 0 -s %dx%d -i :%d -c:v h264 -preset %s -vf format=yuv420p -b:v %dk -b:a %dk -movflags +faststart %s`,
		transcoderStatsPeriod.Seconds(),
		ln.Addr(),
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

	select {
	case <-startedCh:
	case <-time.After(transcoderStartTimeout):
		return fmt.Errorf("timed out waiting for transcoder to start")
	}

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
	client.HTTPClient = &http.Client{
		Transport: &clientTransport{
			transport: http.DefaultTransport,
		},
	}

	return &Recorder{
		cfg:                 cfg,
		readyCh:             make(chan struct{}),
		stopCh:              make(chan struct{}),
		stoppedCh:           make(chan error),
		transcoderStoppedCh: make(chan struct{}),
		client:              client,
	}, nil
}

func (rec *Recorder) Start() error {
	if err := checkOSRequirements(); err != nil {
		return err
	}

	filename, err := rec.getFilenameForCall("mp4")
	if err != nil {
		return fmt.Errorf("failed to get filename for call: %w", err)
	}
	rec.outPath = filepath.Join(getDataDir(), filename)

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

	err = rec.runTranscoder(rec.outPath)
	if err != nil {
		return fmt.Errorf("failed to run transcoder: %s", err)
	}

	slog.Info("transcoder started")
	if err := rec.ReportJobStarted(); err != nil {
		return fmt.Errorf("failed to report job started status: %w", err)
	}

	return nil
}

func (rec *Recorder) Stop() error {
	if rec.transcoder != nil {
		slog.Info("stopping transcoder")
		if err := rec.transcoder.Process.Signal(syscall.SIGTERM); err != nil {
			slog.Error("failed to send signal", slog.String("err", err.Error()))
		}
		if err := rec.transcoder.Wait(); err != nil {
			slog.Error("failed waiting for transcoder to exit", slog.String("err", err.Error()))
		}
		<-rec.transcoderStoppedCh
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
		slog.Info("stopping display server")
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

	if err := rec.publishRecording(); err != nil {
		return fmt.Errorf("failed to publish recording: %w", err)
	}

	slog.Debug("upload successful, removing file", slog.String("outpath", rec.outPath))
	if err := os.Remove(rec.outPath); err != nil {
		slog.Error("failed to remove recording", slog.String("err", err.Error()))
	}

	return nil
}
