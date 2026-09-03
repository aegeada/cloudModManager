package commands

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"cmm/internal/config"
	"cmm/internal/tui"
	"cmm/internal/tui/tea"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive terminal user interface",
	Long: `Terminal UI for managing mods and configuration. Supports keybindings for navigation (Tab, Arrow keys, q to quit).

Keybindings:
  Tab / Shift+Tab  Switch between the 4 main tabs
  1 - 4            Jump directly to Tab 1-4 (Mods, Search, Config, Sync)
  ? / F1           Open context-sensitive keybinding help overlay
  Esc              Dismiss active dialog modals or search filter
  q / Ctrl+C       Quit the application cleanly`,
	Run: func(cmd *cobra.Command, args []string) {
		// Verify cmm.toml exists
		if _, err := config.LoadConfig("cmm.toml"); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cmm.toml not found. Run 'cmm init' first.\n")
			os.Exit(1)
		}

		isTerminal := tea.IsTerminal(os.Stdin)

		if !isTerminal {
			// Check if piped stdin has data (e.g. in scripted e2e test execution)
			var buf bytes.Buffer
			n, err := io.Copy(&buf, os.Stdin)
			if err != nil || n == 0 {
				fmt.Println("Running in non-interactive mode.")
				return
			}

			// Piped input available: execute headless program
			app, err := tui.NewApp("cmm.toml", "cmm.lock")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error initializing TUI: %v\n", err)
				os.Exit(1)
			}

			prog := tea.NewProgram(app, tea.WithInput(&buf), tea.WithOutput(os.Stdout), tea.WithoutRenderer())
			if _, err := prog.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Interactive terminal session
		app, err := tui.NewApp("cmm.toml", "cmm.lock")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing TUI: %v\n", err)
			os.Exit(1)
		}

		prog := tea.NewProgram(app, tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout), tea.WithAltScreen())
		if _, err := prog.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
