package styles

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Color represents a terminal ANSI or RGB color.
type Color string

// Alignment options.
type Alignment int

const (
	AlignLeft Alignment = iota
	AlignCenter
	AlignRight
)

// BorderStyle defines characters used to draw borders around boxes.
type BorderStyle struct {
	Top         string
	Bottom      string
	Left        string
	Right       string
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
}

var (
	NormalBorder = BorderStyle{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "┌",
		TopRight:    "┐",
		BottomLeft:  "└",
		BottomRight: "┘",
	}

	RoundedBorder = BorderStyle{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "╰",
		BottomRight: "╯",
	}

	DoubleBorder = BorderStyle{
		Top:         "═",
		Bottom:      "═",
		Left:        "║",
		Right:       "║",
		TopLeft:     "╔",
		TopRight:    "╗",
		BottomLeft:  "╚",
		BottomRight: "╝",
	}

	HiddenBorder = BorderStyle{
		Top:         " ",
		Bottom:      " ",
		Left:        " ",
		Right:       " ",
		TopLeft:     " ",
		TopRight:    " ",
		BottomLeft:  " ",
		BottomRight: " ",
	}
)

// Style specifies rendering rules for text.
type Style struct {
	fg           Color
	bg           Color
	bold         bool
	faint        bool
	italic       bool
	underline    bool
	width        int
	height       int
	padTop       int
	padRight     int
	padBottom    int
	padLeft      int
	border       *BorderStyle
	borderFg     Color
	borderTop    bool
	borderRight  bool
	borderBottom bool
	borderLeft   bool
	align        Alignment
}

// New creates an empty Style.
func New() Style {
	return Style{
		borderTop:    true,
		borderRight:  true,
		borderBottom: true,
		borderLeft:   true,
	}
}

func (s Style) Foreground(c Color) Style {
	s.fg = c
	return s
}

func (s Style) Background(c Color) Style {
	s.bg = c
	return s
}

func (s Style) Bold(b bool) Style {
	s.bold = b
	return s
}

func (s Style) Faint(b bool) Style {
	s.faint = b
	return s
}

func (s Style) Italic(b bool) Style {
	s.italic = b
	return s
}

func (s Style) Underline(b bool) Style {
	s.underline = b
	return s
}

func (s Style) Width(w int) Style {
	s.width = w
	return s
}

func (s Style) Height(h int) Style {
	s.height = h
	return s
}

func (s Style) Align(a Alignment) Style {
	s.align = a
	return s
}

func (s Style) Padding(top, right, bottom, left int) Style {
	s.padTop = top
	s.padRight = right
	s.padBottom = bottom
	s.padLeft = left
	return s
}

func (s Style) PaddingLeft(p int) Style {
	s.padLeft = p
	return s
}

func (s Style) PaddingRight(p int) Style {
	s.padRight = p
	return s
}

func (s Style) PaddingTop(p int) Style {
	s.padTop = p
	return s
}

func (s Style) PaddingBottom(p int) Style {
	s.padBottom = p
	return s
}

func (s Style) Border(b BorderStyle) Style {
	s.border = &b
	return s
}

func (s Style) BorderForeground(c Color) Style {
	s.borderFg = c
	return s
}

// Render formats the string according to the Style properties.
func (s Style) Render(str string) string {
	lines := strings.Split(str, "\n")

	// Apply horizontal alignment and width padding to lines
	maxContentWidth := 0
	for _, l := range lines {
		w := StringWidth(l)
		if w > maxContentWidth {
			maxContentWidth = w
		}
	}

	targetWidth := s.width
	if targetWidth <= 0 {
		targetWidth = maxContentWidth
	}

	var paddedLines []string
	for _, l := range lines {
		rawW := StringWidth(l)
		var aligned string
		diff := targetWidth - rawW
		if diff < 0 {
			diff = 0
		}

		switch s.align {
		case AlignCenter:
			leftP := diff / 2
			rightP := diff - leftP
			aligned = strings.Repeat(" ", leftP) + l + strings.Repeat(" ", rightP)
		case AlignRight:
			aligned = strings.Repeat(" ", diff) + l
		default: // AlignLeft
			aligned = l + strings.Repeat(" ", diff)
		}

		// Apply left/right padding
		aligned = strings.Repeat(" ", s.padLeft) + aligned + strings.Repeat(" ", s.padRight)
		paddedLines = append(paddedLines, aligned)
	}

	// Apply top/bottom padding
	innerWidth := targetWidth + s.padLeft + s.padRight
	emptyLine := strings.Repeat(" ", innerWidth)

	var fullContentLines []string
	for i := 0; i < s.padTop; i++ {
		fullContentLines = append(fullContentLines, emptyLine)
	}
	fullContentLines = append(fullContentLines, paddedLines...)
	for i := 0; i < s.padBottom; i++ {
		fullContentLines = append(fullContentLines, emptyLine)
	}

	// Height padding
	for len(fullContentLines) < s.height {
		fullContentLines = append(fullContentLines, emptyLine)
	}

	// Apply border if set
	if s.border != nil {
		b := s.border
		topBorder := b.TopLeft + strings.Repeat(b.Top, innerWidth) + b.TopRight
		bottomBorder := b.BottomLeft + strings.Repeat(b.Bottom, innerWidth) + b.BottomRight

		if s.borderFg != "" {
			topBorder = applyColor(topBorder, s.borderFg, "", false, false, false, false)
			bottomBorder = applyColor(bottomBorder, s.borderFg, "", false, false, false, false)
		}

		var borderedLines []string
		borderedLines = append(borderedLines, topBorder)
		for _, line := range fullContentLines {
			leftB := b.Left
			rightB := b.Right
			if s.borderFg != "" {
				leftB = applyColor(leftB, s.borderFg, "", false, false, false, false)
				rightB = applyColor(rightB, s.borderFg, "", false, false, false, false)
			}
			borderedLines = append(borderedLines, leftB+line+rightB)
		}
		borderedLines = append(borderedLines, bottomBorder)
		fullContentLines = borderedLines
	}

	// Apply text styling attributes
	var result strings.Builder
	for i, line := range fullContentLines {
		if i > 0 {
			result.WriteString("\n")
		}
		if s.fg != "" || s.bg != "" || s.bold || s.faint || s.italic || s.underline {
			result.WriteString(applyColor(line, s.fg, s.bg, s.bold, s.faint, s.italic, s.underline))
		} else {
			result.WriteString(line)
		}
	}

	return result.String()
}

func applyColor(str string, fg Color, bg Color, bold, faint, italic, underline bool) string {
	if str == "" {
		return ""
	}
	var codes []string
	if bold {
		codes = append(codes, "1")
	}
	if faint {
		codes = append(codes, "2")
	}
	if italic {
		codes = append(codes, "3")
	}
	if underline {
		codes = append(codes, "4")
	}

	if fg != "" {
		if c := colorCode(fg, true); c != "" {
			codes = append(codes, c)
		}
	}
	if bg != "" {
		if c := colorCode(bg, false); c != "" {
			codes = append(codes, c)
		}
	}

	if len(codes) == 0 {
		return str
	}

	prefix := "\033[" + strings.Join(codes, ";") + "m"
	suffix := "\033[0m"
	return prefix + str + suffix
}

func colorCode(c Color, isFg bool) string {
	s := string(c)
	if s == "" {
		return ""
	}

	// Hex color e.g. #ff007f
	if strings.HasPrefix(s, "#") && len(s) == 7 {
		r, _ := strconv.ParseInt(s[1:3], 16, 64)
		g, _ := strconv.ParseInt(s[3:5], 16, 64)
		b, _ := strconv.ParseInt(s[5:7], 16, 64)
		if isFg {
			return fmt.Sprintf("38;2;%d;%d;%d", r, g, b)
		}
		return fmt.Sprintf("48;2;%d;%d;%d", r, g, b)
	}

	// ANSI 256 or basic color number
	if num, err := strconv.Atoi(s); err == nil {
		if num < 16 {
			if isFg {
				if num < 8 {
					return fmt.Sprintf("%d", 30+num)
				}
				return fmt.Sprintf("%d", 90+num-8)
			}
			if num < 8 {
				return fmt.Sprintf("%d", 40+num)
			}
			return fmt.Sprintf("%d", 100+num-8)
		}
		if isFg {
			return fmt.Sprintf("38;5;%d", num)
		}
		return fmt.Sprintf("48;5;%d", num)
	}

	return ""
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// StripANSI removes all ANSI escape sequences from string.
func StripANSI(str string) string {
	return ansiRegex.ReplaceAllString(str, "")
}

// StringWidth calculates visual width of string ignoring ANSI escape codes.
func StringWidth(str string) int {
	clean := StripANSI(str)
	return utf8.RuneCountInString(clean)
}

// Truncate cuts a string to max visual width, appending tail if truncated.
func Truncate(str string, maxWidth int, tail string) string {
	if maxWidth <= 0 {
		return ""
	}
	clean := StripANSI(str)
	runes := []rune(clean)
	if len(runes) <= maxWidth {
		return str
	}
	tailRunes := []rune(tail)
	if len(tailRunes) >= maxWidth {
		return string(runes[:maxWidth])
	}
	return string(runes[:maxWidth-len(tailRunes)]) + tail
}

// Pad adds spaces to reach width according to Alignment.
func Pad(str string, width int, align Alignment) string {
	w := StringWidth(str)
	if w >= width {
		return str
	}
	diff := width - w
	switch align {
	case AlignCenter:
		left := diff / 2
		right := diff - left
		return strings.Repeat(" ", left) + str + strings.Repeat(" ", right)
	case AlignRight:
		return strings.Repeat(" ", diff) + str
	default:
		return str + strings.Repeat(" ", diff)
	}
}

// JoinHorizontal joins multi-line blocks horizontally side-by-side.
func JoinHorizontal(align Alignment, blocks ...string) string {
	if len(blocks) == 0 {
		return ""
	}
	var blockLines [][]string
	maxLines := 0
	blockWidths := make([]int, len(blocks))

	for i, b := range blocks {
		lines := strings.Split(b, "\n")
		blockLines = append(blockLines, lines)
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
		maxW := 0
		for _, l := range lines {
			w := StringWidth(l)
			if w > maxW {
				maxW = w
			}
		}
		blockWidths[i] = maxW
	}

	var result strings.Builder
	for lineIdx := 0; lineIdx < maxLines; lineIdx++ {
		if lineIdx > 0 {
			result.WriteString("\n")
		}
		for bIdx, lines := range blockLines {
			w := blockWidths[bIdx]
			if lineIdx < len(lines) {
				line := lines[lineIdx]
				lineW := StringWidth(line)
				result.WriteString(line)
				if lineW < w {
					result.WriteString(strings.Repeat(" ", w-lineW))
				}
			} else {
				result.WriteString(strings.Repeat(" ", w))
			}
		}
	}
	return result.String()
}

// JoinVertical joins strings vertically.
func JoinVertical(align Alignment, blocks ...string) string {
	return strings.Join(blocks, "\n")
}
