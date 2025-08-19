//go:build windows

package tail

import "errors"

func mkfifo(path string, mode uint32) error {
	return errors.New("FIFO pipes are not supported on Windows")
}
