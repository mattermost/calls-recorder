package main

import (
	"regexp"
)

var (
	icePasswordRE = regexp.MustCompile(`ice-pwd:[\w|\+|/]+`)
)

func sanitizeConsoleLog(str string) string {
	return icePasswordRE.ReplaceAllString(str, "ice-pwd:XXX")
}
