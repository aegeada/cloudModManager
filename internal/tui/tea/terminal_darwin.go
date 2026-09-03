//go:build darwin

package tea

import (
	"os"
	"syscall"
	"unsafe"
)

// IsTerminal checks if the given file descriptor is a real terminal on Darwin (macOS).
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	var termios syscall.Termios
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGETA), uintptr(unsafe.Pointer(&termios)))
	return err == 0
}
