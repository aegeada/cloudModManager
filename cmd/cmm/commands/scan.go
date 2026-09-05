package commands

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cmm/internal/config"
	"cmm/internal/loader"
	"cmm/internal/modrinth"
	"cmm/internal/sync"
	"github.com/spf13/cobra"
)

var (
	scanPath string
	scanYes  bool
)

func detectExactMCVersion(baseDir string) (string, bool) {
	// 1. Check server.jar / root jars for version.json
	entries, _ := os.ReadDir(baseDir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".jar") {
			jarPath := filepath.Join(baseDir, e.Name())
			if zr, err := zip.OpenReader(jarPath); err == nil {
				var foundMC string
				for _, f := range zr.File {
					if f.Name == "version.json" {
						if rc, err := f.Open(); err == nil {
							var vj struct {
								ID   string `json:"id"`
								Name string `json:"name"`
							}
							if err := json.NewDecoder(rc).Decode(&vj); err == nil {
								if vj.ID != "" {
									foundMC = vj.ID
								} else if vj.Name != "" {
									foundMC = vj.Name
								}
							}
							rc.Close()
							if foundMC != "" {
								break
							}
						}
					}
				}
				zr.Close()
				if foundMC != "" {
					return foundMC, true
				}
			}
		}
	}

	// 2. Check client instance manifests
	for _, mf := range []string{"instance.json", "mmc-pack.json", "modrinth.index.json"} {
		mfPath := filepath.Join(baseDir, mf)
		if data, err := os.ReadFile(mfPath); err == nil {
			var parsed map[string]interface{}
			if err := json.Unmarshal(data, &parsed); err == nil {
				if deps, ok := parsed["dependencies"].(map[string]interface{}); ok {
					if mc, ok := deps["minecraft"].(string); ok && mc != "" {
						return mc, true
					}
				}
				if comps, ok := parsed["components"].([]interface{}); ok {
					for _, c := range comps {
						if cm, ok := c.(map[string]interface{}); ok {
							if cm["uid"] == "net.minecraft" {
								if cv, ok := cm["cachedVersion"].(string); ok && cv != "" {
									return cv, true
								}
								if cv, ok := cm["version"].(string); ok && cv != "" {
									return cv, true
								}
							}
						}
					}
				}
			}
		}
	}

	return "", false
}

func detectExactLoader(baseDir string) (loaderType string, loaderVersion string, exact bool) {
	mfPath := filepath.Join(baseDir, "modrinth.index.json")
	if data, err := os.ReadFile(mfPath); err == nil {
		var parsed map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err == nil {
			if deps, ok := parsed["dependencies"].(map[string]interface{}); ok {
				for k, v := range deps {
					verStr, _ := v.(string)
					if strings.Contains(k, "fabric") {
						return "fabric", verStr, true
					}
					if strings.Contains(k, "neoforge") {
						return "neoforge", verStr, true
					}
					if strings.Contains(k, "forge") {
						return "forge", verStr, true
					}
					if strings.Contains(k, "quilt") {
						return "quilt", verStr, true
					}
				}
			}
		}
	}

	entries, _ := os.ReadDir(baseDir)
	fabricLoaderJarRegex := regexp.MustCompile(`(?i)fabric-loader-([0-9.]+)-([0-9.]+)`)
	for _, e := range entries {
		m := fabricLoaderJarRegex.FindStringSubmatch(e.Name())
		if len(m) > 1 {
			return "fabric", m[1], true
		}
	}

	return "", "", false
}

func detectEnvironment(baseDir string) (loader, mcVersion string) {
	loader = "fabric"
	mcVersion = "1.21.1"

	// 1. Inspect logs/latest.log if available
	logPath := filepath.Join(baseDir, "logs", "latest.log")
	if data, err := os.ReadFile(logPath); err == nil {
		logContent := string(data)
		if len(logContent) > 4096 {
			logContent = logContent[:4096]
		}
		logVerRegex := regexp.MustCompile(`(?i)(?:Loading Minecraft|minecraft server version|Minecraft Version:?)\s+([0-9]+(\.[0-9]+)+|[0-9]{2}w[0-9]{2}[a-z])`)
		if m := logVerRegex.FindStringSubmatch(logContent); len(m) > 1 {
			mcVersion = m[1]
		}
		logLoaderRegex := regexp.MustCompile(`(?i)\b(fabric|neoforge|forge|quilt)\b`)
		if m := logLoaderRegex.FindStringSubmatch(logContent); len(m) > 1 {
			loader = strings.ToLower(m[1])
		}
	}

	// 2. Inspect root directory files & server jars
	entries, _ := os.ReadDir(baseDir)
	serverJarRegex := regexp.MustCompile(`(?i)(?:minecraft_server|fabric-server-mc|forge|neoforge|paper|purpur)[-_.]?([0-9]+(\.[0-9]+)+)`)
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if strings.Contains(name, "fabric") {
			loader = "fabric"
		} else if strings.Contains(name, "neoforge") {
			loader = "neoforge"
		} else if strings.Contains(name, "forge") {
			loader = "forge"
		} else if strings.Contains(name, "quilt") {
			loader = "quilt"
		}

		if m := serverJarRegex.FindStringSubmatch(name); len(m) > 1 {
			mcVersion = m[1]
		}
	}

	// 3. Frequency voting across mods/ directory (including .jar and .jar.disabled)
	modsDir := filepath.Join(baseDir, "mods")
	if modEntries, err := os.ReadDir(modsDir); err == nil && len(modEntries) > 0 {
		verVotes := make(map[string]int)
		loaderVotes := make(map[string]int)

		mcTokenRegex := regexp.MustCompile(`(?i)(?:[+_.-](?:mc)?(1\.(?:1[4-9]|2[0-9])(?:\.[0-9]+)?|26\.[0-9]+|[0-9]{2}w[0-9]{2}[a-z]))`)

		for _, me := range modEntries {
			mName := strings.ToLower(me.Name())
			if !strings.HasSuffix(mName, ".jar") && !strings.HasSuffix(mName, ".disabled") {
				continue
			}

			if strings.Contains(mName, "neoforge") {
				loaderVotes["neoforge"]++
			} else if strings.Contains(mName, "fabric") {
				loaderVotes["fabric"]++
			} else if strings.Contains(mName, "forge") {
				loaderVotes["forge"]++
			} else if strings.Contains(mName, "quilt") {
				loaderVotes["quilt"]++
			}

			matches := mcTokenRegex.FindAllStringSubmatch(mName, -1)
			for _, match := range matches {
				if len(match) > 1 {
					cleanVer := strings.Trim(match[1], "+-_.")
					cleanVer = strings.TrimPrefix(cleanVer, "mc")
					verVotes[cleanVer]++
				}
			}
		}

		bestVer := ""
		maxVerVotes := 0
		for v, count := range verVotes {
			if count > maxVerVotes {
				maxVerVotes = count
				bestVer = v
			}
		}
		if bestVer != "" {
			mcVersion = bestVer
		}

		bestLoader := ""
		maxLoaderVotes := 0
		for l, count := range loaderVotes {
			if count > maxLoaderVotes {
				maxLoaderVotes = count
				bestLoader = l
			}
		}
		if bestLoader != "" {
			loader = bestLoader
		}
	}

	return loader, mcVersion
}

var scanCmd = &cobra.Command{
	Use:   "scan [directory]",
	Short: "Scan server directory for mods, configs, and loader to auto-generate or update cmm files",
	Long: `Scan inspects the target directory for Minecraft server JARs, mod JARs in the mods/ folder, 
and configuration files. It queries Modrinth to identify installed mods and generates or updates 
cmm.toml and cmm.lock automatically.`,
	Run: func(cmd *cobra.Command, args []string) {
		targetDir := "."
		if len(args) > 0 {
			targetDir = args[0]
		} else if scanPath != "" {
			targetDir = scanPath
		}

		absDir, err := filepath.Abs(targetDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving directory '%s': %v\n", targetDir, err)
			os.Exit(1)
		}

		fmt.Printf("🔍 Scanning directory: %s\n", absDir)

		configPath := filepath.Join(absDir, "cmm.toml")
		lockPath := filepath.Join(absDir, "cmm.lock")
		modsDir := filepath.Join(absDir, "mods")
		reader := bufio.NewReader(os.Stdin)

		isNonTerminal := !isTerminal(os.Stdin)

		askConfirmPrompt := func(prompt string) bool {
			if scanYes || isNonTerminal {
				return true
			}
			fmt.Print(prompt)
			input, err := reader.ReadString('\n')
			if err != nil && input == "" {
				return true
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input == "" {
				return true
			}
			return input != "n" && input != "no"
		}

		// 1. Detect or load configuration
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			detLoader, detMC := detectEnvironment(absDir)
			finalMC := detMC
			finalLoader := detLoader
			fmt.Printf("Auto-detected Environment -> Loader: %s, Minecraft: %s\n", detLoader, detMC)

			// Step 1: Detect exact Minecraft version
			exactMC, isExactMC := detectExactMCVersion(absDir)
			if isExactMC {
				finalMC = exactMC
				fmt.Printf("🎯 Exact Minecraft version detected from server files: %s\n", finalMC)
			} else {
				prompt := fmt.Sprintf("Could not determine exact Minecraft version. Use detected version (%s)? [Y/n]: ", detMC)
				if !askConfirmPrompt(prompt) {
					fmt.Print("Enter Minecraft version: ")
					manualMC, err := reader.ReadString('\n')
					manualMC = strings.TrimSpace(manualMC)
					if err != nil && manualMC == "" {
						manualMC = detMC
					}
					if manualMC != "" {
						finalMC = manualMC
					}
				}
			}

			// Step 2: Detect exact Loader
			exactLdrType, exactLdrVer, isExactLdr := detectExactLoader(absDir)
			if isExactLdr {
				finalLoader = exactLdrType
				fmt.Printf("🎯 Exact Loader detected from server files: %s (v%s)\n", finalLoader, exactLdrVer)
			} else {
				prompt := fmt.Sprintf("Could not determine exact Loader version. Use detected version (%s)? [Y/n]: ", detLoader)
				if !askConfirmPrompt(prompt) {
					fmt.Print("Enter Loader (e.g. fabric, forge, neoforge, quilt): ")
					manualLdr, err := reader.ReadString('\n')
					manualLdr = strings.TrimSpace(strings.ToLower(manualLdr))
					if err != nil && manualLdr == "" {
						manualLdr = detLoader
					}
					if manualLdr != "" {
						finalLoader = manualLdr
					}
				}
			}

			cfg = &config.Config{
				Profile: config.Profile{
					Name:             filepath.Base(absDir),
					MinecraftVersion: finalMC,
					Loader:           finalLoader,
					LoaderVersion:    exactLdrVer,
					Side:             "server",
				},
				Paths: config.Paths{
					ModsDir:   "mods",
					ConfigDir: "config",
				},
			}
			if err := config.SaveConfig(configPath, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", configPath, err)
				os.Exit(1)
			}
			fmt.Printf("✨ Created %s with detected profile settings.\n", configPath)
		} else {
			fmt.Printf("ℹ️  Using existing %s (Profile: %s | MC: %s | Loader: %s)\n",
				configPath, cfg.Profile.Name, cfg.Profile.MinecraftVersion, cfg.Profile.Loader)
		}

		// 2. Perform Local Synchronizer Scan
		client, err := modrinth.NewClient("CloudModManager/1.0 (contact: admin@localhost)")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating Modrinth client: %v\n", err)
			os.Exit(1)
		}

		syncEngine := sync.NewLocalSynchronizer(client, configPath, lockPath)
		res, err := syncEngine.Sync(sync.LocalSyncOptions{
			Path: modsDir,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error during scan: %v\n", err)
			os.Exit(1)
		}

		// 3. Print Detailed Summary
		fmt.Println("\n📊 --- Scan Summary ---")
		fmt.Printf("• Recognized Mods Added/Updated (%d):\n", len(res.AddedMods))
		for _, m := range res.AddedMods {
			fmt.Printf("   ✓ %s\n", m)
		}

		if len(res.UnknownJars) > 0 {
			fmt.Printf("\n• Unrecognized / Custom JARs (%d):\n", len(res.UnknownJars))
			for _, uj := range res.UnknownJars {
				fmt.Printf("   ⚠️  %s\n", uj)
			}
		}

		// 4. Check for config directory
		cfgDir := filepath.Join(absDir, "config")
		if stat, err := os.Stat(cfgDir); err == nil && stat.IsDir() {
			if cfgEntries, err := os.ReadDir(cfgDir); err == nil {
				fmt.Printf("\n• Config directory detected (%d files/folders in %s)\n", len(cfgEntries), cfgDir)
			}
		}

		fmt.Printf("\n✅ Scan complete. %s is up to date.\n", lockPath)

		// 5. Notice: Check if a newer Loader version is available
		if cfg != nil && cfg.Profile.Loader != "" {
			if latestVer, available, _ := loader.CheckLatestLoaderVersion(cfg.Profile.Loader, cfg.Profile.LoaderVersion); available {
				fmt.Printf("\nNotice: A new %s Loader version is available (v%s). Run: cmm loader update\n", strings.Title(cfg.Profile.Loader), latestVer)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().StringVarP(&scanPath, "path", "p", "", "Target directory path to scan")
	scanCmd.Flags().BoolVarP(&scanYes, "yes", "y", false, "Automatically accept default detection values")
}
