package styles

import (
	"strings"
	"testing"
)

func TestStyles_RenderAndAttributes(t *testing.T) {
	st := New().
		Foreground(ColorCyan).
		Background(ColorBlack).
		Bold(true).
		Underline(true).
		Padding(1, 2, 1, 2)

	rendered := st.Render("Hello")
	clean := StripANSI(rendered)

	if !strings.Contains(clean, "Hello") {
		t.Errorf("Expected rendered string to contain 'Hello', got '%s'", clean)
	}

	// Border rendering
	bordered := New().
		Border(RoundedBorder).
		BorderForeground(ColorPrimary).
		Render("Box Content")

	cleanBordered := StripANSI(bordered)
	if !strings.Contains(cleanBordered, "Box Content") || !strings.Contains(cleanBordered, "╭") {
		t.Errorf("Expected rounded border in rendered output, got:\n%s", cleanBordered)
	}
}

func TestStyles_Helpers(t *testing.T) {
	// StringWidth
	coloredStr := "\033[31mRed Text\033[0m"
	if StringWidth(coloredStr) != 8 {
		t.Errorf("Expected visual width 8, got %d", StringWidth(coloredStr))
	}

	// Truncate
	truncated := Truncate("This is a long sentence", 10, "…")
	if StringWidth(truncated) > 10 {
		t.Errorf("Expected truncated width <= 10, got %d ('%s')", StringWidth(truncated), truncated)
	}

	// Pad
	paddedLeft := Pad("abc", 6, AlignLeft)
	if paddedLeft != "abc   " {
		t.Errorf("Expected 'abc   ', got '%s'", paddedLeft)
	}

	paddedRight := Pad("abc", 6, AlignRight)
	if paddedRight != "   abc" {
		t.Errorf("Expected '   abc', got '%s'", paddedRight)
	}

	paddedCenter := Pad("abc", 7, AlignCenter)
	if paddedCenter != "  abc  " {
		t.Errorf("Expected '  abc  ', got '%s'", paddedCenter)
	}

	// JoinHorizontal
	block1 := "A1\nA2"
	block2 := "B1\nB2"
	joined := JoinHorizontal(AlignLeft, block1, "  ", block2)
	lines := strings.Split(joined, "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "A1  B1") {
		t.Errorf("Expected joined horizontal lines, got:\n%s", joined)
	}

	// JoinVertical
	vert := JoinVertical(AlignLeft, "Line1", "Line2")
	if vert != "Line1\nLine2" {
		t.Errorf("Expected 'Line1\\nLine2', got '%s'", vert)
	}
}
