package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/mattermost/calls-recorder/cmd/recorder/config"

	"github.com/chromedp/chromedp"
)

var (
	unpriviledgeUsersCloneSysctlPath = "/proc/sys/kernel/unprivileged_userns_clone"
	icePasswordRE                    = regexp.MustCompile(`ice-pwd:[\w|\+|/]+`)
	filenameSanitizationRE           = regexp.MustCompile(`[\\:*?\"<>|\n\s/]`)
)

func sanitizeConsoleLog(str string) string {
	return icePasswordRE.ReplaceAllString(str, "ice-pwd:XXX")
}

func slogDebugF(format string, args ...any) {
	slog.Debug(fmt.Sprintf(format, args...))
}

func slogReplaceAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.SourceKey {
		source := a.Value.Any().(*slog.Source)
		source.File = filepath.Base(source.File)
	}

	return a
}

func checkOSRequirements() error {
	// Verify that the required sysctl is set.
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile(unpriviledgeUsersCloneSysctlPath); err != nil {
			slog.Warn("failed to read sysctl", slog.String("err", err.Error()))
		} else if strings.TrimSpace(string(data)) != "1" {
			return fmt.Errorf("kernel.unprivileged_userns_clone should be enabled for the recording process to work")
		} else {
			slog.Debug("kernel.unprivileged_userns_clone is correctly set")
		}
	}

	return nil
}

func pollBrowserEvaluateExpr(ctx context.Context, expr string, interval, timeout time.Duration, stopCh chan struct{}) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var timeoutCh <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	for {
		select {
		case <-timeoutCh:
			return fmt.Errorf("timed out")
		case <-stopCh:
			return fmt.Errorf("stop signal received while polling")
		case <-ticker.C:
			var res bool
			if err := chromedp.Run(ctx,
				chromedp.Evaluate(expr, &res),
			); err != nil {
				slog.Error("failed to run chromedp", slog.String("err", err.Error()))
			} else if res {
				slog.Info("expression succeeded", slog.String("expr", expr))
				return nil
			}
			slog.Debug("expression failed", slog.String("expr", expr))
		}
	}
}

func getDataDir() string {
	if dir := os.Getenv("DATA_DIR"); dir != "" {
		return dir
	}
	return dataDir
}

func sanitizeFilename(name string) string {
	return filenameSanitizationRE.ReplaceAllString(name, "_")
}

func (rec *Recorder) getFilenameForCall(ext string) (string, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), httpRequestTimeout)
	defer cancelFn()

	url := fmt.Sprintf("%s/plugins/%s/bot/calls/%s/filename", rec.cfg.SiteURL, pluginID, rec.cfg.CallID)
	resp, err := rec.client.DoAPIRequest(ctx, http.MethodGet, url, "", "")
	if err != nil {
		return "", fmt.Errorf("failed to get filename: %w", err)
	}
	defer resp.Body.Close()

	var m map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return "", fmt.Errorf("failed to unmarshal filename: %w", err)
	}

	filename := sanitizeFilename(m["filename"])

	if filename == "" {
		return "", fmt.Errorf("invalid empty filename")
	}

	return filename + "." + ext, nil
}

type clientTransport struct {
	transport http.RoundTripper
}

func (ct *clientTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := ct.transport.RoundTrip(req)
	if err != nil {
		slog.Error("request failed with error",
			slog.String("err", err.Error()),
			slog.Any("url", req.URL),
			slog.String("method", req.Method),
		)
	} else if res != nil && res.StatusCode >= 300 {
		defer res.Body.Close()
		buf := &bytes.Buffer{}
		_, err := buf.ReadFrom(res.Body)
		if err != nil {
			slog.Error("failed to read response body",
				slog.String("err", err.Error()),
				slog.Any("url", req.URL),
				slog.String("method", req.Method),
			)
		}
		slog.Error("request failed with failure status code",
			slog.Int("code", res.StatusCode),
			slog.Any("url", req.URL),
			slog.String("method", req.Method),
			slog.String("data", buf.String()),
		)
		res.Body = io.NopCloser(buf)
	}

	return res, err
}

func getInsecureOrigins(siteURL string) ([]string, error) {
	if siteURL == "" {
		return nil, fmt.Errorf("invalid siteURL: should not be empty")
	}

	var insecureOrigins []string

	u, err := url.Parse(siteURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SiteURL: %w", err)
	}
	// If the given SiteURL is not running on a secure HTTPs connection, we add
	// it to the list of allowed insecure origins. We assume this would only
	// happen either on internal private networks or for development/testing purposes.
	if u.Scheme == "http" {
		insecureOrigins = []string{
			siteURL,
		}
	}

	if devMode := os.Getenv("DEV_MODE"); devMode == "true" {
		insecureOrigins = append(insecureOrigins, []string{
			"http://172.17.0.1:8065",
			"http://host.docker.internal:8065",
			"http://mm-server:8065",
			"http://host.minikube.internal:8065",
		}...)
	}

	return insecureOrigins, nil
}

func genChromiumOptions(cfg config.RecorderConfig) ([]chromedp.ExecAllocatorOption, []chromedp.ContextOption, error) {
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
		chromedp.Flag("window-size", fmt.Sprintf("%d,%d", cfg.Width, cfg.Height)),
		chromedp.Flag("display", fmt.Sprintf(":%d", displayID)),
	}

	contextOpts := []chromedp.ContextOption{
		chromedp.WithErrorf(slogDebugF),
	}

	if devMode := os.Getenv("DEV_MODE"); devMode == "true" {
		opts = append(opts, chromedp.NoSandbox)
		contextOpts = append(contextOpts, chromedp.WithLogf(slogDebugF))
		contextOpts = append(contextOpts, chromedp.WithDebugf(slogDebugF))
	}

	if insecureOrigins, err := getInsecureOrigins(cfg.SiteURL); err != nil {
		return nil, nil, fmt.Errorf("failed to get insecure origins: %w", err)
	} else if len(insecureOrigins) > 0 {
		slog.Info("adding insecure origins exceptions", slog.String("origins", strings.Join(insecureOrigins, ",")))
		opts = append(opts, chromedp.Flag("unsafely-treat-insecure-origin-as-secure", strings.Join(insecureOrigins, ",")))
	}

	return opts, contextOpts, nil
}
