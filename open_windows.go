//go:build windows

package tail

import (
	"os"
	"syscall"
)

// openFile opens a file for reading with FILE_SHARE_* mode on Windows.
func openFile(path string) (*os.File, error) {
	return createSharedFile(path,
		syscall.GENERIC_READ,
		syscall.OPEN_EXISTING,
	)
}

// createTempFile creates a temporary file opened with FILE_SHARE_* mode on Windows.
func createTempFile(dir, pattern string) (*os.File, error) {
	// First create temp file normally to get unique name.
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	path := f.Name()
	f.Close()

	return createSharedFile(path,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.OPEN_EXISTING,
	)
}

// createFile creates a file with FILE_SHARE_* mode on Windows.
func createFile(path string) (*os.File, error) {
	return createSharedFile(path,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.CREATE_ALWAYS,
	)
}

// createSharedFile creates a file with FILE_SHARE_* mode on Windows.
func createSharedFile(path string, access uint32, createmode uint32) (*os.File, error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	handle, err := syscall.CreateFile(
		pathPtr,
		access,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		createmode,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(handle), path), nil
}
