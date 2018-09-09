package gitstatus

import "fmt"

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
