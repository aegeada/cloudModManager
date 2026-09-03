package components

import (
	"strings"

	"cmm/internal/tui/styles"
	"cmm/internal/tui/tea"
)

// Column defines header title and width for a table column.
type Column struct {
	Title string
	Width int
}

// Table renders a scrollable list of rows with highlighted cursor.
type Table struct {
	Columns      []Column
	Rows         [][]string
	cursor       int
	scrollOffset int
	height       int
	width        int
	focused      bool
}

// NewTable creates a new Table.
func NewTable(cols []Column, height int) Table {
	return Table{
		Columns: cols,
		Rows:    [][]string{},
		height:  height,
		focused: true,
	}
}

func (t *Table) SetColumns(cols []Column) {
	t.Columns = cols
}

func (t *Table) SetRows(rows [][]string) {
	t.Rows = rows
	if t.cursor >= len(rows) {
		t.cursor = len(rows) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
	t.adjustScroll()
}

func (t *Table) SetHeight(h int) {
	if h < 3 {
		h = 3
	}
	t.height = h
	t.adjustScroll()
}

func (t *Table) SetWidth(w int) {
	t.width = w
}

func (t *Table) Cursor() int {
	return t.cursor
}

func (t *Table) SetCursor(c int) {
	if len(t.Rows) == 0 {
		t.cursor = 0
		t.scrollOffset = 0
		return
	}
	if c < 0 {
		c = 0
	}
	if c >= len(t.Rows) {
		c = len(t.Rows) - 1
	}
	t.cursor = c
	t.adjustScroll()
}

func (t *Table) SelectedRow() []string {
	if len(t.Rows) == 0 || t.cursor < 0 || t.cursor >= len(t.Rows) {
		return nil
	}
	return t.Rows[t.cursor]
}

func (t *Table) Focus() {
	t.focused = true
}

func (t *Table) Blur() {
	t.focused = false
}

func (t *Table) Focused() bool {
	return t.focused
}

func (t *Table) MoveUp() {
	if t.cursor > 0 {
		t.cursor--
		t.adjustScroll()
	}
}

func (t *Table) MoveDown() {
	if t.cursor < len(t.Rows)-1 {
		t.cursor++
		t.adjustScroll()
	}
}

func (t *Table) PageUp() {
	pageSize := t.height - 1
	if pageSize < 1 {
		pageSize = 1
	}
	t.cursor -= pageSize
	if t.cursor < 0 {
		t.cursor = 0
	}
	t.adjustScroll()
}

func (t *Table) PageDown() {
	pageSize := t.height - 1
	if pageSize < 1 {
		pageSize = 1
	}
	t.cursor += pageSize
	if t.cursor >= len(t.Rows) {
		t.cursor = len(t.Rows) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
	t.adjustScroll()
}

func (t *Table) GotoTop() {
	t.cursor = 0
	t.adjustScroll()
}

func (t *Table) GotoBottom() {
	if len(t.Rows) > 0 {
		t.cursor = len(t.Rows) - 1
		t.adjustScroll()
	}
}

func (t *Table) adjustScroll() {
	visibleRows := t.height - 1 // 1 for header
	if visibleRows < 1 {
		visibleRows = 1
	}

	if t.cursor < t.scrollOffset {
		t.scrollOffset = t.cursor
	} else if t.cursor >= t.scrollOffset+visibleRows {
		t.scrollOffset = t.cursor - visibleRows + 1
	}

	if t.scrollOffset < 0 {
		t.scrollOffset = 0
	}
}

// Update processes table navigation keys.
func (t Table) Update(msg tea.Msg) (Table, tea.Cmd) {
	if !t.focused {
		return t, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return t, nil
	}

	switch keyMsg.Type {
	case tea.KeyUp:
		t.MoveUp()
	case tea.KeyDown:
		t.MoveDown()
	case tea.KeyPgUp:
		t.PageUp()
	case tea.KeyPgDown:
		t.PageDown()
	case tea.KeyHome:
		t.GotoTop()
	case tea.KeyEnd:
		t.GotoBottom()
	case tea.KeyRunes:
		switch keyMsg.String() {
		case "k":
			t.MoveUp()
		case "j":
			t.MoveDown()
		case "g":
			t.GotoTop()
		case "G":
			t.GotoBottom()
		}
	}

	return t, nil
}

// View renders the formatted table.
func (t Table) View() string {
	var sb strings.Builder

	// Render Header
	var headerCols []string
	for _, col := range t.Columns {
		title := styles.Pad(col.Title, col.Width, styles.AlignLeft)
		headerCols = append(headerCols, title)
	}
	headerLine := strings.Join(headerCols, "  ")
	sb.WriteString(styles.TableHeaderStyle.Render(headerLine))
	sb.WriteString("\n")

	if len(t.Rows) == 0 {
		sb.WriteString(styles.TableRowDim.Render("  (No entries)"))
		return sb.String()
	}

	visibleRows := t.height - 1
	if visibleRows < 1 {
		visibleRows = 1
	}

	endIdx := t.scrollOffset + visibleRows
	if endIdx > len(t.Rows) {
		endIdx = len(t.Rows)
	}

	for i := t.scrollOffset; i < endIdx; i++ {
		row := t.Rows[i]
		var formattedCols []string
		for colIdx, col := range t.Columns {
			val := ""
			if colIdx < len(row) {
				val = row[colIdx]
			}
			val = styles.Truncate(val, col.Width, "…")
			val = styles.Pad(val, col.Width, styles.AlignLeft)
			formattedCols = append(formattedCols, val)
		}
		rowStr := strings.Join(formattedCols, "  ")
		if i == t.cursor && t.focused {
			sb.WriteString(styles.TableRowSelected.Render(rowStr))
		} else {
			sb.WriteString(styles.TableRowNormal.Render(rowStr))
		}
		if i < endIdx-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
