package tui

import (
	"os"
	"syscall"
	"unsafe"
)

func IsTerminal(f *os.File) bool {
	var term syscall.Termios
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TIOCGETA, uintptr(unsafe.Pointer(&term)))
	return e == 0
}
