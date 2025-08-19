//go:build !windows

package tail

import "os"

// openFile opens a file for reading in a platform-specific way.
func openFile(path string) (*os.File, error) {
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
