package commands

import (
	"fmt"
	"os"
	"strings"

	"cmm/internal/config"
	"cmm/internal/modrinth"
	"github.com/spf13/cobra"
)

var (
	searchLimit int
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for mods on Modrinth",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := strings.Join(args, " ")

		userAgent := "CloudModManager/1.0 (contact: user@domain.local)"
		client, err := modrinth.NewClient(userAgent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating Modrinth client: %v\n", err)
			os.Exit(1)
		}

		var facets [][]string
		facets = append(facets, []string{"project_type:mod"})

		// Read cmm.toml if exists to apply loader & mc_version filters
		if cfg, err := config.LoadConfig("cmm.toml"); err == nil {
			if cfg.Profile.Loader != "" {
				facets = append(facets, []string{fmt.Sprintf("categories:%s", strings.ToLower(cfg.Profile.Loader))})
			}
			if cfg.Profile.MinecraftVersion != "" {
				facets = append(facets, []string{fmt.Sprintf("versions:%s", cfg.Profile.MinecraftVersion)})
			}
		}

		limit := searchLimit
		if limit <= 0 {
			limit = 10
		}

		resp, err := client.SearchWithOptions(query, facets, "relevance", 0, limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
			os.Exit(1)
		}

		if len(resp.Hits) == 0 {
			fmt.Println("No mods found matching your query.")
			return
		}

		fmt.Printf("Found %d results for '%s':\n\n", resp.TotalHits, query)
		for i, hit := range resp.Hits {
			fmt.Printf("%2d. %s (%s)\n", i+1, hit.Title, hit.Slug)
			if hit.Description != "" {
				fmt.Printf("    %s\n", hit.Description)
			}
			fmt.Printf("    Downloads: %d | Client: %s | Server: %s\n\n", hit.Downloads, hit.ClientSide, hit.ServerSide)
		}
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 10, "Maximum number of results to return")
}
