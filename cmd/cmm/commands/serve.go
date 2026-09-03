package commands

import (
	"fmt"
	"os"

	"cmm/internal/config"
	"cmm/internal/modrinth"
	"cmm/internal/sync"
	"github.com/spf13/cobra"
)

var (
	servePort  int
	serveToken string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP sync server daemon to share lockfile and accept push deployments",
	Long:  `Hosts a local HTTP server exposing GET /lock for remote cmm sync clients and POST /push for remote deployments.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, _ := config.LoadConfig("cmm.toml")
		if cfg == nil {
			cfg = config.DefaultConfig()
		}

		// Fallback to sync_token from cmm.toml if --token flag was omitted
		if serveToken == "" && cfg.SyncToken != "" {
			serveToken = cfg.SyncToken
		}

		modsDir := cfg.Paths.ModsDir
		if modsDir == "" {
			modsDir = "mods"
		}
		configDir := cfg.Paths.ConfigDir
		if configDir == "" {
			configDir = "config"
		}

		client, _ := modrinth.NewClient("CloudModManager/1.0 (server daemon)")

		srv := sync.NewServer(servePort, serveToken, "cmm.lock")
		srv.ConfigPath = "cmm.toml"
		srv.ModsDir = modsDir
		srv.ConfigDir = configDir
		srv.Client = client

		if err := srv.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running server: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 8080, "Port to listen on")
	serveCmd.Flags().StringVarP(&serveToken, "token", "t", "", "Bearer token required for authorization")
}
