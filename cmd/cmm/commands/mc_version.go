package commands

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"cmm/internal/config"
	"cmm/internal/loader"
	"github.com/spf13/cobra"
)

var (
	mcVersionRegex = regexp.MustCompile(`^(1\.[0-9]+(\.[0-9]+)?(-(pre|rc)\.?[0-9]+)?|2[0-9]\.[0-9]+(\.[0-9]+)?|[0-9]{2}w[0-9]{2}[a-z])$`)
	mcVersionYes   bool
	mcVersionForce bool
)

var mcVersionCmd = &cobra.Command{
	Use:   "mc-version [version]",
	Short: "View or update the configured Minecraft version in cmm.toml",
	Long:  `View the configured Minecraft version, or safely update it after confirmation and format validation.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig("cmm.toml")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cmm.toml not found. Run 'cmm init' or 'cmm scan' first.\n")
			os.Exit(1)
		}

		if len(args) == 0 {
			currentVer := cfg.Profile.MinecraftVersion
			if currentVer == "" {
				currentVer = "not configured"
			}
			fmt.Printf("Configured Minecraft version: %s\n", currentVer)
			return
		}

		newVer := strings.TrimSpace(args[0])
		if !mcVersionRegex.MatchString(newVer) {
			fmt.Fprintf(os.Stderr, "Error: invalid Minecraft version format '%s'. Example formats: 1.21.1, 1.21.2-rc1, 26.2, 24w14a\n", newVer)
			os.Exit(1)
		}

		// Process safety check
		if !mcVersionForce && loader.IsServerOrMinecraftRunning() {
			fmt.Fprintf(os.Stderr, "Error: Server is currently running. Please stop the server before changing Minecraft version (or use --force).\n")
			os.Exit(1)
		}

		oldVer := cfg.Profile.MinecraftVersion
		if oldVer == "" {
			oldVer = "none"
		}

		if strings.EqualFold(oldVer, newVer) {
			fmt.Printf("Minecraft version is already set to %s.\n", newVer)
			return
		}

		if !mcVersionYes {
			fmt.Printf("Change configured Minecraft version from %s to %s? [y/N]: ", oldVer, newVer)
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil && input == "" {
				fmt.Println("Minecraft version change cancelled.")
				return
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "y" && input != "yes" {
				fmt.Println("Minecraft version change cancelled.")
				return
			}
		}

		cfg.Profile.MinecraftVersion = newVer
		if err := config.SaveConfig("cmm.toml", cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving cmm.toml: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Minecraft version updated to %s in cmm.toml.\n", newVer)
	},
}

func init() {
	rootCmd.AddCommand(mcVersionCmd)
	mcVersionCmd.Flags().BoolVarP(&mcVersionYes, "yes", "y", false, "Automatically confirm Minecraft version change")
	mcVersionCmd.Flags().BoolVarP(&mcVersionForce, "force", "f", false, "Force update even if server is running")
}
