package commands

import (
	"fmt"
	"os"

	"cmm/internal/selfinstall"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "cmm",
	Short:   "Cloud Mod Manager",
	Long:    `A CLI tool for managing Minecraft mods on a server and syncing modpacks.`,
	Version: "0.1.0",
}

var selfInstallCmd = &cobra.Command{
	Use:     "self-install",
	Aliases: []string{"install-self"},
	Short:   "Install cmm binary into ~/.local/bin and configure PATH",
	Run: func(cmd *cobra.Command, args []string) {
		if err := selfinstall.ManualInstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Error installing cmm: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Successfully installed cmm to ~/.local/bin/cmm!\n")
	},
}

func init() {
	rootCmd.AddCommand(selfInstallCmd)
}

func Execute() {
	// Auto-install to ~/.local/bin on first standalone run
	selfinstall.AutoInstall()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
