package components

import (
	"strings"

	"cmm/internal/tui/styles"
)

// RenderModal places a bordered modal dialog centered within terminal dimensions.
func RenderModal(title, content, footer string, modalWidth, modalHeight, termWidth, termHeight int) string {
	if modalWidth <= 0 {
		modalWidth = 60
	}
	if termWidth > 0 && modalWidth > termWidth-4 {
		modalWidth = termWidth - 4
	}
	if modalWidth < 30 {
		modalWidth = 30
	}

	innerWidth := modalWidth - 4 // minus borders and padding
	if innerWidth < 10 {
		innerWidth = 10
	}

	var sb strings.Builder

	// Title
	if title != "" {
		sb.WriteString(styles.ModalTitleStyle.Render(styles.Pad(" "+title+" ", innerWidth, styles.AlignCenter)))
		sb.WriteString("\n\n")
	}

	// Content lines
	contentLines := strings.Split(content, "\n")
	for _, line := range contentLines {
		sb.WriteString(styles.Pad(styles.Truncate(line, innerWidth, "…"), innerWidth, styles.AlignLeft))
		sb.WriteString("\n")
	}

	// Footer
	if footer != "" {
		sb.WriteString("\n")
		sb.WriteString(styles.HelpDescStyle.Render(styles.Pad(footer, innerWidth, styles.AlignCenter)))
	}

	modalBox := styles.New().
		Border(styles.RoundedBorder).
		BorderForeground(styles.ColorPrimary).
		Padding(1, 2, 1, 2).
		Width(modalWidth).
		Render(sb.String())

	// Center horizontally and vertically within terminal
	boxLines := strings.Split(modalBox, "\n")
	boxHeight := len(boxLines)
	topPad := (termHeight - boxHeight) / 2
	if topPad < 0 {
		topPad = 0
	}

	leftPad := (termWidth - modalWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}

	var result strings.Builder
	for i := 0; i < topPad; i++ {
		result.WriteString("\n")
	}
	for i, line := range boxLines {
		result.WriteString(strings.Repeat(" ", leftPad) + line)
		if i < len(boxLines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}
