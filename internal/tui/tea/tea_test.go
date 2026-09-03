package tea

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

type testModel struct {
	text   string
	quited bool
}

func (m testModel) Init() Cmd {
	return func() Msg {
		return "init-msg"
	}
}

func (m testModel) Update(msg Msg) (Model, Cmd) {
	switch msg := msg.(type) {
	case string:
		m.text = msg
	case KeyMsg:
		if msg.String() == "q" {
			m.quited = true
			return m, func() Msg { return Quit() }
		}
	}
	return m, nil
}

func (m testModel) View() string {
	return "View: " + m.text
}

func TestTea_ProgramRun(t *testing.T) {
	inBuf := strings.NewReader("q")
	var outBuf bytes.Buffer

	model := testModel{text: "start"}
	prog := NewProgram(model, WithInput(inBuf), WithOutput(&outBuf), WithoutRenderer())

	finalModel, err := prog.Run()
	if err != nil {
		t.Fatalf("Program Run failed: %v", err)
	}

	tm, ok := finalModel.(testModel)
	if !ok || !tm.quited {
		t.Errorf("Expected model to have quited, got %+v", finalModel)
	}
}

func TestTea_BatchAndSequence(t *testing.T) {
	cmd1 := func() Msg { return "cmd1" }
	cmd2 := func() Msg { return "cmd2" }

	batch := Batch(cmd1, cmd2)
	if batch == nil {
		t.Errorf("Expected non-nil batch")
	}

	seq := Sequence(cmd1, cmd2)
	if seq == nil {
		t.Errorf("Expected non-nil sequence")
	}
}

func TestTea_KeyMsgStrings(t *testing.T) {
	cases := []struct {
		msg      KeyMsg
		expected string
	}{
		{KeyMsg{Type: KeyEnter}, "enter"},
		{KeyMsg{Type: KeyTab}, "tab"},
		{KeyMsg{Type: KeyShiftTab}, "shift+tab"},
		{KeyMsg{Type: KeyEsc}, "esc"},
		{KeyMsg{Type: KeyUp}, "up"},
		{KeyMsg{Type: KeyDown}, "down"},
		{KeyMsg{Type: KeyLeft}, "left"},
		{KeyMsg{Type: KeyRight}, "right"},
		{KeyMsg{Type: KeyCtrlC}, "ctrl+c"},
		{KeyMsg{Type: KeyF1}, "f1"},
		{KeyMsg{Type: KeyRunes, Runes: []rune{'a'}}, "a"},
	}

	for _, c := range cases {
		if c.msg.String() != c.expected {
			t.Errorf("Expected string '%s', got '%s'", c.expected, c.msg.String())
		}
	}

	if !KeyMatches(KeyMsg{Type: KeyEnter}, "enter", "return") {
		t.Errorf("KeyMatches should match 'enter'")
	}
}

func TestTea_Tick(t *testing.T) {
	cmd := Tick(10*time.Millisecond, func(tm time.Time) Msg {
		return "ticked"
	})
	msg := cmd()
	if msg != "ticked" {
		t.Errorf("Expected 'ticked', got %v", msg)
	}
}
