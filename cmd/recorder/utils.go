package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

var (
	icePasswordRE = regexp.MustCompile(`ice-pwd:[\w|\+|/]+`)
	tokenRE       = regexp.MustCompile(`\\"token\\":[\\|"|\s|\w]+`)
	bearerRE      = regexp.MustCompile(`BEARER \w+`)
	hashRE        = regexp.MustCompile(`#\w+`)
)

func sanitizeLog(str string) string {
	str = icePasswordRE.ReplaceAllString(str, "ice-pwd:XXX")
	str = bearerRE.ReplaceAllString(str, "BEARER XXX")
	str = hashRE.ReplaceAllString(str, "#XXX")
	str = tokenRE.ReplaceAllString(str, `\"token\":\"XXX\"`)
	return str
}

func sanitizedPrintf(str string, args ...any) {
	str = fmt.Sprintf(str, args...)

	// Ignoring stylesheet related logs which can be extremely heavy.
	if strings.Contains(str, "CSS.styleSheetAdded") {
		return
	}

	log.Printf(sanitizeLog(str))
}
