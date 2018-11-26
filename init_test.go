package tail

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/powerman/check"
)

var ( // nolint:gochecknoglobals
	testTimeFactor = floatGetenv("GO_TEST_TIME_FACTOR", 1.0)
	testSecond     = time.Duration(float64(time.Second) * testTimeFactor)
	pollDelay      = testSecond / 10
	pollTimeout    = testSecond / 2
)

func floatGetenv(name string, def float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(name), 64); err == nil {
		return v
	}
	return def
}

type testTail struct {
	t       *check.C
	f       *os.File
	path    string
	symlink string
	bufc    chan []byte
	errc    chan error
	created []string
	opened  []*os.File
	Cancel  context.CancelFunc
	*Tail
}

func newTestTail(t *check.C) *testTail {
	t.Helper()
	f, err := ioutil.TempFile("", "gotest")
	t.Nil(err)
	tail := &testTail{
		t:       t,
		f:       f,
		path:    f.Name(),
		bufc:    make(chan []byte),
		errc:    make(chan error), // must not be buffered to sync on errors
		created: []string{f.Name()},
		opened:  []*os.File{f},
	}
	return tail
}

func (tail *testTail) Run() {
	if tail.Tail != nil {
		panic("tail.Run() must be called only once")
	}
	ctx, cancel := context.WithCancel(context.Background())
	tail.Cancel = cancel
	options := []Option{PollDelay(pollDelay), PollTimeout(pollTimeout)}
	if tail.symlink == "" {
		tail.Tail = Follow(ctx, LoggerFunc(tail.t.Logf), tail.path, options...)
	} else {
		tail.Tail = Follow(ctx, LoggerFunc(tail.t.Logf), tail.symlink, options...)
	}
	go tail.reader()
}

func (tail *testTail) Close() {
	if tail.Tail == nil {
		panic("tail.Run() must be called before tail.Close()")
	}
	t := tail.t
	t.Helper()
	tail.Cancel()
WAIT_READER:
	for {
		select {
		case <-tail.bufc:
		case _, ok := <-tail.errc:
			if !ok {
				break WAIT_READER
			}
		}
	}
	for _, f := range tail.opened {
		t.Nil(f.Close())
	}
	for _, path := range tail.created {
		t.Nil(os.Remove(path))
	}
}

func (tail *testTail) reader() {
	for {
		buf := make([]byte, 8) // use small buffer to ease test large reads
		n, err := tail.Read(buf)
		switch {
		case err == io.EOF:
			tail.errc <- err
			close(tail.errc)
			return
		case err != nil:
			tail.errc <- err
		default:
			tail.bufc <- buf[:n]
		}
	}
}

func (tail *testTail) Want(timeout time.Duration, want string, wanterr error) {
	if tail.Tail == nil {
		panic("tail.Run() must be called before tail.Want()")
	}
	t := tail.t
	t.Helper()
	timeoutc := time.After(timeout)
	var s string
	for {
		select {
		case buf := <-tail.bufc:
			s += string(buf)
		case err := <-tail.errc:
			if err == nil {
				err = io.ErrClosedPipe
			}
			t.Equal(s, want)
			if perr, ok := err.(*os.PathError); ok {
				err = perr.Err
			}
			t.Err(err, wanterr)
			return
		case <-timeoutc:
			t.Equal(s, want)
			t.Nil(wanterr)
			return
		}
	}
}

func (tail *testTail) Remove() {
	if _, err := os.Stat(tail.path); os.IsNotExist(err) {
		panic("tail.Remove() or tail.Rename() must not be called before tail.Remove()")
	}
	t := tail.t
	t.Helper()
	t.Nil(os.Remove(tail.path))
	for i := range tail.created {
		if tail.created[i] == tail.path {
			tail.created[i] = tail.created[0]
			tail.created = tail.created[1:]
			break
		}
	}
}

func (tail *testTail) Rename() {
	if _, err := os.Stat(tail.path); os.IsNotExist(err) {
		panic("tail.Remove() or tail.Rename() must not be called before tail.Rename()")
	}
	t := tail.t
	t.Helper()
	path := tail.tempPath()
	t.Nil(os.Rename(tail.path, path))
	for i := range tail.created {
		if tail.created[i] == tail.path {
			tail.created[i] = path
			break
		}
	}
}

func (tail *testTail) Create() {
	if _, err := os.Stat(tail.path); !os.IsNotExist(err) {
		panic("tail.Remove() or tail.Rename() must be called before tail.Create()")
	}
	t := tail.t
	t.Helper()
	f, err := os.Create(tail.path)
	t.Nil(err)
	tail.f = f
	tail.created = append(tail.created, tail.path)
	tail.opened = append(tail.opened, f)
}

func (tail *testTail) CreateFIFO() {
	if _, err := os.Stat(tail.path); !os.IsNotExist(err) {
		panic("tail.Remove() or tail.Rename() must be called before tail.CreateFIFO()")
	}
	t := tail.t
	t.Helper()
	t.Nil(syscall.Mkfifo(tail.path, 0600))
	f, err := os.OpenFile(tail.path, os.O_RDWR, 0600)
	t.Nil(err)
	tail.f = f
	tail.created = append(tail.created, tail.path)
	tail.opened = append(tail.opened, f)
}

func (tail *testTail) CreateSymlink() {
	if _, err := os.Stat(tail.path); os.IsNotExist(err) {
		panic("tail.Remove() or tail.Rename() must not be called before tail.CreateSymlink()")
	}
	t := tail.t
	t.Helper()
	if tail.symlink == "" {
		tail.symlink = tail.tempPath()
	}
	t.Nil(os.Symlink(tail.path, tail.symlink))
	tail.created = append(tail.created, tail.symlink)
}

func (tail *testTail) RemoveSymlink() {
	if _, err := os.Stat(tail.symlink); os.IsNotExist(err) {
		panic("tail.RemoveSymlink() must not be called before tail.CreateSymlink()")
	}
	t := tail.t
	t.Helper()
	t.Nil(os.Remove(tail.symlink))
	for i := range tail.created {
		if tail.created[i] == tail.symlink {
			tail.created[i] = tail.created[0]
			tail.created = tail.created[1:]
			break
		}
	}
}

func (tail *testTail) Write(s string) {
	t := tail.t
	t.Helper()
	_, err := tail.f.Write([]byte(s))
	t.Nil(err)
}

func (tail *testTail) tempPath() string {
	t := tail.t
	t.Helper()
	f, err := ioutil.TempFile("", "gotest")
	t.Nil(err)
	t.Nil(f.Close())
	t.Nil(os.Remove(f.Name()))
	return f.Name()
}

func TestMain(m *testing.M) { check.TestMain(m) }
