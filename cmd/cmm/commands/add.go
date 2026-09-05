package commands

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"cmm/internal/config"
	"cmm/internal/mod"
	"cmm/internal/modrinth"
	"cmm/internal/tui/tea"
	"github.com/spf13/cobra"
)

var (
	addVersion string
	addChannel string
	addYes     bool
)

func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	if devNull, err := os.Open(os.DevNull); err == nil {
		defer devNull.Close()
		if fStat, err1 := f.Stat(); err1 == nil {
			if nullStat, err2 := devNull.Stat(); err2 == nil && os.SameFile(fStat, nullStat) {
				return false
			}
		}
	}
	if stat, err := f.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		return false
	}
	return tea.IsTerminal(f)
}

var addCmd = &cobra.Command{
	Use:     "add <slug...>",
	Aliases: []string{"install"},
	Short:   "Install mod(s) from Modrinth with interactive version selection or batch mode",
	Args:    cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		channel := strings.ToLower(strings.TrimSpace(addChannel))
		if channel == "" {
			channel = "release"
		}
		if channel != "release" && channel != "beta" && channel != "alpha" {
			fmt.Fprintf(os.Stderr, "Error: invalid channel '%s'. Choose from: release, beta, alpha\n", addChannel)
			os.Exit(1)
		}

		userAgent := "CloudModManager/1.0 (contact: user@domain.local)"
		client, err := modrinth.NewClient(userAgent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating Modrinth client: %v\n", err)
			os.Exit(1)
		}

		mgr := mod.NewManager(client, "cmm.toml", "cmm.lock")
		reader := bufio.NewReader(os.Stdin)

		isNonTerminal := !isTerminal(os.Stdin)

		askConfirm := func(prompt string, defTrue bool) bool {
			if addYes {
				return true
			}
			fmt.Print(prompt)
			input, err := reader.ReadString('\n')
			if err != nil && input == "" {
				if isNonTerminal {
					return defTrue
				}
				return false
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input == "" {
				return defTrue
			}
			return input == "y" || input == "yes"
		}

		lock, _ := config.LoadLockfile("cmm.lock")

		// --------------------------------------------------------------------
		// Multi-mod batch installation (len(args) > 1)
		// --------------------------------------------------------------------
		if len(args) > 1 {
			type batchPlan struct {
				slug      string
				projTitle string
				verNumber string
				fileName  string
				isReplace bool
				curVer    string
			}
			var plans []batchPlan

			fmt.Printf("🔍 Resolving %d mod(s) [Channel: %s]...\n", len(args), channel)
			for _, slug := range args {
				versions, proj, err := mgr.GetCompatibleVersions(slug, channel)
				if err != nil || len(versions) == 0 {
					fmt.Fprintf(os.Stderr, "⚠️  Could not find compatible %s versions for '%s': %v\n", channel, slug, err)
					continue
				}
				latest := versions[0]
				fileName := ""
				if len(latest.Files) > 0 {
					fileName = latest.Files[0].Filename
				}
				title := slug
				if proj != nil && proj.Title != "" {
					title = proj.Title
				}

				existing := lock.GetMod(slug)
				isReplace := false
				curVer := ""
				if existing != nil {
					isReplace = true
					curVer = existing.GetVersion()
				}

				plans = append(plans, batchPlan{
					slug:      slug,
					projTitle: title,
					verNumber: latest.VersionNumber,
					fileName:  fileName,
					isReplace: isReplace,
					curVer:    curVer,
				})
			}

			if len(plans) == 0 {
				fmt.Println("No mods to install.")
				return
			}

			fmt.Printf("\n📦 Preparing to install %d mod(s) [Channel: %s]:\n", len(plans), channel)
			for _, p := range plans {
				if p.isReplace {
					fmt.Printf("- %s (%s) -> %s (%s) [REPLACING existing %s]\n", p.projTitle, p.slug, p.verNumber, p.fileName, p.curVer)
				} else {
					fmt.Printf("- %s (%s) -> %s (%s)\n", p.projTitle, p.slug, p.verNumber, p.fileName)
				}
			}
			fmt.Println()

			if !askConfirm("Proceed with installation? [Y/n]: ", true) {
				fmt.Println("Installation cancelled.")
				return
			}

			for _, p := range plans {
				res, err := mgr.AddWithChannelAndReplace(p.slug, p.verNumber, channel, p.isReplace)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error installing '%s': %v\n", p.slug, err)
					continue
				}
				if res.Replaced {
					fmt.Printf("🔄 Successfully upgraded/replaced %s (%s)\n", p.projTitle, p.verNumber)
				} else if res.InstalledMod != nil {
					fmt.Printf("✅ Successfully installed %s (%s)\n", res.InstalledMod.Name, res.InstalledMod.GetVersion())
				}
			}
			return
		}

		// --------------------------------------------------------------------
		// Single Mod Installation (len(args) == 1)
		// --------------------------------------------------------------------
		slug := args[0]

		// 1. If specific version is requested via -v flag
		if addVersion != "" {
			versions, proj, err := mgr.GetCompatibleVersions(slug, channel)
			targetTitle := slug
			if proj != nil && proj.Title != "" {
				targetTitle = proj.Title
			}
			targetFile := ""
			if err == nil {
				for _, v := range versions {
					if strings.EqualFold(v.VersionNumber, addVersion) ||
						strings.EqualFold(v.ID, addVersion) ||
						strings.EqualFold(strings.TrimPrefix(v.VersionNumber, "v"), strings.TrimPrefix(addVersion, "v")) {
						if len(v.Files) > 0 {
							targetFile = v.Files[0].Filename
						}
						break
					}
				}
			}

			existing := lock.GetMod(slug)
			isReplace := false
			if existing != nil {
				isReplace = true
				prompt := fmt.Sprintf("⚠️  Mod '%s' is already installed (Current: %s, Target: %s).\nReplace installed version with %s? [y/N]: ",
					targetTitle, existing.GetVersion(), addVersion, addVersion)
				if !askConfirm(prompt, false) {
					fmt.Println("Installation cancelled.")
					return
				}
			} else {
				fileMsg := ""
				if targetFile != "" {
					fileMsg = fmt.Sprintf(" (File: %s)", targetFile)
				}
				prompt := fmt.Sprintf("Target: %s (%s) version %s%s\nInstall this version? [Y/n]: ",
					targetTitle, slug, addVersion, fileMsg)
				if !askConfirm(prompt, true) {
					fmt.Println("Installation cancelled.")
					return
				}
			}

			res, err := mgr.AddWithChannelAndReplace(slug, addVersion, channel, isReplace)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error adding mod '%s': %v\n", slug, err)
				os.Exit(1)
			}
			if res.Replaced {
				fmt.Printf("🔄 Successfully replaced %s with version %s\n", targetTitle, addVersion)
			} else if res.InstalledMod != nil {
				fmt.Printf("✅ Successfully installed %s (%s)\n", res.InstalledMod.Name, res.InstalledMod.GetVersion())
			}
			return
		}

		// 2. Interactive pagination if version is not specified
		versions, proj, err := mgr.GetCompatibleVersions(slug, channel)
		if err != nil || len(versions) == 0 {
			fmt.Fprintf(os.Stderr, "Error: No compatible %s versions found for '%s'. Try --channel beta or --channel alpha.\n", channel, slug)
			os.Exit(1)
		}

		targetTitle := slug
		if proj != nil && proj.Title != "" {
			targetTitle = proj.Title
		}

		const pageSize = 10
		page := 0
		totalVersions := len(versions)
		totalPages := (totalVersions + pageSize - 1) / pageSize

		var selectedVersion *modrinth.Version

		for {
			start := page * pageSize
			end := start + pageSize
			if end > totalVersions {
				end = totalVersions
			}

			fmt.Printf("\n🔍 Available versions for %s (%s) [Channel: %s, Total: %d]:\n\n",
				targetTitle, slug, channel, totalVersions)

			for i := start; i < end; i++ {
				v := versions[i]
				fn := ""
				if len(v.Files) > 0 {
					fn = v.Files[0].Filename
				}
				published := ""
				if !v.DatePublished.IsZero() {
					published = fmt.Sprintf(" (Published: %s)", v.DatePublished.Format("2006-01-02"))
				}
				fmt.Printf("  [%d] %-12s - %s%s\n", (i - start + 1), v.VersionNumber, fn, published)
			}

			fmt.Printf("\nPage %d/%d (Showing %d-%d of %d). [n] Next page, [p] Prev page, [q] Cancel\n",
				page+1, totalPages, start+1, end, totalVersions)
			fmt.Printf("Select version [1-%d]: ", (end - start))

			if addYes {
				selectedVersion = &versions[0]
				break
			}

			input, err := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))

			if err != nil && input == "" {
				if isNonTerminal {
					selectedVersion = &versions[0]
					break
				}
				fmt.Println("\nInstallation cancelled.")
				return
			}

			if input == "q" || input == "quit" || input == "cancel" {
				fmt.Println("Installation cancelled.")
				return
			}
			if input == "n" || input == "next" {
				if page+1 < totalPages {
					page++
				} else {
					fmt.Println("Already on the last page.")
				}
				continue
			}
			if input == "p" || input == "prev" {
				if page > 0 {
					page--
				} else {
					fmt.Println("Already on the first page.")
				}
				continue
			}

			choice, err := strconv.Atoi(input)
			if err == nil && choice >= 1 && choice <= (end-start) {
				selectedVersion = &versions[start+choice-1]
				break
			}
			fmt.Println("Invalid selection, please try again.")
		}

		if selectedVersion == nil {
			fmt.Println("Installation cancelled.")
			return
		}

		targetFileName := ""
		if len(selectedVersion.Files) > 0 {
			targetFileName = selectedVersion.Files[0].Filename
		}

		existing := lock.GetMod(slug)
		isReplace := false
		if existing != nil {
			isReplace = true
			prompt := fmt.Sprintf("⚠️  Mod '%s' is already installed (Current: %s, Target: %s).\nReplace installed version with %s (%s)? [y/N]: ",
				targetTitle, existing.GetVersion(), selectedVersion.VersionNumber, selectedVersion.VersionNumber, targetFileName)
			if !askConfirm(prompt, false) {
				fmt.Println("Installation cancelled.")
				return
			}
		} else {
			prompt := fmt.Sprintf("Download and install %s (%s)? [Y/n]: ", selectedVersion.VersionNumber, targetFileName)
			if !askConfirm(prompt, true) {
				fmt.Println("Installation cancelled.")
				return
			}
		}

		res, err := mgr.AddWithChannelAndReplace(slug, selectedVersion.VersionNumber, channel, isReplace)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error installing mod '%s': %v\n", slug, err)
			os.Exit(1)
		}

		if res.Replaced {
			fmt.Printf("🔄 Successfully replaced %s with version %s (%s)\n", targetTitle, selectedVersion.VersionNumber, targetFileName)
		} else if res.InstalledMod != nil {
			fmt.Printf("✅ Successfully installed %s (%s)\n", res.InstalledMod.Name, res.InstalledMod.GetVersion())
		}
		for _, dep := range res.InstalledDeps {
			fmt.Printf("  -> Installed dependency: %s (%s)\n", dep.Name, dep.GetVersion())
		}
		for _, opt := range res.OptionalDeps {
			fmt.Printf("Notice: Optional dependency available: %s\n", opt)
		}
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().StringVarP(&addVersion, "version", "v", "", "Specific version to install")
	addCmd.Flags().StringVarP(&addChannel, "channel", "c", "release", "Stability channel filter (release, beta, alpha)")
	addCmd.Flags().BoolVarP(&addYes, "yes", "y", false, "Automatically confirm prompts")
}
