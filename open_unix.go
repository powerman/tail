//go:build !windows

package tail

import (
	"io/fs"
	"os"
	"runtime"
)

// openFile opens a file for reading in a platform-specific way.
func openFile(path string) (*os.File, error) {
	// On macOS, FIFO (named pipes) opened with O_RDONLY will block until
	// someone opens the write end. To avoid this, we check if the file is a FIFO
	// and open it with O_RDWR instead.
	if runtime.GOOS == "darwin" {
		fi, err := os.Stat(path)
		if err == nil && fi.Mode()&fs.ModeNamedPipe != 0 {
			// This is a FIFO on macOS, open with O_RDWR to avoid blocking
			return os.OpenFile(path, os.O_RDWR, 0) //nolint:gosec // Path comes from trusted sources.
		}
	}
	return os.Open(path) //nolint:gosec // Path comes from trusted sources.
}

// createTempFile creates a temporary file in a platform-specific way.
func createTempFile(dir, pattern string) (*os.File, error) {
	return os.CreateTemp(dir, pattern)
}

// createFile creates a file in a platform-specific way.
func createFile(path string) (*os.File, error) {
	return os.Create(path) //nolint:gosec // Path comes from trusted sources.
}
