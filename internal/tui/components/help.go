package components

import (
	"fmt"
	"strings"

	"cmm/internal/tui/styles"
)

// RenderHelpModal returns a formatted help dialog with keybindings.
func RenderHelpModal(termWidth, termHeight int) string {
	var sb strings.Builder

	addSection := func(title string, bindings [][2]string) {
		sb.WriteString(styles.New().Bold(true).Foreground(styles.ColorCyan).Render("── " + title + " ──"))
		sb.WriteString("\n")
		for _, b := range bindings {
			key := styles.HelpKeyStyle.Render(fmt.Sprintf("%-14s", b[0]))
			desc := styles.HelpDescStyle.Render(b[1])
			sb.WriteString(fmt.Sprintf("  %s %s\n", key, desc))
		}
		sb.WriteString("\n")
	}

	addSection("Global Navigation", [][2]string{
		{"Tab / Shift+Tab", "Next / Previous tab"},
		{"1, 2, 3, 4", "Jump directly to Tab 1-4"},
		{"? / F1", "Toggle this help dialog"},
		{"Esc", "Dismiss modals or clear filter"},
		{"q / Ctrl+C", "Quit application"},
	})

	addSection("Tab 1: Installed Mods", [][2]string{
		{"↑ / ↓ / j / k", "Navigate mods list"},
		{"p", "Toggle pin/unpin mod status"},
		{"d / x", "Delete mod (prompts confirmation)"},
		{"u", "Check for updates & apply"},
		{"Enter", "View detailed mod metadata"},
		{"/", "Filter mods list live"},
	})

	addSection("Tab 2: Modrinth Search", [][2]string{
		{"Typing", "Live debounced search"},
		{"↑ / ↓", "Navigate search results"},
		{"PgUp / PgDn", "Scroll results list"},
		{"Enter", "Open version picker modal"},
		{"i", "Install latest compatible version"},
	})

	addSection("Tab 3: Config Editor", [][2]string{
		{"↑ / ↓ / Tab", "Navigate between fields"},
		{"Space", "Cycle loader / side options"},
		{"s / Enter", "Validate and save cmm.toml"},
		{"r", "Reload/discard changes from disk"},
	})

	addSection("Tab 4: Sync & Tools Dashboard", [][2]string{
		{"1 - 7", "Select workflow (1:Scan, 2:Pack, 3:Git, 4:Pull, 5:Push, 6:Diff, 7:Launcher)"},
		{"Enter / x", "Execute active workflow action"},
		{"Tab / Shift+Tab", "Navigate fields, checkboxes, and tables"},
		{"Space", "Toggle option checkboxes (Include Config, Dry Run)"},
		{"s / Enter", "Sync modpack to selected launcher instance (Tab 7)"},
		{"r", "Rescan launcher instances (Tab 7)"},
		{"f / d", "Toggle force / dry-run options (Tab 7)"},
		{"↑ / ↓ / j / k", "Navigate diff & launcher tables / scroll logs"},
	})

	return RenderModal(
		"Help & Keybindings",
		strings.TrimSpace(sb.String()),
		"Press Esc or ? to close",
		74,
		28,
		termWidth,
		termHeight,
	)
}
