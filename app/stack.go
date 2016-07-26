package app

import (
	"runtime/debug"
	"strings"
)

func buildStack(skip int) []byte {
	if skip < 0 {
		skip = 0
	}

	// skip ourselves, too
	skip += 2

	stackTrace := strings.Split(string(debug.Stack()), "\n")

	if strings.HasPrefix(stackTrace[0], "goroutine ") {
		stackTrace = append(stackTrace[:1], stackTrace[(skip*2+1):]...)
	} else {
		stackTrace = stackTrace[skip*2:]
	}

	return []byte(strings.Join(stackTrace, "\n"))
}
