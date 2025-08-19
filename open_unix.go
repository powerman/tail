//go:build !windows

package tail

import (
	"io/fs"
	"os"
	"runtime"
)

// openFile opens a file for reading in a platform-specific way.
func openFile(path string) (*os.File, error) {
	// On macOS, opening FIFO with O_RDONLY blocks until a writer opens the write end.
	// For FIFO files, use O_RDWR to avoid blocking.
	if runtime.GOOS == "darwin" {
		fi, err := os.Lstat(path)
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
