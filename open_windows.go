//go:build windows

package tail

import (
	"os"
	"syscall"
)

// openFile opens a file for reading with FILE_SHARE_DELETE on Windows
func openFile(path string) (*os.File, error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	handle, err := syscall.CreateFile(
		pathPtr,
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(handle), path), nil
}

// createTempFile creates a temporary file with FILE_SHARE_DELETE on Windows
func createTempFile(dir, pattern string) (*os.File, error) {
	// First create temp file normally to get unique name
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	path := f.Name()
	f.Close()
	os.Remove(path) // Remove it so we can recreate with proper flags

	// Now create with proper sharing flags
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	handle, err := syscall.CreateFile(
		pathPtr,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.CREATE_ALWAYS,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(handle), path), nil
}

// createFile creates a file with FILE_SHARE_DELETE on Windows
func createFile(path string) (*os.File, error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	handle, err := syscall.CreateFile(
		pathPtr,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.CREATE_ALWAYS,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(handle), path), nil
}
