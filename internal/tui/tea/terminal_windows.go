//go:build windows

package tea

import (
	"os"
	"syscall"
)

// IsTerminal checks if the given file descriptor is a real Windows console.
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	var mode uint32
	err := syscall.GetConsoleMode(syscall.Handle(f.Fd()), &mode)
	return err == nil
}
