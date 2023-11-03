package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"

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

func (rec *Recorder) getChannelForCall() (*model.Channel, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), httpRequestTimeout)
	defer cancelFn()

	url := fmt.Sprintf("%s/plugins/%s/bot/channels/%s", rec.cfg.SiteURL, pluginID, rec.cfg.CallID)
	resp, err := rec.client.DoAPIRequest(ctx, http.MethodGet, url, "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channel: %w", err)
	}
	defer resp.Body.Close()

	var channel *model.Channel
	if err := json.NewDecoder(resp.Body).Decode(&channel); err != nil {
		return nil, fmt.Errorf("failed to unmarshal channel: %w", err)
	}

	return channel, nil
}

func sanitizeFilename(name string) string {
	return filenameSanitizationRE.ReplaceAllString(name, "_")
}
