//go:build windows

package tail

import (
	"context"
	"os"
	"syscall"
)

type trackedFile struct {
	*os.File

	ctx    context.Context //nolint:containedctx // By design.
	path   string
	cancel context.CancelFunc
	info   os.FileInfo
}

func newTrackedFile(ctx context.Context, path string) *trackedFile {
	return &trackedFile{
		ctx:    ctx,
		path:   path,
		cancel: nil,
		info:   nil,
		File:   nil,
	}
}

func (f *trackedFile) Open() error {
	// On Windows, open file with FILE_SHARE_DELETE to allow rename/delete
	// Use syscall to get proper sharing flags
	pathPtr, err := syscall.UTF16PtrFromString(f.path)
	if err != nil {
		return err
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
		return err
	}

	file := os.NewFile(uintptr(handle), f.path)
	fi, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}

	// Make it possible to interrupt f.Read(), which may
	// block when reading from FIFO or file mounted by network.
	ctx, cancel := context.WithCancel(f.ctx)
	go func() {
		<-ctx.Done()
		_ = file.Close()
	}()

	f.File = file
	f.info = fi
	f.cancel = cancel
	return nil
}

func (f *trackedFile) Close() {
	if f.cancel != nil {
		f.cancel()
	}
	f.File = nil
	f.info = nil
}

func (f *trackedFile) Opened() bool {
	return f.File != nil
}

func (f *trackedFile) Usual() bool {
	return f.info.Mode()&os.ModeType == 0
}

func (f *trackedFile) Detached() bool {
	fi, err := os.Stat(f.path)
	return err != nil || !os.SameFile(f.info, fi)
}
