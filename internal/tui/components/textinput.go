package components

import (
	"strings"

	"cmm/internal/tui/styles"
	"cmm/internal/tui/tea"
)

// TextInput represents a single-line editable text component.
type TextInput struct {
	Prompt      string
	Placeholder string
	value       []rune
	cursor      int
	focused     bool
	CharLimit   int
	Width       int
}

// NewTextInput constructs a TextInput.
func NewTextInput() TextInput {
	return TextInput{
		Prompt:    "> ",
		value:     []rune{},
		cursor:    0,
		CharLimit: 256,
		Width:     40,
	}
}

func (t *TextInput) Value() string {
	return string(t.value)
}

func (t *TextInput) SetValue(v string) {
	t.value = []rune(v)
	t.cursor = len(t.value)
}

func (t *TextInput) Reset() {
	t.value = []rune{}
	t.cursor = 0
}

func (t *TextInput) Focus() {
	t.focused = true
}

func (t *TextInput) Blur() {
	t.focused = false
}

func (t *TextInput) Focused() bool {
	return t.focused
}

func (t *TextInput) Cursor() int {
	return t.cursor
}

func (t *TextInput) SetCursor(pos int) {
	if pos < 0 {
		pos = 0
	}
	if pos > len(t.value) {
		pos = len(t.value)
	}
	t.cursor = pos
}

// Update processes key events when focused.
func (t TextInput) Update(msg tea.Msg) (TextInput, tea.Cmd) {
	if !t.focused {
		return t, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return t, nil
	}

	switch keyMsg.Type {
	case tea.KeyLeft:
		if t.cursor > 0 {
			t.cursor--
		}
	case tea.KeyRight:
		if t.cursor < len(t.value) {
			t.cursor++
		}
	case tea.KeyHome, tea.KeyCtrlA:
		t.cursor = 0
	case tea.KeyEnd, tea.KeyCtrlE:
		t.cursor = len(t.value)
	case tea.KeyBackspace:
		if t.cursor > 0 {
			t.value = append(t.value[:t.cursor-1], t.value[t.cursor:]...)
			t.cursor--
		}
	case tea.KeyDelete:
		if t.cursor < len(t.value) {
			t.value = append(t.value[:t.cursor], t.value[t.cursor+1:]...)
		}
	case tea.KeyCtrlU:
		// Delete from beginning to cursor
		t.value = t.value[t.cursor:]
		t.cursor = 0
	case tea.KeyCtrlK:
		// Delete from cursor to end
		t.value = t.value[:t.cursor]
	case tea.KeyCtrlW:
		// Delete last word
		if t.cursor > 0 {
			i := t.cursor - 1
			for i > 0 && t.value[i] == ' ' {
				i--
			}
			for i > 0 && t.value[i] != ' ' {
				i--
			}
			if i > 0 {
				i++
			}
			t.value = append(t.value[:i], t.value[t.cursor:]...)
			t.cursor = i
		}
	case tea.KeySpace:
		if t.CharLimit == 0 || len(t.value) < t.CharLimit {
			t.value = append(t.value[:t.cursor], append([]rune{' '}, t.value[t.cursor:]...)...)
			t.cursor++
		}
	case tea.KeyRunes:
		for _, r := range keyMsg.Runes {
			if t.CharLimit == 0 || len(t.value) < t.CharLimit {
				t.value = append(t.value[:t.cursor], append([]rune{r}, t.value[t.cursor:]...)...)
				t.cursor++
			}
		}
	}

	return t, nil
}

// View renders the text input with cursor.
func (t TextInput) View() string {
	var sb strings.Builder
	sb.WriteString(t.Prompt)

	if len(t.value) == 0 && t.Placeholder != "" && !t.focused {
		sb.WriteString(styles.InputPlaceholderStyle.Render(t.Placeholder))
		return sb.String()
	}

	if !t.focused {
		sb.WriteString(styles.InputValueStyle.Render(string(t.value)))
		return sb.String()
	}

	// When focused, render with cursor
	val := t.value
	c := t.cursor
	if c > len(val) {
		c = len(val)
	}

	before := string(val[:c])
	sb.WriteString(styles.InputValueStyle.Render(before))

	if c < len(val) {
		currChar := string(val[c])
		cursorStr := styles.New().Bold(true).Foreground(styles.ColorBlack).Background(styles.ColorWhite).Render(currChar)
		sb.WriteString(cursorStr)
		after := string(val[c+1:])
		sb.WriteString(styles.InputValueStyle.Render(after))
	} else {
		// Cursor at the very end
		cursorStr := styles.New().Bold(true).Foreground(styles.ColorBlack).Background(styles.ColorWhite).Render(" ")
		sb.WriteString(cursorStr)
	}

	return sb.String()
}
