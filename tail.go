// Package tail implements behaviour of `tail -n 0 -F path`.
package tail

import (
	"context"
	"io"
	"os"
	"time"
)

var (
	pollDelay   = 200 * time.Millisecond // delay between polling to save CPU
	pollTimeout = time.Second            // how long to wait before returning os.ErrNotExist
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
	switch err := err.(type) {
	case *os.PathError:
		return err.Err
	default:
		return err
	}
}

// Tail is an io.Reader with `tail -n 0 -F path` behaviour.
//
// Unlike `tail` it does track renamed/removed file contents up to the
// moment new file will be created with original name - this ensure no
// data will be lost in case log rotation is done by external tool (not
// the one which write to log file) and thus original log file may be
// appended between rename/removal and reopening.
//
// Unlike `tail` it does not support file truncation. While this can't
// work reliable, truncate support may be added in the future.
type Tail struct {
	ctx         context.Context
	log         Logger
	path        string
	f           *trackedFile
	lasterr     error
	logReplaced bool
}

// Follow starts tracking the path using polling.
//
// If path already exists tracking begins from the end of the file.
//
// Supported path types: usual file, FIFO and symlink to usual or FIFO.
func Follow(ctx context.Context, log Logger, path string) *Tail {
	t := &Tail{
		ctx:  ctx,
		log:  log,
		path: path,
		f:    newTrackedFile(ctx, path),
	}

	err := t.f.Open()
	if err == nil && t.f.Usual() {
		_, err = t.f.Seek(0, io.SeekEnd)
		if err != nil {
			t.f.Close()
		}
	}
	if err != nil {
		t.log.Printf("tail: cannot open %q for reading: %s", t.path, unwrap(err))
	}

	return t
}

// Read returns appended data as the file grows, keep trying to open a
// file if it is inaccessible, and continue reading from beginning of the
// file when it will became accessible (e.g. after log rotation).
//
// Returned data is not guaranteed to contain full lines of text.
//
// If Read returns any error except io.EOF, then following Read will
// return either some data or io.EOF.
//
// Read may return 0, nil only if len(p) == 0.
//
// Read will return io.EOF only after cancelling ctx.
// Following Read will always return io.EOF.
//
// Read must not be called from simultaneous goroutines.
func (t *Tail) Read(p []byte) (int, error) {
	if t.lasterr == io.EOF {
		return 0, t.lasterr
	}

	if len(p) == 0 {
		return 0, nil
	}

	var timeoutc <-chan time.Time
	if t.lasterr == nil {
		timeoutc = time.After(pollTimeout)
	}
	t.lasterr = nil

	// Open file for the first time, if Follow failed to open it.
	// Reopen file, if it become inaccessible or was replaced.
	if !t.f.Opened() {
		t.lasterr = t.tryOpen(timeoutc)
		if t.lasterr != nil {
			return 0, t.lasterr
		}
	}

	var n int
	for n == 0 && t.lasterr == nil {
		n, t.lasterr = t.read(timeoutc, p)
	}
	return n, t.lasterr
}

func (t *Tail) tryOpen(timeoutc <-chan time.Time) error {
	for err := t.f.Open(); err != nil; err = t.f.Open() {
		err = unwrap(err)
		if t.logReplaced {
			t.logReplaced = false
			t.log.Printf("tail: %q has become inaccessible: %s", t.path, err)
		}

		select {
		case <-time.After(pollDelay):
		case <-timeoutc:
			return err
		case <-t.ctx.Done():
			return io.EOF
		}
	}

	if t.logReplaced {
		t.logReplaced = false
		t.log.Printf("tail: %q has been replaced;  following new file", t.path)
	} else {
		t.log.Printf("tail: %q has appeared;  following new file", t.path)
	}
	return nil
}

func (t *Tail) read(timeoutc <-chan time.Time, p []byte) (int, error) {
	n, err := t.f.Read(p)
	if err == io.EOF {
		if err2 := t.f.Tracking(); err2 != nil {
			// Read again in case file was appended
			// and rotated between Read and Tracking.
			n, err = t.f.Read(p)
			if err == io.EOF {
				t.f.Close()
				if err2 == errFileReplaced {
					t.logReplaced = true
				} else {
					t.log.Printf("tail: %q has become inaccessible: %s", t.path, unwrap(err2))
				}
				return t.Read(p)
			}
		}
	}
	err = unwrap(err)

	switch err {
	case nil:
		return n, nil
	case os.ErrClosed:
		return 0, io.EOF
	case io.EOF:
		timeoutc = nil
	}

	select {
	case <-time.After(pollDelay):
		return 0, nil
	case <-timeoutc:
		t.log.Printf("tail: error reading %q: %s", t.path, err)
		return 0, err
	case <-t.ctx.Done():
		return 0, io.EOF
	}
}
