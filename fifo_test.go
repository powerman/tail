//go:build !windows && debug_fifo

// Tests to demonstrate differences in FIFO behavior between Linux and macOS.
// These tests don't "test" anything and don't depend on the tail library.
package tail_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const step = 200 * time.Millisecond

func mkfifoTest(path string) error {
	return syscall.Mkfifo(path, 0o600)
}

func createTempFIFO(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "fifo_test_*.fifo")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	f.Close()
	os.Remove(path)

	err = mkfifoTest(path)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.Remove(path)
	})

	return path
}

func logWithTime(t *testing.T, format string, args ...any) {
	t.Helper()
	now := time.Now().Round(100 * time.Millisecond).Format("15:04:05.0")
	args = append([]any{now}, args...)
	fmt.Printf("  [%s] "+format+"\n", args...)
}

func write(t *testing.T, path string, id string, messages ...string) *os.File {
	t.Helper()
	logWithTime(t, "%s: Opening FIFO for writing: %s", id, filepath.Base(path))
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		logWithTime(t, "%s: Error opening FIFO for writing: %s", id, err)
		return f
	}
	logWithTime(t, "%s: Opened FIFO for writing", id)
	for _, msg := range messages {
		time.Sleep(step)
		f.SetWriteDeadline(time.Now().Add(step / 2))
		logWithTime(t, "%s: %q", id, msg)
		_, err = f.WriteString(msg)
		if err != nil {
			logWithTime(t, "%s: Write error: %s", id, err)
			return f
		}
	}
	return f
}

func writeAndClose(t *testing.T, path string, id string, messages ...string) {
	t.Helper()
	f := write(t, path, id, messages...)
	logWithTime(t, "%s: Closing FIFO writer", id)
	f.Close()
}

func openFIFOForReadNonblock(path string) (*os.File, error) {
	// Open the FIFO in read-write non-blocking mode:
	// - O_RDWR ensures we never get EOF when reading because at least one writer is present.
	// - O_NONBLOCK is required to have Close() interrupt the Read() call.
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func read(ctx context.Context, t *testing.T, path string, done chan<- struct{}) {
	t.Helper()
	defer close(done)
	const id = "R "
	logWithTime(t, "%s: Opening FIFO for reading: %s", id, filepath.Base(path))
	f, err := openFIFOForReadNonblock(path)
	if err != nil {
		logWithTime(t, "%s: Error opening FIFO for reading: %s", id, err)
		return
	}
	logWithTime(t, "%s: Opened FIFO for reading in non-blocking mode", id)

	go func() {
		<-ctx.Done()
		logWithTime(t, "%s: Context done, closing FIFO reader", id)
		f.Close()
	}()

	buf := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			logWithTime(t, "%s: Read interrupted by context", id)
			return
		default:
			n, err := f.Read(buf)
			switch {
			case errors.Is(err, os.ErrClosed):
				logWithTime(t, "%s: FIFO reader closed", id)
				return
			case os.IsTimeout(err):
			case errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK):
				time.Sleep(step / 4)
			case errors.Is(err, io.EOF):
				logWithTime(t, "%s: Got EOF on read", id)
				time.Sleep(step / 4)
			case err != nil:
				logWithTime(t, "%s: Read error: %s", id, err)
				return
			default:
				data := string(buf[:n])
				logWithTime(t, "%s: %q", id, data)
			}
		}
	}
}

func setup(t *testing.T) (context.Context, string, chan struct{}) {
	t.Helper()
	logWithTime(t, "Started: "+t.Name())
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	path := createTempFIFO(t)

	t.Cleanup(func() {
		cancel()
		logWithTime(t, "Finished")
		<-done
	})

	return ctx, path, done
}

func TestFIFO_OnlyReader(t *testing.T) {
	ctx, path, done := setup(t)

	go read(ctx, t, path, done)
	time.Sleep(2 * step)
}

func TestFIFO_WriterWithoutReader(t *testing.T) {
	_, path, done := setup(t)
	close(done)

	go write(t, path, "W1", "message1\n")
	time.Sleep(2 * step)
}

func TestFIFO_WriterFirst(t *testing.T) {
	ctx, path, done := setup(t)

	go write(t, path, "W1", "message1\n")
	time.Sleep(step)
	go read(ctx, t, path, done)
	time.Sleep(2 * step)
}

func TestFIFO_ReaderFirst(t *testing.T) {
	ctx, path, done := setup(t)

	go read(ctx, t, path, done)
	time.Sleep(step)
	go write(t, path, "W1", "message1\n")
	time.Sleep(2 * step)
}

func TestFIFO_TwoWritersBeforeReader(t *testing.T) {
	ctx, path, done := setup(t)

	go write(t, path, "W1", "writer1\n")
	time.Sleep(step / 2)
	go write(t, path, "W2", "writer2\n")
	time.Sleep(step)
	go read(ctx, t, path, done)
	time.Sleep(2 * step)
}

func TestFIFO_WritersAroundReader(t *testing.T) {
	ctx, path, done := setup(t)

	go write(t, path, "W1", "early_writer\n")
	time.Sleep(step)
	go read(ctx, t, path, done)
	time.Sleep(2 * step)
	go write(t, path, "W2", "late_writer\n")
	time.Sleep(2 * step)
}

func TestFIFO_TwoWritersAfterReader(t *testing.T) {
	ctx, path, done := setup(t)

	go read(ctx, t, path, done)
	time.Sleep(step)
	go write(t, path, "W1", "writer1\n")
	time.Sleep(2 * step)
	go write(t, path, "W2", "writer2\n")
	time.Sleep(2 * step)
}

func TestFIFO_OneWriterCloses(t *testing.T) {
	ctx, path, done := setup(t)

	go read(ctx, t, path, done)
	time.Sleep(step)
	go writeAndClose(t, path, "W1", "msg1\n", "msg2\n", "msg3\n")
	time.Sleep(4 * step)
}

func TestFIFO_TwoWritersOneCloses(t *testing.T) {
	ctx, path, done := setup(t)

	go read(ctx, t, path, done)
	time.Sleep(step)
	go write(t, path, "W1", "persistent_writer\n")
	time.Sleep(2 * step)
	go writeAndClose(t, path, "W2", "closing_writer\n")
	time.Sleep(2 * step)
}

func TestFIFO_TwoWritersBothClose(t *testing.T) {
	ctx, path, done := setup(t)

	go read(ctx, t, path, done)
	time.Sleep(step)
	go writeAndClose(t, path, "W1", "first_writer\n")
	time.Sleep(2 * step)
	go writeAndClose(t, path, "W2", "second_writer\n")
	time.Sleep(2 * step)
}
