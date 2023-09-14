package main

import (
	"os"
	"regexp"
)

var (
	icePasswordRE = regexp.MustCompile(`ice-pwd:[\w|\+|/]+`)
)

func sanitizeConsoleLog(str string) string {
	return icePasswordRE.ReplaceAllString(str, "ice-pwd:XXX")
}

func getDataDir() string {
	if dir := os.Getenv("DATA_DIR"); dir != "" {
		return dir
	}
	return dataDir
}
