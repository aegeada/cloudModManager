package commands

import (
	"fmt"
	"os"

	"cmm/internal/mod"
	"cmm/internal/modrinth"
	"github.com/spf13/cobra"
)

var (
	enableDryRun bool
)

var enableCmd = &cobra.Command{
	Use:     "enable <mod-slug-or-name>",
	Aliases: []string{"activate"},
	Short:   "Enable a disabled mod by restoring its .jar filename",
	Long: `Enable restores a disabled mod by renaming its file from .jar.disabled back to .jar in the mods directory and updating cmm.lock.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		userAgent := "CloudModManager/1.0 (contact: user@domain.local)"
		client, _ := modrinth.NewClient(userAgent)
		mgr := mod.NewManager(client, "cmm.toml", "cmm.lock")

		for _, arg := range args {
			if enableDryRun {
				fmt.Printf("[Dry-Run] Would enable mod '%s'\n", arg)
				continue
			}

			res, err := mgr.EnableMod(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error enabling mod '%s': %v\n", arg, err)
				os.Exit(1)
			}

			if res.Warning != "" {
				fmt.Printf("ℹ️  %s\n", res.Warning)
			} else {
				fmt.Printf("✅ Successfully enabled %s (%s -> %s)\n", res.ModName, res.OldFileName, res.NewFileName)
			}
		}
	},
}

func init() {
	enableCmd.Flags().BoolVar(&enableDryRun, "dry-run", false, "Simulate enabling mod without renaming files or updating lockfile")
	rootCmd.AddCommand(enableCmd)
}
