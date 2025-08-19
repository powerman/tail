//go:build !windows

package tail

import "syscall"

func mkfifo(path string, mode uint32) error {
	return syscall.Mkfifo(path, mode)
}
