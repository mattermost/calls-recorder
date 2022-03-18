package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const (
	width  = 1920
	height = 1080
)

type browserConfig struct {
	siteURL  string
	recURL   string
	username string
	password string
}

func runBrowser(cfg browserConfig, readyCh, stopCh chan struct{}) {
	display := ":45"

	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.DisableGPU,
		chromedp.NoSandbox,

		// puppeteer default behavior
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("enable-automation", true),
		// chromedp.Flag("excludeSwitches", "enable-automation"),
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
		chromedp.Flag("kiosk", true),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("autoplay-policy", "no-user-gesture-required"),
		chromedp.Flag("window-position", "0,0"),
		chromedp.Flag("window-size", fmt.Sprintf("%d,%d", width, height)),
		chromedp.Flag("display", display),
		// chromedp.Flag("force-device-scale-factor", "1.5"),

		// disable security
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("allow-running-insecure-content", true),
		chromedp.Flag("ignore-certificate-errors", true),
	}

	allocCtx, cancelAllocCtx := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelCtx := chromedp.NewContext(allocCtx)

	go func() {
		<-stopCh
		cancelCtx()
		cancelAllocCtx()
	}()

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			args := make([]string, 0, len(ev.Args))
			for _, arg := range ev.Args {
				var val interface{}
				err := json.Unmarshal(arg.Value, &val)
				if err != nil {
					continue
				}
				args = append(args, fmt.Sprint(val))
			}

			str := fmt.Sprintf("chrome console %s %s", ev.Type.String(), strings.Join(args, " "))
			// TODO: improve this check
			if strings.Contains(str, "rtc connected") {
				close(readyCh)
			}
			log.Printf(str)
		}
	})

	if err := chromedp.Run(ctx,
		// chromedp.EmulateViewport(1706, 960, chromedp.EmulateScale(1.5)),
		chromedp.Navigate(cfg.siteURL),
		chromedp.WaitVisible(`#loginId`),
		chromedp.SetValue(`#loginId`, cfg.username),
		chromedp.SetValue(`#loginPassword`, cfg.password),
		chromedp.Click(`#loginButton`),
		chromedp.WaitVisible(`#global-header`),
		chromedp.Navigate(cfg.recURL),
		chromedp.WaitVisible(`#calls-recording-view`),
	); err != nil {
		log.Printf(err.Error())
	}
}

func runRecorder(dst string) (*exec.Cmd, error) {
	args := fmt.Sprintf(`-y -thread_queue_size 1024 -f pulse -i default -r 25 -thread_queue_size 1024 -f x11grab -draw_mouse 0 -s %dx%d -i :45 -c:v h264 -preset fast -vf format=yuv420p -b:v 1500k -b:a 64k -movflags +faststart %s`, width, height, dst)
	log.Printf(args)
	rec := exec.Command("ffmpeg", strings.Split(args, " ")...)
	return rec, rec.Start()
}

func runDisplay(displayID, width, height int) (*exec.Cmd, error) {
	args := fmt.Sprintf(`:%d -screen 0 %dx%dx24 -dpi 96`, displayID, width, height)
	cmd := exec.Command("Xvfb", strings.Split(args, " ")...)
	return cmd, cmd.Start()
}

func main() {
	var wg sync.WaitGroup
	wg.Add(1)

	dis, err := runDisplay(45, width, height)
	if err != nil {
		log.Fatalf("failed to run display: %s", err.Error())
	}
	defer func() {
		if err := dis.Process.Kill(); err != nil {
			log.Printf(err.Error())
		}
	}()

	var cfg browserConfig
	cfg.siteURL = os.Getenv("SITE_URL")
	if cfg.siteURL == "" {
		log.Fatalf("site URL cannot be empty")
	}
	teamName := os.Getenv("TEAM_NAME")
	if teamName == "" {
		log.Fatalf("team name cannot be empty")
	}
	channelID := os.Getenv("CHANNEL_ID")
	if channelID == "" {
		log.Fatalf("channel id cannot be empty")
	}
	cfg.username = os.Getenv("USERNAME")
	if cfg.username == "" {
		log.Fatalf("username cannot be empty")
	}
	cfg.password = os.Getenv("PASSWORD")
	if cfg.password == "" {
		log.Fatalf("password cannot be empty")
	}

	cfg.recURL = fmt.Sprintf("%s/%s/com.mattermost.calls/recording/%s", cfg.siteURL, teamName, channelID)

	fmt.Println(cfg.recURL)

	stopCh := make(chan struct{})
	readyCh := make(chan struct{})
	go func() {
		defer wg.Done()
		runBrowser(cfg, readyCh, stopCh)
	}()

	<-readyCh

	log.Printf("ready to record")

	filename := fmt.Sprintf("%s-%s-%s.mp4", teamName, channelID, time.Now().UTC().Format("2006-01-02-15_04_05"))
	rec, err := runRecorder(filepath.Join("/recs", filename))
	if err != nil {
		log.Fatalf("failed to run recorder: %s", err.Error())
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Printf("got SIGTERM, exiting")

	if err := rec.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("failed to send signal: %s", err.Error())
	}
	out := rec.Wait()
	log.Printf("%+v", out)

	close(stopCh)

	wg.Wait()

	fmt.Println("done")
}
