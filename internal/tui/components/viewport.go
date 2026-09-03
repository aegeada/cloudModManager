package components

import (
	"strings"

	"cmm/internal/tui/styles"
	"cmm/internal/tui/tea"
)

// Viewport provides a scrollable multi-line text viewport.
type Viewport struct {
	Width   int
	Height  int
	lines   []string
	yOffset int
}

// NewViewport creates a Viewport with dimensions.
func NewViewport(width, height int) Viewport {
	if height < 1 {
		height = 1
	}
	return Viewport{
		Width:   width,
		Height:  height,
		lines:   []string{},
		yOffset: 0,
	}
}

func (v *Viewport) SetContent(content string) {
	v.lines = strings.Split(content, "\n")
	if v.yOffset >= len(v.lines) {
		v.GotoBottom()
	}
}

func (v *Viewport) Append(line string) {
	newLines := strings.Split(line, "\n")
	v.lines = append(v.lines, newLines...)
	v.GotoBottom()
}

func (v *Viewport) Clear() {
	v.lines = []string{}
	v.yOffset = 0
}

func (v *Viewport) ScrollUp(n int) {
	v.yOffset -= n
	if v.yOffset < 0 {
		v.yOffset = 0
	}
}

func (v *Viewport) ScrollDown(n int) {
	maxOffset := len(v.lines) - v.Height
	if maxOffset < 0 {
		maxOffset = 0
	}
	v.yOffset += n
	if v.yOffset > maxOffset {
		v.yOffset = maxOffset
	}
}

func (v *Viewport) PageUp() {
	v.ScrollUp(v.Height)
}

func (v *Viewport) PageDown() {
	v.ScrollDown(v.Height)
}

func (v *Viewport) GotoTop() {
	v.yOffset = 0
}

func (v *Viewport) GotoBottom() {
	maxOffset := len(v.lines) - v.Height
	if maxOffset < 0 {
		maxOffset = 0
	}
	v.yOffset = maxOffset
}

// Update handles scroll keys.
func (v Viewport) Update(msg tea.Msg) (Viewport, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return v, nil
	}

	switch keyMsg.Type {
	case tea.KeyUp:
		v.ScrollUp(1)
	case tea.KeyDown:
		v.ScrollDown(1)
	case tea.KeyPgUp:
		v.PageUp()
	case tea.KeyPgDown:
		v.PageDown()
	case tea.KeyHome:
		v.GotoTop()
	case tea.KeyEnd:
		v.GotoBottom()
	case tea.KeyRunes:
		switch keyMsg.String() {
		case "k":
			v.ScrollUp(1)
		case "j":
			v.ScrollDown(1)
		}
	}

	return v, nil
}

// View renders the visible lines within the viewport.
func (v Viewport) View() string {
	if len(v.lines) == 0 {
		return ""
	}

	start := v.yOffset
	if start < 0 {
		start = 0
	}
	if start >= len(v.lines) {
		start = len(v.lines) - 1
	}

	end := start + v.Height
	if end > len(v.lines) {
		end = len(v.lines)
	}

	var sb strings.Builder
	for i := start; i < end; i++ {
		line := v.lines[i]
		if v.Width > 0 {
			line = styles.Truncate(line, v.Width, "")
		}
		sb.WriteString(line)
		if i < end-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
