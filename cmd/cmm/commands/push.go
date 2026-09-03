package commands

import (
	"fmt"
	"os"

	"cmm/internal/config"
	"cmm/internal/sync"
	"github.com/spf13/cobra"
)

var (
	pushURL           string
	pushToken         string
	pushIncludeConfig bool
	pushDryRun        bool
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push local lockfile and configs to remote CMM server daemon",
	Long:  `Packages local cmm.lock and optional config/ directory and pushes them to a remote CMM server daemon (cmm serve), triggering remote delta synchronization.`,
	Run: func(cmd *cobra.Command, args []string) {
		if pushURL == "" {
			fmt.Fprintf(os.Stderr, "Error: --url <http://server:port> is required for push\n")
			os.Exit(1)
		}

		if pushToken == "" {
			cfg, err := config.LoadConfig("cmm.toml")
			if err == nil && cfg != nil && cfg.SyncToken != "" {
				pushToken = cfg.SyncToken
			}
		}

		syncer := sync.NewPushSynchronizer("cmm.toml", "cmm.lock", "config")
		res, err := syncer.Push(sync.PushOptions{
			URL:           pushURL,
			Token:         pushToken,
			IncludeConfig: pushIncludeConfig,
			DryRun:        pushDryRun,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if pushDryRun {
			fmt.Println("[Dry-Run] Remote push simulation result:")
		} else {
			fmt.Println("Successfully pushed modpack to remote server.")
		}

		if res.Message != "" {
			fmt.Printf("Server Message: %s\n", res.Message)
		}
		if len(res.AddedMods) > 0 {
			fmt.Printf("Added Mods (%d):\n", len(res.AddedMods))
			for _, m := range res.AddedMods {
				fmt.Printf("  + %s\n", m)
			}
		}
		if len(res.UpdatedMods) > 0 {
			fmt.Printf("Updated Mods (%d):\n", len(res.UpdatedMods))
			for _, m := range res.UpdatedMods {
				fmt.Printf("  ~ %s\n", m)
			}
		}
		if len(res.PrunedMods) > 0 {
			fmt.Printf("Pruned Mods (%d):\n", len(res.PrunedMods))
			for _, m := range res.PrunedMods {
				fmt.Printf("  - %s\n", m)
			}
		}
		if res.ConfigsUpdated > 0 {
			fmt.Printf("Configs Updated: %d files\n", res.ConfigsUpdated)
		}
	},
}

func init() {
	rootCmd.AddCommand(pushCmd)
	pushCmd.Flags().StringVarP(&pushURL, "url", "u", "", "Remote server URL (e.g. http://127.0.0.1:8080)")
	pushCmd.Flags().StringVarP(&pushToken, "token", "t", "", "Authentication Bearer token")
	pushCmd.Flags().BoolVarP(&pushIncludeConfig, "include-config", "c", false, "Include config/ directory in push payload")
	pushCmd.Flags().BoolVar(&pushDryRun, "dry-run", false, "Simulate push on remote server without applying changes")
	_ = pushCmd.MarkFlagRequired("url")
}
