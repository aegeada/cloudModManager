package commands

import (
	"fmt"
	"os"

	"cmm/internal/modrinth"
	"cmm/internal/sync"
	"github.com/spf13/cobra"
)

var (
	syncSource string
	syncPath   string
	syncSlug   string
	syncFile   string
	syncRepo   string
	syncBranch string
	syncToken  string
	syncURL    string
)

var syncCmd = &cobra.Command{
	Use:   "sync [subcommand]",
	Short: "Synchronize mods with local directory, Modrinth modpack, GitHub, or remote server",
	Run: func(cmd *cobra.Command, args []string) {
		userAgent := "CloudModManager/1.0 (contact: user@domain.local)"
		client, err := modrinth.NewClient(userAgent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating Modrinth client: %v\n", err)
			os.Exit(1)
		}

		// Mode 1: Remote URL sync (--url)
		if syncURL != "" {
			syncer := sync.NewRemoteSynchronizer(client, "cmm.toml", "cmm.lock")
			res, err := syncer.Sync(sync.RemoteSyncOptions{
				URL:   syncURL,
				Token: syncToken,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error syncing with remote server: %v\n", err)
				os.Exit(1)
			}
			if res.UpToDate {
				fmt.Println(res.Message)
			} else {
				fmt.Println("Successfully synchronized with remote server.")
			}
			return
		}

		// Mode 2: GitHub sync (--source github or --repo provided)
		if syncSource == "github" || syncRepo != "" {
			syncer := sync.NewGitHubSynchronizer(client, "cmm.toml", "cmm.lock")
			res, err := syncer.Sync(sync.GitHubSyncOptions{
				Repo:   syncRepo,
				Branch: syncBranch,
				Token:  syncToken,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if res.UpToDate {
				fmt.Println(res.Message)
			} else {
				fmt.Println("Successfully synchronized with GitHub repository.")
			}
			return
		}

		// Mode 3: Modrinth modpack sync (--source modrinth or --slug / --file provided)
		if syncSource == "modrinth" || syncSlug != "" || syncFile != "" {
			syncer := sync.NewModrinthSynchronizer(client, "cmm.toml", "cmm.lock")
			_, err := syncer.Sync(sync.ModrinthSyncOptions{
				Slug:     syncSlug,
				FilePath: syncFile,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Successfully synchronized with Modrinth modpack.")
			return
		}

		// Mode 4: Local sync fallback (--source local or default when no other mode specified)
		if syncSource == "local" || len(args) == 0 {
			runLocalSync(client, syncPath)
			return
		}

		fmt.Fprintf(os.Stderr, "Error: unknown sync source or invalid flags\n")
		os.Exit(1)
	},
}

var syncLocalCmd = &cobra.Command{
	Use:   "local",
	Short: "Synchronize local JAR files with Modrinth and generate cmm.lock",
	Run: func(cmd *cobra.Command, args []string) {
		userAgent := "CloudModManager/1.0 (contact: user@domain.local)"
		client, err := modrinth.NewClient(userAgent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating Modrinth client: %v\n", err)
			os.Exit(1)
		}
		runLocalSync(client, syncPath)
	},
}

func runLocalSync(client *modrinth.Client, path string) {
	syncer := sync.NewLocalSynchronizer(client, "cmm.toml", "cmm.lock")
	res, err := syncer.Sync(sync.LocalSyncOptions{Path: path})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if res.Message != "" {
		fmt.Println(res.Message)
		return
	}

	for _, jar := range res.UnknownJars {
		fmt.Printf("Warning: Unrecognized JAR file: %s (unknown)\n", jar)
	}

	fmt.Printf("Successfully synchronized %d mods.\n", len(res.AddedMods))
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.AddCommand(syncLocalCmd)

	syncCmd.PersistentFlags().StringVarP(&syncSource, "source", "s", "", "Sync source type (local, modrinth, github, remote)")
	syncCmd.PersistentFlags().StringVarP(&syncPath, "path", "p", "", "Path to local mods directory")
	syncCmd.PersistentFlags().StringVar(&syncSlug, "slug", "", "Modrinth modpack slug")
	syncCmd.PersistentFlags().StringVarP(&syncFile, "file", "f", "", "Path to .mrpack modpack archive")
	syncCmd.PersistentFlags().StringVarP(&syncRepo, "repo", "r", "", "GitHub repository (owner/repo)")
	syncCmd.PersistentFlags().StringVarP(&syncBranch, "branch", "b", "main", "GitHub branch")
	syncCmd.PersistentFlags().StringVarP(&syncToken, "token", "t", "", "Authentication token")
	syncCmd.PersistentFlags().StringVarP(&syncURL, "url", "u", "", "Remote server URL")

	syncLocalCmd.Flags().StringVarP(&syncPath, "path", "p", "", "Path to local mods directory")
}
