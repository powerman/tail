package tail

import (
	"context"
	"os"
)

type trackedFile struct {
	ctx    context.Context
	path   string
	cancel context.CancelFunc
	info   os.FileInfo
	*os.File
}

func newTrackedFile(ctx context.Context, path string) *trackedFile {
	return &trackedFile{
		ctx:  ctx,
		path: path,
	}
}

func (f *trackedFile) Open() error {
	file, err := os.Open(f.path)
	if err != nil {
		return err
	}
	fi, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}

	// Make it possible to interrupt ff.Read(), which may
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
	f.File = nil
	f.info = nil
	f.cancel()
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
