package commands

import (
	"fmt"
	"os"
	"strings"

	"cmm/internal/export"
	"github.com/spf13/cobra"
)

var (
	exportFormat    string
	exportOutput    string
	exportName      string
	exportVersionID string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export installed mods to mrpack or github format",
	Long:  `Packages current modpack configuration into Modrinth .mrpack or clean GitHub repository format.`,
	Run: func(cmd *cobra.Command, args []string) {
		format := strings.ToLower(strings.TrimSpace(exportFormat))
		if format == "" {
			format = "mrpack"
		}

		switch format {
		case "mrpack":
			opts := export.MrpackExportOptions{
				OutputPath: exportOutput,
				Name:       exportName,
				VersionID:  exportVersionID,
				ConfigPath: "cmm.toml",
				LockPath:   "cmm.lock",
				ModsDir:    "mods",
				ConfigDir:  "config",
			}
			if err := export.ExportMrpack(opts); err != nil {
				fmt.Fprintf(os.Stderr, "Error exporting mrpack: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Successfully exported Modrinth modpack (.mrpack).")

		case "github":
			opts := export.GitHubExportOptions{
				OutputDir:  exportOutput,
				ConfigPath: "cmm.toml",
				LockPath:   "cmm.lock",
			}
			if err := export.ExportGitHub(opts); err != nil {
				fmt.Fprintf(os.Stderr, "Error exporting GitHub repository: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Successfully exported GitHub repository files.")

		default:
			fmt.Fprintf(os.Stderr, "Invalid export format '%s'. Supported formats: mrpack, github\n", exportFormat)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "mrpack", "Export format (mrpack or github)")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file path (.mrpack) or directory (github)")
	exportCmd.Flags().StringVar(&exportName, "name", "", "Modpack name override")
	exportCmd.Flags().StringVar(&exportVersionID, "version-id", "", "Modpack version ID override")
}
