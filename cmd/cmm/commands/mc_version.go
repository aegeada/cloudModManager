package commands

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"cmm/internal/config"
	"github.com/spf13/cobra"
)

var mcVersionRegex = regexp.MustCompile(`^(1\.[0-9]+(\.[0-9]+)?|2[0-9]\.[0-9]+(\.[0-9]+)?|[0-9]{2}w[0-9]{2}[a-z])$`)

var mcVersionCmd = &cobra.Command{
	Use:   "mc-version [version]",
	Short: "View or update the configured Minecraft version in cmm.toml",
	Long: `View the configured Minecraft version, or safely update it after confirmation and format validation.`,
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
			fmt.Fprintf(os.Stderr, "Error: invalid Minecraft version format '%s'. Example formats: 1.21.1, 26.2, 24w14a\n", newVer)
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

		fmt.Printf("Change configured Minecraft version from %s to %s? [y/N]: ", oldVer, newVer)
		var input string
		fmt.Scanln(&input)
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println("Minecraft version change cancelled.")
			return
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
}
