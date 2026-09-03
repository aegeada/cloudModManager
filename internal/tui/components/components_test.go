package components

import (
	"strings"
	"testing"
	"time"

	"cmm/internal/tui/styles"
	"cmm/internal/tui/tea"
)

func TestTextInput(t *testing.T) {
	ti := NewTextInput()
	ti.Focus()

	// Type "hello world"
	for _, r := range "hello world" {
		ti, _ = ti.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if ti.Value() != "hello world" {
		t.Errorf("Expected value 'hello world', got '%s'", ti.Value())
	}

	// Backspace
	ti, _ = ti.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if ti.Value() != "hello worl" {
		t.Errorf("Expected value 'hello worl', got '%s'", ti.Value())
	}

	// Move Left and Insert
	ti, _ = ti.Update(tea.KeyMsg{Type: tea.KeyLeft})
	ti, _ = ti.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if ti.Value() != "hello worXl" {
		t.Errorf("Expected value 'hello worXl', got '%s'", ti.Value())
	}

	// Ctrl+W: delete word
	ti, _ = ti.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	if ti.Value() != "hello l" {
		t.Errorf("Expected 'hello l' after Ctrl+W, got '%s'", ti.Value())
	}

	// Ctrl+U: delete to start (cursor is at 'l')
	ti, _ = ti.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	if ti.Value() != "l" {
		t.Errorf("Expected 'l' after Ctrl+U from middle, got '%s'", ti.Value())
	}

	// Reset
	ti.Reset()
	if ti.Value() != "" {
		t.Errorf("Expected empty string after Reset, got '%s'", ti.Value())
	}

	// View rendering
	ti.SetValue("test input")
	view := styles.StripANSI(ti.View())
	if !strings.Contains(view, "test input") {
		t.Errorf("Expected view to contain 'test input', got '%s'", view)
	}
}

func TestTable(t *testing.T) {
	cols := []Column{
		{Title: "COL1", Width: 10},
		{Title: "COL2", Width: 15},
	}
	tbl := NewTable(cols, 5)

	rows := [][]string{
		{"r1c1", "r1c2"},
		{"r2c1", "r2c2"},
		{"r3c1", "r3c2"},
		{"r4c1", "r4c2"},
		{"r5c1", "r5c2"},
	}
	tbl.SetRows(rows)

	if tbl.Cursor() != 0 {
		t.Errorf("Expected cursor 0, got %d", tbl.Cursor())
	}

	// Move Down
	tbl, _ = tbl.Update(tea.KeyMsg{Type: tea.KeyDown})
	if tbl.Cursor() != 1 {
		t.Errorf("Expected cursor 1, got %d", tbl.Cursor())
	}

	// Page Down
	tbl, _ = tbl.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if tbl.Cursor() < 2 {
		t.Errorf("Expected cursor >= 2 after PageDown, got %d", tbl.Cursor())
	}

	// Goto Top
	tbl.GotoTop()
	if tbl.Cursor() != 0 {
		t.Errorf("Expected cursor 0 after GotoTop, got %d", tbl.Cursor())
	}

	// Selected Row
	sel := tbl.SelectedRow()
	if len(sel) != 2 || sel[0] != "r1c1" {
		t.Errorf("Expected selected row ['r1c1', 'r1c2'], got %v", sel)
	}

	// View
	view := styles.StripANSI(tbl.View())
	if !strings.Contains(view, "COL1") || !strings.Contains(view, "r1c1") {
		t.Errorf("Expected header and row in view, got:\n%s", view)
	}
}

func TestSpinner(t *testing.T) {
	sp := NewSpinner()
	if len(sp.View()) == 0 {
		t.Errorf("Expected non-empty spinner view")
	}

	sp, cmd := sp.Update(SpinnerTickMsg{Time: time.Now()})
	if cmd == nil {
		t.Errorf("Expected next tick cmd from spinner update")
	}
}

func TestViewport(t *testing.T) {
	vp := NewViewport(40, 5)
	vp.SetContent("Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7")

	if vp.yOffset != 0 {
		t.Errorf("Expected initial yOffset 0, got %d", vp.yOffset)
	}

	vp.GotoBottom()
	if vp.yOffset != 2 { // 7 - 5 = 2
		t.Errorf("Expected yOffset 2 after GotoBottom, got %d", vp.yOffset)
	}

	vp.GotoTop()
	if vp.yOffset != 0 {
		t.Errorf("Expected yOffset 0 after GotoTop, got %d", vp.yOffset)
	}

	vp.ScrollDown(2)
	if vp.yOffset != 2 {
		t.Errorf("Expected yOffset 2 after ScrollDown(2), got %d", vp.yOffset)
	}

	vp.Append("Line 8")
	view := styles.StripANSI(vp.View())
	if !strings.Contains(view, "Line 8") {
		t.Errorf("Expected 'Line 8' in view after append, got:\n%s", view)
	}
}

func TestModalAndHelp(t *testing.T) {
	modal := RenderModal("Test Title", "Line of text inside modal", "Footer key hint", 50, 10, 80, 24)
	cleanModal := styles.StripANSI(modal)
	if !strings.Contains(cleanModal, "Test Title") || !strings.Contains(cleanModal, "Line of text") {
		t.Errorf("Modal missing title or content:\n%s", cleanModal)
	}

	help := RenderHelpModal(80, 24)
	cleanHelp := styles.StripANSI(help)
	if !strings.Contains(cleanHelp, "Help & Keybindings") || !strings.Contains(cleanHelp, "Tab 1: Installed Mods") {
		t.Errorf("Help modal missing sections:\n%s", cleanHelp)
	}
}
