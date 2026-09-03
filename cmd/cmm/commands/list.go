package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"cmm/internal/config"
	"cmm/internal/mod"
	"cmm/internal/modrinth"
	"github.com/spf13/cobra"
)

var (
	listFormat string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed mods",
	Run: func(cmd *cobra.Command, args []string) {
		userAgent := "CloudModManager/1.0 (contact: user@domain.local)"
		client, _ := modrinth.NewClient(userAgent)

		mgr := mod.NewManager(client, "cmm.toml", "cmm.lock")

		statuses, err := mgr.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing mods: %v\n", err)
			os.Exit(1)
		}

		if strings.ToLower(listFormat) == "json" {
			if statuses == nil {
				statuses = []mod.ModStatus{}
			}
			data, err := json.MarshalIndent(statuses, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
			return
		}

		if len(statuses) == 0 {
			fmt.Println("No mods installed.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tSLUG\tVERSION\tSIDE\tSTATUS\tPINNED\tUPDATE AVAILABLE")
		for _, st := range statuses {
			statusStr := "enabled"
			if st.Disabled {
				statusStr = "disabled"
			}
			pinnedStr := "no"
			if st.Pinned {
				pinnedStr = "yes"
			}
			updateStr := "no"
			if st.UpdateAvailable {
				updateStr = fmt.Sprintf("yes (%s)", st.LatestVersion)
			}
			sideStr := config.NormalizeSide(st.Side)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", st.Name, st.Slug, st.Version, sideStr, statusStr, pinnedStr, updateStr)
		}
		w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&listFormat, "format", "f", "table", "Output format (table or json)")
}
