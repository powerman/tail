//go:build !windows

package tail

import (
	"os"

	"golang.org/x/sys/unix"
)

// openFile opens a file for reading in a platform-specific way.
func openFile(path string) (*os.File, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	var flags int
	if fi.Mode()&os.ModeNamedPipe != 0 {
		// Open FIFO in read-write non-blocking mode:
		// - O_RDWR ensures we never get EOF when reading because at least one writer is present.
		// - O_NONBLOCK is required to have Close() interrupt the Read() call.
		// - Also one of them is required to avoid blocking on Open() FIFO without writers.
		flags = unix.O_RDWR | unix.O_NONBLOCK
	} else {
		// Regular files only need read access.
		flags = unix.O_RDONLY | unix.O_NONBLOCK
	}

	fd, err := unix.Open(path, flags, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path) //nolint:gosec // False positive.
	return f, nil
}

// createTempFile creates a temporary file in a platform-specific way.
func createTempFile(dir, pattern string) (*os.File, error) {
	return os.CreateTemp(dir, pattern)
}

// createFile creates a file in a platform-specific way.
func createFile(path string) (*os.File, error) {
	return os.Create(path) //nolint:gosec // Path comes from trusted sources.
}
