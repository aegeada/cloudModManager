package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"cmm/internal/launcher"
	"cmm/internal/modrinth"
	"github.com/spf13/cobra"
)

var (
	launcherListFormat string
	launcherListFilter string

	launcherSyncFilter  string
	launcherSyncFormat  string
	launcherSyncConfig  string
	launcherSyncLock    string
	launcherSyncForce   bool
	launcherSyncDryRun  bool
)

var launcherCmd = &cobra.Command{
	Use:   "launcher",
	Short: "Manage and sync Minecraft client launcher instances",
	Long:  `Auto-detect local Minecraft launcher instances (Vanilla, Prism, Modrinth App, CurseForge) and sync modpacks directly into them.`,
}

var launcherListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List detected local Minecraft launcher instances",
	Run: func(cmd *cobra.Command, args []string) {
		detector := launcher.NewDetector(launcher.DetectorOptions{})
		var instances []launcher.Instance
		var err error

		filter := launcher.NormalizeLauncherType(launcherListFilter)
		if filter == launcher.LauncherAll || filter == "" {
			instances, err = detector.DetectAll()
		} else {
			instances, err = detector.DetectLauncher(filter)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error detecting launcher instances: %v\n", err)
			os.Exit(1)
		}

		if strings.ToLower(launcherListFormat) == "json" {
			if instances == nil {
				instances = []launcher.Instance{}
			}
			data, err := json.MarshalIndent(instances, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
			return
		}

		if len(instances) == 0 {
			fmt.Println("No Minecraft launcher instances detected.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "LAUNCHER\tNAME\tMC VERSION\tLOADER\tLOADER VER\tMODS DIRECTORY")
		for _, inst := range instances {
			loaderVer := inst.LoaderVersion
			if loaderVer == "" {
				loaderVer = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				inst.LauncherName, inst.Name, inst.MinecraftVersion, inst.Loader, loaderVer, inst.ModsDir)
		}
		w.Flush()
	},
}

var launcherSyncCmd = &cobra.Command{
	Use:   "sync <instance-name>",
	Short: "Synchronize local modpack directly into a launcher instance directory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		instanceQuery := args[0]
		userAgent := "CloudModManager/1.0 (contact: user@domain.local)"
		client, err := modrinth.NewClient(userAgent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing Modrinth client: %v\n", err)
			os.Exit(1)
		}

		syncer := launcher.NewSyncer(client, launcher.DetectorOptions{})
		res, err := syncer.SyncInstance(launcher.LauncherSyncOptions{
			InstanceName: instanceQuery,
			LauncherType: launcher.NormalizeLauncherType(launcherSyncFilter),
			ConfigPath:   launcherSyncConfig,
			LockPath:     launcherSyncLock,
			Force:        launcherSyncForce,
			DryRun:       launcherSyncDryRun,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error syncing launcher instance: %v\n", err)
			os.Exit(1)
		}

		if strings.ToLower(launcherSyncFormat) == "json" {
			data, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
			return
		}

		if launcherSyncDryRun {
			fmt.Printf("🔍 [DRY RUN] Direct sync preview for %s instance '%s':\n", res.Instance.LauncherName, res.Instance.Name)
			fmt.Printf("Target directory: %s\n", res.Instance.ModsDir)
			if res.UpToDate {
				fmt.Println("Instance is already up to date.")
			} else {
				if len(res.AddedMods) > 0 {
					fmt.Printf("Will Add (%d): %s\n", len(res.AddedMods), strings.Join(res.AddedMods, ", "))
				}
				if len(res.UpdatedMods) > 0 {
					fmt.Printf("Will Update (%d): %s\n", len(res.UpdatedMods), strings.Join(res.UpdatedMods, ", "))
				}
				if len(res.RemovedMods) > 0 {
					fmt.Printf("Will Remove (%d): %s\n", len(res.RemovedMods), strings.Join(res.RemovedMods, ", "))
				}
			}
			if len(res.SkippedMods) > 0 {
				fmt.Printf("Skipped server-only mods (%d): %s\n", len(res.SkippedMods), strings.Join(res.SkippedMods, ", "))
			}
			return
		}

		if res.UpToDate {
			fmt.Printf("Instance '%s' is already up to date.\n", res.Instance.Name)
			if len(res.SkippedMods) > 0 {
				fmt.Printf("Skipped server-only mods (%d): %s\n", len(res.SkippedMods), strings.Join(res.SkippedMods, ", "))
			}
			return
		}

		fmt.Printf("Successfully synchronized modpack into %s instance '%s'\n", res.Instance.LauncherName, res.Instance.Name)
		fmt.Printf("Target directory: %s\n", res.Instance.ModsDir)
		if len(res.AddedMods) > 0 {
			fmt.Printf("Added (%d): %s\n", len(res.AddedMods), strings.Join(res.AddedMods, ", "))
		}
		if len(res.UpdatedMods) > 0 {
			fmt.Printf("Updated (%d): %s\n", len(res.UpdatedMods), strings.Join(res.UpdatedMods, ", "))
		}
		if len(res.RemovedMods) > 0 {
			fmt.Printf("Removed (%d): %s\n", len(res.RemovedMods), strings.Join(res.RemovedMods, ", "))
		}
		if len(res.SkippedMods) > 0 {
			fmt.Printf("Skipped server-only mods (%d): %s\n", len(res.SkippedMods), strings.Join(res.SkippedMods, ", "))
		}
	},
}

func init() {
	rootCmd.AddCommand(launcherCmd)
	launcherCmd.AddCommand(launcherListCmd)
	launcherCmd.AddCommand(launcherSyncCmd)

	launcherListCmd.Flags().StringVarP(&launcherListFormat, "format", "f", "table", "Output format (table or json)")
	launcherListCmd.Flags().StringVarP(&launcherListFilter, "launcher", "l", "all", "Filter by launcher type (vanilla, prism, modrinth, curseforge, all)")

	launcherSyncCmd.Flags().StringVarP(&launcherSyncFilter, "launcher", "l", "", "Specify launcher type filter")
	launcherSyncCmd.Flags().StringVarP(&launcherSyncConfig, "config", "c", "cmm.toml", "Path to cmm.toml")
	launcherSyncCmd.Flags().StringVarP(&launcherSyncLock, "lock", "k", "cmm.lock", "Path to cmm.lock")
	launcherSyncCmd.Flags().BoolVar(&launcherSyncForce, "force", false, "Force sync ignoring version/loader mismatch warnings")
	launcherSyncCmd.Flags().BoolVar(&launcherSyncDryRun, "dry-run", false, "Simulate sync operations without modifying files on disk")
	launcherSyncCmd.Flags().StringVarP(&launcherSyncFormat, "format", "f", "text", "Output format (text or json)")
}
