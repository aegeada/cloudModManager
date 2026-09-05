package commands

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"

	"cmm/internal/selfupdate"
	"github.com/spf13/cobra"
)

var (
	versionCheckOnly bool
	versionYes       bool
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display Cloud Mod Manager version and check for new releases",
	Long:  `Displays current version, checks GitHub for new releases, and allows updating the cmm executable in place.`,
	Run: func(cmd *cobra.Command, args []string) {
		currentVersion := rootCmd.Version
		if currentVersion == "" {
			currentVersion = "0.1.0"
		}
		if !strings.HasPrefix(currentVersion, "v") {
			currentVersion = "v" + currentVersion
		}

		fmt.Printf("Cloud Mod Manager %s (%s/%s)\n", currentVersion, runtime.GOOS, runtime.GOARCH)

		fmt.Println("🔍 Checking for updates...")
		rel, updateAvailable, err := selfupdate.CheckUpdate("aegeada/cloudModManager", currentVersion)
		if err != nil {
			fmt.Printf("ℹ️  (Could not check for updates: %v)\n", err)
			return
		}

		if !updateAvailable {
			fmt.Printf("✅ Cloud Mod Manager is up to date (%s).\n", currentVersion)
			return
		}

		fmt.Printf("\n🚀 A new version of Cloud Mod Manager is available: %s (Current: %s)\n", rel.TagName, currentVersion)
		if rel.HTMLURL != "" {
			fmt.Printf("Release details: %s\n", rel.HTMLURL)
		}

		if versionCheckOnly {
			return
		}

		shouldUpdate := versionYes
		if !shouldUpdate {
			reader := bufio.NewReader(os.Stdin)
			fmt.Printf("\nDownload and install %s now? [y/N]: ", rel.TagName)
			input, err := reader.ReadString('\n')
			if err != nil && input == "" {
				fmt.Println("Update cancelled.")
				return
			}
			input = strings.TrimSpace(strings.ToLower(input))
			shouldUpdate = input == "y" || input == "yes"
		}

		if !shouldUpdate {
			fmt.Println("Update cancelled.")
			return
		}

		fmt.Printf("⬇️  Downloading and installing %s...\n", rel.TagName)
		if err := selfupdate.ApplyUpdate(rel); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating cmm: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✨ Successfully updated Cloud Mod Manager to %s!\n", rel.TagName)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolVarP(&versionCheckOnly, "check", "c", false, "Only check for updates without prompting to install")
	versionCmd.Flags().BoolVarP(&versionYes, "yes", "y", false, "Automatically confirm update without prompt")
}
