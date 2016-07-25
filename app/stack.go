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
		stackTrace = stackTrace[1:]
	}

	if len(stackTrace) <= skip*2 {
		// stack trace would be empty; give full stack trace
		return []byte(strings.Join(stackTrace, "\n"))
	}

	stackTrace = stackTrace[skip*2:]

	return []byte(strings.Join(stackTrace, "\n"))
}
