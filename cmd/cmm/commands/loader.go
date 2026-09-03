package commands

import (
	"fmt"
	"os"
	"strings"

	"cmm/internal/config"
	"cmm/internal/loader"
	"github.com/spf13/cobra"
)

var (
	loaderVersion string
	loaderForce   bool
)

var loaderCmd = &cobra.Command{
	Use:   "loader",
	Short: "Manage Minecraft mod loaders (Fabric, Forge, NeoForge, Quilt)",
}

var loaderListCmd = &cobra.Command{
	Use:   "list [loader]",
	Short: "List available versions for a loader",
	Run: func(cmd *cobra.Command, args []string) {
		targetLoader := "all"
		if len(args) > 0 {
			targetLoader = args[0]
		}

		loaders, err := loader.ListLoaders(targetLoader)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing loaders: %v\n", err)
			os.Exit(1)
		}

		if strings.EqualFold(targetLoader, "all") {
			fmt.Println("Supported mod loaders:")
			for _, l := range loaders {
				fmt.Printf("  - %s\n", l)
			}
		} else {
			fmt.Printf("Available versions for %s:\n", targetLoader)
			for _, l := range loaders {
				fmt.Printf("  - %s\n", l)
			}
		}
	},
}

var loaderInstallCmd = &cobra.Command{
	Use:   "install <loader>",
	Short: "Configure or install a mod loader in cmm.toml",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		targetLoader := args[0]

		if err := loader.InstallLoader("cmm.toml", targetLoader, loaderVersion); err != nil {
			fmt.Fprintf(os.Stderr, "Error installing loader version: %v\n", err)
			os.Exit(1)
		}

		if loaderVersion != "" {
			fmt.Printf("Successfully configured loader '%s' (version %s) in cmm.toml\n", targetLoader, loaderVersion)
		} else {
			fmt.Printf("Successfully configured loader '%s' in cmm.toml\n", targetLoader)
		}
	},
}

var loaderUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check and update the configured mod loader to the latest stable version",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig("cmm.toml")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cmm.toml not found. Run 'cmm init' or 'cmm scan' first.\n")
			os.Exit(1)
		}

		targetLoader := strings.ToLower(cfg.Profile.Loader)
		if targetLoader == "" {
			fmt.Fprintf(os.Stderr, "Error: No loader configured in cmm.toml.\n")
			os.Exit(1)
		}

		// Process safety check
		if !loaderForce && loader.IsServerOrMinecraftRunning() {
			fmt.Fprintf(os.Stderr, "Error: Server is currently running. Please stop the server before updating the loader (or use --force).\n")
			os.Exit(1)
		}

		latestVer, available, err := loader.CheckLatestLoaderVersion(targetLoader, cfg.Profile.LoaderVersion)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking loader updates: %v\n", err)
			os.Exit(1)
		}

		if !available {
			currVer := cfg.Profile.LoaderVersion
			if currVer == "" {
				currVer = "latest"
			}
			fmt.Printf("%s Loader is already up to date (%s).\n", strings.Title(targetLoader), currVer)
			return
		}

		currVer := cfg.Profile.LoaderVersion
		if currVer == "" {
			currVer = "unknown"
		}
		fmt.Printf("Current %s Loader: v%s\n", strings.Title(targetLoader), currVer)
		fmt.Printf("Latest %s Loader:  v%s\n", strings.Title(targetLoader), latestVer)

		fmt.Printf("Update %s Loader to v%s? [y/N]: ", strings.Title(targetLoader), latestVer)
		var input string
		fmt.Scanln(&input)
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println("Loader update cancelled.")
			return
		}

		if err := loader.InstallLoader("cmm.toml", targetLoader, latestVer); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating loader: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Successfully updated %s Loader to v%s in cmm.toml\n", strings.Title(targetLoader), latestVer)
	},
}

func init() {
	rootCmd.AddCommand(loaderCmd)
	loaderCmd.AddCommand(loaderListCmd)
	loaderCmd.AddCommand(loaderInstallCmd)
	loaderCmd.AddCommand(loaderUpdateCmd)
	loaderInstallCmd.Flags().StringVarP(&loaderVersion, "version", "v", "", "Specific loader version")
	loaderUpdateCmd.Flags().BoolVarP(&loaderForce, "force", "f", false, "Force update even if server is running")
}
