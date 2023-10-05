package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
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

func getDataDir() string {
	if dir := os.Getenv("DATA_DIR"); dir != "" {
		return dir
	}
	return dataDir
}
