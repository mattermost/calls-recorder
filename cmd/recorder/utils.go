package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

var (
	unpriviledgeUsersCloneSysctlPath = "/proc/sys/kernel/unprivileged_userns_clone"
	icePasswordRE                    = regexp.MustCompile(`ice-pwd:[\w|\+|/]+`)
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
