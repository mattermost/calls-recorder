package main

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
)

var (
	icePasswordRE = regexp.MustCompile(`ice-pwd:[\w|\+|/]+`)
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
