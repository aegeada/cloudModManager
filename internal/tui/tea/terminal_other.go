//go:build !linux && !darwin && !windows

package tea

import "os"

// IsTerminal fallback for other operating systems.
func IsTerminal(f *os.File) bool {
	return false
}
