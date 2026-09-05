package commands

import (
	"fmt"
	"os"
	"strings"

	"cmm/internal/mod"
	"cmm/internal/modrinth"
	"github.com/spf13/cobra"
)

var (
	disableForce  bool
	disableDryRun bool
)

var disableCmd = &cobra.Command{
	Use:     "disable <mod-slug-or-name>",
	Aliases: []string{"deactivate"},
	Short:   "Disable an active mod by renaming its file to .jar.disabled",
	Long:    `Disable renames the target mod's file from .jar to .jar.disabled in the mods directory and updates cmm.lock so Minecraft skips loading it.`,
	Args:    cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		userAgent := "CloudModManager/1.0 (contact: user@domain.local)"
		client, _ := modrinth.NewClient(userAgent)
		mgr := mod.NewManager(client, "cmm.toml", "cmm.lock")

		for _, arg := range args {
			if disableDryRun {
				fmt.Printf("[Dry-Run] Would disable mod '%s'\n", arg)
				continue
			}

			res, err := mgr.DisableMod(arg, disableForce)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error disabling mod '%s': %v\n", arg, err)
				os.Exit(1)
			}

			if res.HasDependentsWarning {
				fmt.Fprintf(os.Stderr, "⚠️  Warning: Cannot disable '%s': active mod(s) depend on it: %s (use --force to bypass)\n", res.Name, strings.Join(res.ActiveDependents, ", "))
				continue
			}

			if res.AlreadyDisabled {
				fmt.Printf("ℹ️  Mod '%s' is already disabled\n", res.Name)
				continue
			}

			fmt.Printf("⏸️  Successfully disabled %s (%s -> %s)\n", res.Name, res.OldFileName, res.NewFileName)
			if len(res.ActiveDependents) > 0 {
				fmt.Printf("⚠️  Warning: The following enabled mods depend on '%s': %s\n", res.Name, strings.Join(res.ActiveDependents, ", "))
			}
		}
	},
}

func init() {
	disableCmd.Flags().BoolVarP(&disableForce, "force", "f", false, "Force disable even if other active mods depend on this mod")
	disableCmd.Flags().BoolVar(&disableDryRun, "dry-run", false, "Simulate disabling mod without renaming files or updating lockfile")
	rootCmd.AddCommand(disableCmd)
}
