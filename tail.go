// Package tail implements behaviour of `tail -n 0 -F path`.
package tail

import (
	"context"
	"errors"
	"io"
	"os"
	"time"
)

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
	ctx         context.Context //nolint:containedctx // By design.
	log         Logger
	path        string
	pollDelay   time.Duration
	pollTimeout time.Duration
	f           *trackedFile
	next        *trackedFile
	lasterr     error
}

// Follow starts tracking the path using polling.
//
// If path already exists tracking begins from the end of the file.
//
// Supported path types: usual file, FIFO and symlink to usual or FIFO.
func Follow(ctx context.Context, log Logger, path string, options ...Option) *Tail {
	t := &Tail{
		ctx:         ctx,
		log:         log,
		path:        path,
		pollDelay:   DefaultPollDelay,
		pollTimeout: DefaultPollTimeout,
		f:           newTrackedFile(ctx, path),
		next:        nil,
		lasterr:     nil,
	}
	for _, option := range options {
		option.apply(t)
	}

	err := t.f.Open() //nolint:contextcheck // False positive.
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
	if errors.Is(t.lasterr, io.EOF) {
		return 0, t.lasterr
	}

	if len(p) == 0 {
		return 0, nil
	}

	var timeoutc <-chan time.Time
	if t.lasterr == nil {
		timeoutc = time.After(t.pollTimeout)
	}
	t.lasterr = nil

	// Open file for the first time, if Follow failed to open it.
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
		select {
		case <-time.After(t.pollDelay):
		case <-timeoutc:
			return unwrap(err)
		case <-t.ctx.Done():
			return io.EOF
		}
	}
	t.log.Printf("tail: %q has appeared;  following new file", t.path)
	return nil
}

func (t *Tail) openNext() (err error) {
	if t.next == nil && t.f.Detached() { //nolint:nestif // TODO
		t.next = newTrackedFile(t.ctx, t.path)
		err = unwrap(t.next.Open())
		if err != nil {
			t.log.Printf("tail: %q has become inaccessible: %s", t.path, err)
		} else {
			t.log.Printf("tail: %q has been replaced;  following new file", t.path)
		}
	} else if t.next != nil && !t.next.Opened() {
		err = unwrap(t.next.Open())
		if err == nil {
			t.log.Printf("tail: %q has appeared;  following new file", t.path)
		}
	}
	return err
}

func (t *Tail) read(timeoutc <-chan time.Time, p []byte) (int, error) {
	errOpen := t.openNext()

	n, err := t.f.Read(p)
	err = unwrap(err)
	if errors.Is(err, io.EOF) && t.next != nil && t.next.Opened() {
		t.f.Close()
		t.f, t.next = t.next, nil
		return t.read(timeoutc, p)
	}

	switch {
	case err == nil:
		return n, nil
	case errors.Is(err, os.ErrClosed):
		return 0, io.EOF
	case errors.Is(err, io.EOF):
		err = errOpen
	case isWouldBlock(err):
		// For non-blocking FIFO on macOS, treat EAGAIN/EWOULDBLOCK as temporary
		err = nil
	default:
		t.log.Printf("tail: error reading %q: %s", t.path, err)
	}

	if err == nil {
		timeoutc = nil
	}
	select {
	case <-time.After(t.pollDelay):
		return 0, nil
	case <-timeoutc:
		return 0, err
	case <-t.ctx.Done():
		return 0, io.EOF
	}
}
