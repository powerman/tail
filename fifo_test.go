// Tests to demonstrate differences in FIFO behavior between Linux and macOS.
//
// On Linux:
// - When the last writer closes FIFO, reader gets EOF
// - Reader blocks on opening FIFO if there are no writers
// - Data can be buffered and read after writer closes
//
// On macOS (expected behavior):
// - Reader might NOT get EOF when writer closes
// - Blocking behavior might differ
// - This can cause programs expecting EOF to hang
//
// These tests don't depend on the tail library and use only standard
// Go functions for working with FIFO pipes.
package tail_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const step = 200 * time.Millisecond

// mkfifoTest creates FIFO pipe for testing.
func mkfifoTest(path string) error {
	return syscall.Mkfifo(path, 0o600)
}

// createTempFIFO creates temporary FIFO file.
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

// logWithTime outputs message with timestamp.
func logWithTime(t *testing.T, format string, args ...any) {
	t.Helper()
	now := time.Now().Round(100 * time.Millisecond).Format("15:04:05.0")
	args = append([]any{now}, args...)
	fmt.Printf("  [%s] "+format+"\n", args...)
}

// write writes multiple messages to FIFO with delay.
func write(t *testing.T, path string, id string, messages ...string) *os.File {
	t.Helper()
	logWithTime(t, "%s: Opening FIFO for writing: %s", id, filepath.Base(path))
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		logWithTime(t, "%s: Error opening FIFO for writing: %s", id, err)
		return f
	}
	logWithTime(t, "%s: Opened FIFO for writing", id)
	// Note: FIFO stays open until process ends or explicit close
	for _, msg := range messages {
		time.Sleep(step)
		// Set write deadline to detect hanging writes
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

// writeAndClose writes multiple messages to FIFO with delay and explicitly closes.
func writeAndClose(t *testing.T, path string, id string, messages ...string) {
	t.Helper()
	f := write(t, path, id, messages...)
	logWithTime(t, "%s: Closing FIFO writer", id)
	f.Close()
}

func openFIFOForReadNonblock(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

// readFromFIFO reads data from FIFO until context cancellation.
func readFromFIFO(ctx context.Context, t *testing.T, path string, id string, done chan<- struct{}) {
	t.Helper()
	defer close(done)
	logWithTime(t, "%s: Opening FIFO for reading: %s", id, filepath.Base(path))
	f, err := openFIFOForReadNonblock(path)
	if err != nil {
		logWithTime(t, "%s: Error opening FIFO for reading: %s", id, err)
		return
	}
	defer f.Close()
	logWithTime(t, "%s: Opened FIFO for reading in non-blocking mode", id)

	buf := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			logWithTime(t, "%s: Read interrupted by context", id)
			return
		default:
			// Set small timeout for read operation
			f.SetReadDeadline(time.Now().Add(step / 4))
			n, err := f.Read(buf)
			switch {
			case errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK):
				time.Sleep(step / 4)
			case errors.Is(err, io.EOF):
				logWithTime(t, "%s: Got EOF on read", id)
				time.Sleep(step / 4)
			case os.IsTimeout(err):
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

func TestFIFO_OnlyReader(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFO is not supported on Windows")
	}

	logWithTime(t, "Test: only reader")
	path := createTempFIFO(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*step)
	defer cancel()

	done := make(chan struct{})
	go readFromFIFO(ctx, t, path, "R1", done)

	// Wait for test completion
	<-ctx.Done()
	logWithTime(t, "Test finished")
	<-done
}

func TestFIFO_WriterWithoutReader(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFO is not supported on Windows")
	}

	logWithTime(t, "Test: writer without reader (should timeout on write)")
	path := createTempFIFO(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*step)
	defer cancel()

	// Writer tries to write without any reader
	go write(t, path, "W1", "message1\n")

	// Wait for test completion
	<-ctx.Done()
	logWithTime(t, "Test finished")
}

func TestFIFO_WriterFirst(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFO is not supported on Windows")
	}

	logWithTime(t, "Test: writer starts step sec before reader")
	path := createTempFIFO(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*step)
	defer cancel()

	// Writer starts immediately
	go write(t, path, "W1", "message1\n")

	// Reader starts after step
	time.Sleep(step)
	done := make(chan struct{})
	go readFromFIFO(ctx, t, path, "R1", done)

	<-ctx.Done()
	logWithTime(t, "Test finished")
	<-done
}

func TestFIFO_ReaderFirst(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFO is not supported on Windows")
	}

	logWithTime(t, "Test: writer starts step sec after reader")
	path := createTempFIFO(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*step)
	defer cancel()

	// Reader starts immediately
	done := make(chan struct{})
	go readFromFIFO(ctx, t, path, "R1", done)

	// Writer starts after step
	time.Sleep(step)
	go write(t, path, "W1", "message1\n")

	<-ctx.Done()
	logWithTime(t, "Test finished")
	<-done
}

func TestFIFO_TwoWritersBeforeReader(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFO is not supported on Windows")
	}

	logWithTime(t, "Test: two writers start 2*step and step/2 sec before reader")
	path := createTempFIFO(t)

	ctx, cancel := context.WithTimeout(context.Background(), 7*step)
	defer cancel()

	// First writer starts immediately
	go write(t, path, "W1", "writer1\n")

	// Second writer starts after step/2

	time.Sleep(step / 2)
	go write(t, path, "W2", "writer2\n")

	// Reader starts after step
	time.Sleep(step)
	done := make(chan struct{})
	go readFromFIFO(ctx, t, path, "R1", done)

	<-ctx.Done()
	logWithTime(t, "Test finished")
	<-done
}

func TestFIFO_WritersAroundReader(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFO is not supported on Windows")
	}

	logWithTime(t, "Test: two writers - one step/2 sec before, another step/2 sec after reader")
	path := createTempFIFO(t)

	ctx, cancel := context.WithTimeout(context.Background(), 7*step)
	defer cancel()

	// First writer starts immediately
	go write(t, path, "W1", "early_writer\n")

	// Reader starts через 0.1 sec
	time.Sleep(step)
	done := make(chan struct{})
	go readFromFIFO(ctx, t, path, "R1", done)

	// Second writer запускается after step/2 sec
	time.Sleep(2 * step)
	go write(t, path, "W2", "late_writer\n")

	<-ctx.Done()
	logWithTime(t, "Test finished")
	<-done
}

func TestFIFO_TwoWritersAfterReader(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFO is not supported on Windows")
	}

	logWithTime(t, "Test: two writers start step/2 and 2*step sec after reader")
	path := createTempFIFO(t)

	ctx, cancel := context.WithTimeout(context.Background(), 7*step)
	defer cancel()

	// Reader starts immediately
	done := make(chan struct{})
	go readFromFIFO(ctx, t, path, "R1", done)

	// First writer starts after step
	time.Sleep(step)
	go write(t, path, "W1", "writer1\n")

	// Second writer запускается after step/2 sec
	time.Sleep(2 * step)
	go write(t, path, "W2", "writer2\n")

	<-ctx.Done()
	logWithTime(t, "Test finished")
	<-done
}

func TestFIFO_OneWriterCloses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFO is not supported on Windows")
	}

	logWithTime(t, "Test: one writer closes FIFO after writing three messages")
	path := createTempFIFO(t)

	ctx, cancel := context.WithTimeout(context.Background(), 8*step)
	defer cancel()

	// Reader starts immediately
	done := make(chan struct{})
	go readFromFIFO(ctx, t, path, "R1", done)

	// Writer with closing
	time.Sleep(step)
	go writeAndClose(t, path, "W1", "msg1\n", "msg2\n", "msg3\n")

	<-ctx.Done()
	logWithTime(t, "Test finished")
	<-done
}

func TestFIFO_TwoWritersOneCloses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFO is not supported on Windows")
	}

	logWithTime(t, "Test: two writers, one closes FIFO")
	path := createTempFIFO(t)

	ctx, cancel := context.WithTimeout(context.Background(), 8*step)
	defer cancel()

	// Reader starts immediately
	done := make(chan struct{})
	go readFromFIFO(ctx, t, path, "R1", done)

	// First writer without closing
	time.Sleep(step)
	go write(t, path, "W1", "persistent_writer\n")

	// Second writer with closing (starts after step/2 delay)
	time.Sleep(2 * step)
	go writeAndClose(t, path, "W2", "closing_writer\n")

	<-ctx.Done()
	logWithTime(t, "Test finished")
	<-done
}

func TestFIFO_TwoWritersBothClose(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFO is not supported on Windows")
	}

	logWithTime(t, "Test: two writers, both close FIFO")
	path := createTempFIFO(t)

	ctx, cancel := context.WithTimeout(context.Background(), 8*step)
	defer cancel()

	// Reader starts immediately
	done := make(chan struct{})
	go readFromFIFO(ctx, t, path, "R1", done)

	// First writer with closing
	time.Sleep(step)
	go writeAndClose(t, path, "W1", "first_writer\n")

	// Second writer with closing (starts after step/2 delay)
	time.Sleep(2 * step)
	go writeAndClose(t, path, "W2", "second_writer\n")

	<-ctx.Done()
	logWithTime(t, "Test finished")
	<-done
}
