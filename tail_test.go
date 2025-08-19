package tail //nolint:testpackage // TODO

import (
	"io"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/powerman/check"
)

func TestInvalidPath(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.path = ""
	tail.Run()
	tail.Want(pollTimeout-pollDelay/2, "", nil)
	if runtime.GOOS == "windows" {
		tail.Want(pollDelay, "", syscall.Errno(3)) // ERROR_PATH_NOT_FOUND
	} else {
		tail.Want(pollDelay, "", syscall.ENOENT)
	}
}

func TestNotExists(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.Remove()
	tail.Run()

	tail.Want(pollTimeout-pollDelay/2, "", nil)
	tail.Want(pollDelay, "", syscall.ENOENT)
}

func TestNotExistsGrow(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.Remove()
	tail.Run()

	time.Sleep(pollDelay / 2)
	tail.Create()
	tail.Write("new1.1\nnew1.2\n")
	tail.Want(pollDelay*3/2, "new1.1\nnew1.2\n", nil)
	tail.Want(pollTimeout+pollDelay/2, "", nil)
}

func TestEmpty(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.Run()

	tail.Want(pollTimeout+pollDelay/2, "", nil)
}

func TestEmptyGrow(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.Run()

	tail.Write("new1\n")
	tail.Want(pollDelay*3/2, "new1\n", nil)
}

func TestEmptyGrowAsync(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.Run()

	go func() {
		time.Sleep(pollDelay / 2)
		tail.Write("new1.1\nnew1.2\n")
		time.Sleep(pollDelay)
		tail.Write("new2\n")
		time.Sleep(pollTimeout * 2)
		tail.Write("new3\n")
		time.Sleep(pollTimeout)
		tail.Write("new4\n")
	}()
	tail.Want(pollTimeout*3, "new1.1\nnew1.2\nnew2\nnew3\n", nil)
	tail.Want(pollTimeout+pollDelay/2, "new4\n", nil)
}

func TestNotEmptyGrow(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.Write("old1.1\nold1.2\n")
	tail.Run()

	tail.Want(pollTimeout+pollDelay/2, "", nil)

	tail.Write("new1.1\nnew1.2\n")
	tail.Write("new2\n")
	tail.Want(pollDelay*3/2, "new1.1\nnew1.2\nnew2\n", nil)
}

func TestNotEmptyGrowBytes(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.Write("old\nab")
	tail.Run()

	tail.Want(pollTimeout+pollDelay/2, "", nil)

	tail.Write("cd")
	tail.Want(pollDelay*3/2, "cd", nil)

	tail.Write("ef\ngh")
	tail.Want(pollDelay*3/2, "ef\ngh", nil)
}

func TestFIFOGrow(tt *testing.T) {
	if runtime.GOOS == "windows" {
		tt.Skip("FIFO pipes test is not supported on Windows")
	}

	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.Remove()
	tail.CreateFIFO()
	tail.Run()

	go func() {
		tail.Write("new1.1\nnew1.2\n")
		time.Sleep(pollDelay)
		tail.Write("new2\n")
	}()
	tail.Want(pollDelay*5/2, "new1.1\nnew1.2\nnew2\n", nil)
	tail.Want(pollTimeout+pollDelay/2, "", nil)
}

func TestClose(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.Write("old1.1\nold1.2\n")
	tail.Run()

	tail.Write("new1\n")
	tail.Want(pollDelay*3/2, "new1\n", nil)

	tail.Cancel()
	tail.Write("new2\n")
	tail.Want(pollDelay*2, "", io.EOF)

	tail.Cancel()
	tail.Write("new3\n")
	tail.Want(pollDelay*2, "", io.ErrClosedPipe) // error from testTail, not from Tail

	_, err := tail.Read(make([]byte, 8))
	t.Err(err, io.EOF)
	_, err = tail.Read(nil)
	t.Err(err, io.EOF)
}

func TestRenameGrow(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.Run()

	tail.Want(pollTimeout+pollDelay/2, "", nil)

	tail.Rename()
	tail.Write("new1.1\nnew1.2\n")
	tail.Want(pollDelay*2, "new1.1\nnew1.2\n", nil)
}

func TestRemoveGrow(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.Run()

	tail.Want(pollTimeout+pollDelay/2, "", nil)

	tail.Remove()
	tail.Write("new1.1\nnew1.2\n")
	tail.Want(pollDelay*2, "new1.1\nnew1.2\n", nil)
}

func TestRotate(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.Run()

	tail.Write("old1\n")
	time.Sleep(pollDelay / 2)
	tail.Write("old2\n")
	time.Sleep(pollDelay / 2)
	tail.Write("old3\n")
	time.Sleep(pollDelay / 2)
	tail.Write("old4\n")
	time.Sleep(pollDelay / 2)
	tail.Write("old5\n") // this one shouldn't be read by tail.Read before read()
	time.Sleep(pollDelay / 2)
	tail.Rename()
	tail.Create()
	tail.Write("new1.1\nnew1.2\n")
	tail.Want(pollDelay*3/2, "old1\nold2\nold3\nold4\nold5\nnew1.1\nnew1.2\n", nil)
}

func TestRotateAtEOF(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.Run()

	tail.Write("old1.1\nold1.2\n")
	tail.Want(pollDelay*3/2, "old1.1\nold1.2\n", nil)

	tail.Rename()
	tail.Create()
	tail.Write("new1.1\nnew1.2\n")
	tail.Want(pollDelay*3/2, "new1.1\nnew1.2\n", nil)
}

func TestRotateAtEOFGrow(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.Run()

	tail.Write("old1.1\nold1.2\n")
	tail.Want(pollDelay*3/2, "old1.1\nold1.2\n", nil)

	f := tail.f
	tail.Rename()
	tail.Create()

	time.Sleep(pollDelay * 3 / 2)
	_, err := f.WriteString("old2\n")
	t.Nil(err)
	tail.Write("new1.1\nnew1.2\n")
	tail.Want(pollDelay*3/2, "new1.1\nnew1.2\n", nil)
}

func TestSymlink(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.CreateSymlink()
	tail.Run()

	tail.Write("old1.1\nold1.2\n")
	tail.Want(pollDelay*3/2, "old1.1\nold1.2\n", nil)

	tail.Rename()
	tail.Create()
	tail.Write("new1.1\nnew1.2\n")
	tail.Want(pollDelay*3/2, "new1.1\nnew1.2\n", nil)
}

func TestRotateSymlink(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	tail.CreateSymlink()
	tail.Run()

	tail.Write("old1.1\nold1.2\n")
	tail.Want(pollDelay*3/2, "old1.1\nold1.2\n", nil)

	tail.RemoveSymlink()
	time.Sleep(pollTimeout - pollDelay*5/2)
	tail.Want(pollDelay*3/2, "", syscall.ENOENT)
	tail.CreateSymlink()
	tail.Write("new1.1\nnew1.2\n")
	tail.Want(pollDelay*3/2, "new1.1\nnew1.2\nold1.1\nold1.2\nnew1.1\nnew1.2\n", nil)

	tail.RemoveSymlink()
	time.Sleep(pollTimeout + pollDelay/2)
	tail.CreateSymlink()
	tail.Write("new2\n")
	tail.Want(pollDelay*3/2, "", syscall.ENOENT)
	tail.Want(pollDelay*3/2, "new2\nold1.1\nold1.2\nnew1.1\nnew1.2\nnew2\n", nil)
}

func TestErrors(tt *testing.T) {
	if runtime.GOOS == "windows" {
		tt.Skip("Permission tests work differently on Windows")
	} else if os.Getuid() == 0 { // Happens in QEMU/Docker containers used for ARM/ARM64.
		tt.Skip("Permission tests does not work as root")
	}

	t := check.T(tt)
	t.Parallel()
	tail := newTestTail(t)

	t.Nil(os.Chmod(tail.path, 0))
	tail.Run()

	tail.Want(pollTimeout-pollDelay/2, "", nil)
	tail.Want(pollDelay, "", syscall.EACCES)

	tail.Remove()
	tail.Want(pollTimeout-pollDelay/2, "", nil)
	tail.Want(pollDelay, "", nil)

	tail.Create()
	tail.Want(pollDelay*3/2, "", nil)
	tail.Remove()
	tail.Want(pollTimeout-pollDelay/2, "", nil)
	tail.Want(pollDelay, "", nil)

	tail.Create()
	tail.Write("new\n")
	tail.Want(pollDelay*3/2, "new\n", nil)
	tail.Remove()
	tail.Want(pollTimeout-pollDelay, "", nil)
	tail.Want(pollDelay*3/2, "", syscall.ENOENT)
}
