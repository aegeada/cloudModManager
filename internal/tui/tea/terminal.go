package tea

import (
	"os"
	"strconv"
)

// GetTerminalSize returns the current terminal width and height, falling back to COLUMNS/LINES or 80x24.
func GetTerminalSize() (int, int, error) {
	width := 80
	height := 24

	if cols := os.Getenv("COLUMNS"); cols != "" {
		if c, err := strconv.Atoi(cols); err == nil && c > 0 {
			width = c
		}
	}
	if lines := os.Getenv("LINES"); lines != "" {
		if l, err := strconv.Atoi(lines); err == nil && l > 0 {
			height = l
		}
	}

	return width, height, nil
}
