package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"cmm/internal/config"
	"cmm/internal/loader"
	"cmm/internal/mod"
	"cmm/internal/modrinth"
	"github.com/spf13/cobra"
)

var (
	updateForce   bool
	updateChannel string
)

var updateCmd = &cobra.Command{
	Use:   "update [slug...]",
	Short: "Update installed mods to their latest compatible versions",
	Run: func(cmd *cobra.Command, args []string) {
		channel := strings.ToLower(strings.TrimSpace(updateChannel))
		if channel == "" {
			channel = "release"
		}
		if channel != "release" && channel != "beta" && channel != "alpha" {
			fmt.Fprintf(os.Stderr, "Error: invalid channel '%s'. Choose from: release, beta, alpha\n", updateChannel)
			os.Exit(1)
		}

		fmt.Printf("🔍 Checking for updates... [Channel: %s]\n", channel)

		userAgent := "CloudModManager/1.0 (contact: user@domain.local)"
		client, err := modrinth.NewClient(userAgent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating Modrinth client: %v\n", err)
			os.Exit(1)
		}

		mgr := mod.NewManager(client, "cmm.toml", "cmm.lock")

		candidates, skippedPinned, err := mgr.CheckUpdatesMulti(args, channel, updateForce)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking updates: %v\n", err)
			os.Exit(1)
		}

		for _, s := range skippedPinned {
			fmt.Printf("Skipping pinned mod '%s' - use --force to update (skipped)\n", s)
		}

		if len(candidates) == 0 {
			fmt.Println("All mods are up to date.")
		} else {
			fmt.Printf("Found %d available update(s):\n\n", len(candidates))
			for _, c := range candidates {
				slugStr := c.Mod.Slug
				if slugStr == "" {
					slugStr = c.Mod.Name
				}
				nameStr := c.Mod.Name
				if nameStr == "" {
					nameStr = slugStr
				}
				fmt.Printf("- %s (%s): %s -> %s\n", nameStr, slugStr, c.Mod.GetVersion(), c.TargetVersion.VersionNumber)
				if c.Changelog != "" {
					fmt.Printf("  Changelog:\n    %s\n\n", strings.ReplaceAll(strings.TrimSpace(c.Changelog), "\n", "\n    "))
				}
			}

			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Apply updates? [y/N]: ")
			input, err := reader.ReadString('\n')
			if err != nil && input == "" {
				fmt.Println("Update cancelled.")
				return
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "y" && input != "yes" {
				fmt.Println("Update cancelled.")
				return
			}

			if err := mgr.ApplyUpdates(candidates); err != nil {
				fmt.Fprintf(os.Stderr, "Error applying updates: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Successfully updated mods.")
		}

		// Notice: Check if a newer Loader version is available
		if cfg, err := config.LoadConfig("cmm.toml"); err == nil && cfg.Profile.Loader != "" {
			if latestVer, available, _ := loader.CheckLatestLoaderVersion(cfg.Profile.Loader, cfg.Profile.LoaderVersion); available {
				fmt.Printf("\nNotice: A new %s Loader version is available (v%s). Run: cmm loader update\n", strings.Title(cfg.Profile.Loader), latestVer)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVarP(&updateForce, "force", "f", false, "Force update pinned mods")
	updateCmd.Flags().StringVarP(&updateChannel, "channel", "c", "release", "Stability channel filter (release, beta, alpha)")
}
