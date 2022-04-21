package tail

import (
	"errors"
	"os"
)

// Logger is an interface used to log tail state changes.
type Logger interface {
	Printf(format string, v ...interface{})
}

// The LoggerFunc type is an adapter to allow the use of ordinary
// function as a logger. If f is a function with the appropriate
// signature, LoggerFunc(f) is a Logger that calls f.
type LoggerFunc func(format string, v ...interface{})

// Printf implements Logger interface.
func (f LoggerFunc) Printf(format string, v ...interface{}) { f(format, v...) }

func unwrap(err error) error {
	var perr *os.PathError
	switch {
	case errors.As(err, &perr):
		return perr.Err
	default:
		return err
	}
}
