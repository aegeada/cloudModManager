package commands

import (
	"fmt"
	"os"

	"cmm/internal/config"
	"cmm/internal/sync"
	"github.com/spf13/cobra"
)

var (
	diffURL   string
	diffToken string
	diffLock  string
	diffJSON  bool
)

var diffCmd = &cobra.Command{
	Use:   "diff [file1] [file2]",
	Short: "Audit client-server mod synchronization differences",
	Long: `Compare modpack synchronization between local client and remote CMM server daemon, 
or compare two local lockfiles. Categorizes mods into [OK], [MISMATCH], [CLIENT], [SERVER], and [MISSING].`,
	Run: func(cmd *cobra.Command, args []string) {
		lockPath := diffLock
		if lockPath == "" {
			lockPath = "cmm.lock"
		}

		engine := sync.NewDiffEngine()
		var res *sync.DiffResult
		var err error

		if diffURL != "" {
			// Case 1: Remote server diff
			token := diffToken
			if token == "" {
				cfg, errCfg := config.LoadConfig("cmm.toml")
				if errCfg == nil && cfg != nil && cfg.SyncToken != "" {
					token = cfg.SyncToken
				}
			}
			res, err = engine.CompareRemote(lockPath, diffURL, token)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error comparing with remote server: %v\n", err)
				os.Exit(1)
			}
		} else if len(args) >= 2 {
			// Case 2: Compare two lockfiles
			res, err = engine.CompareFiles(args[0], args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error comparing lockfiles: %v\n", err)
				os.Exit(1)
			}
		} else if len(args) == 1 {
			// Case 3: Compare local cmm.lock against specified lockfile
			res, err = engine.CompareFiles(lockPath, args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error comparing lockfiles: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Case 4: Check if cmm.toml has a sync source with URL configured
			cfg, errCfg := config.LoadConfig("cmm.toml")
			var remoteSource *config.SyncSource
			if errCfg == nil && cfg != nil {
				for i := range cfg.SyncSources {
					if cfg.SyncSources[i].URL != "" {
						remoteSource = &cfg.SyncSources[i]
						break
					}
				}
			}

			if remoteSource != nil {
				token := diffToken
				if token == "" {
					if remoteSource.Token != "" {
						token = remoteSource.Token
					} else if cfg.SyncToken != "" {
						token = cfg.SyncToken
					}
				}
				res, err = engine.CompareRemote(lockPath, remoteSource.URL, token)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error comparing with remote server (%s): %v\n", remoteSource.URL, err)
					os.Exit(1)
				}
			} else {
				fmt.Fprintf(os.Stderr, "Error: specify a remote server --url or lockfile path(s) to compare (e.g. 'cmm diff --url http://127.0.0.1:8080' or 'cmm diff client.lock server.lock')\n")
				os.Exit(1)
			}
		}

		if diffJSON {
			if err := sync.FormatJSON(os.Stdout, res); err != nil {
				fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if res.Target != "" {
			fmt.Printf("Comparing against: %s\n\n", res.Target)
		}
		sync.FormatTable(os.Stdout, res)
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringVarP(&diffURL, "url", "u", "", "Remote server URL to compare against (e.g. http://127.0.0.1:8080)")
	diffCmd.Flags().StringVarP(&diffToken, "token", "t", "", "Authentication Bearer token for remote server")
	diffCmd.Flags().StringVarP(&diffLock, "lock", "l", "cmm.lock", "Path to local lockfile")
	diffCmd.Flags().BoolVar(&diffJSON, "json", false, "Output diff result as structured JSON")
}
