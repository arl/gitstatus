package gitstatus

import (
	"fmt"
	"os/exec"
	"strings"
)

type wrappedErr struct {
	cause error
	msg   string
}

func (w *wrappedErr) Error() string { return w.msg + ": " + w.cause.Error() }

func wrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return &wrappedErr{
		cause: err,
		msg:   message,
	}
}

func wrapErrorf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return &wrappedErr{
		cause: err,
		msg:   fmt.Sprintf(format, args...),
	}
}

// errCmd creates an error wrapping the err, with details extracted from cmd.
func errCmd(err error, cmd *exec.Cmd) error {
	return wrapErrorf(err, `exec %s "%v"`, cmd.Path, strings.Join(cmd.Args, " "))
}
