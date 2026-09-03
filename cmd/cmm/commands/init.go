package commands

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"cmm/internal/config"
	"cmm/internal/modrinth"
	"cmm/internal/sync"
	"github.com/spf13/cobra"
)

var (
	initName      string
	initMCVersion string
	initLoader    string
	initSide      string
	initModsDir   string
	initConfigDir string
)

func promptString(reader *bufio.Reader, msg, current, defVal string, validate func(string) error) string {
	if current != "" {
		if err := validate(current); err == nil {
			return current
		}
		fmt.Printf("Invalid value '%s': %v\n", current, validate(current))
		current = ""
	}

	for {
		if defVal != "" {
			fmt.Printf("%s[%s]: ", msg, defVal)
		} else {
			fmt.Print(msg)
		}

		val, err := reader.ReadString('\n')
		val = strings.TrimSpace(val)
		if err != nil && val == "" {
			if defVal != "" {
				return defVal
			}
			fmt.Printf("Error reading input: %v\n", err)
			os.Exit(1)
		}

		if val == "" && defVal != "" {
			val = defVal
		}

		if err := validate(val); err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		return val
	}
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize cmm.toml config",
	Run: func(cmd *cobra.Command, args []string) {
		configPath := "cmm.toml"

		if _, err := os.Stat(configPath); err == nil {
			fmt.Fprintf(os.Stderr, "Error: %s already exists\n", configPath)
			os.Exit(1)
		}

		if initMCVersion != "" && !strings.ContainsAny(initMCVersion, "0123456789") {
			fmt.Fprintf(os.Stderr, "Validation error: mc-version must contain a digit\n")
			os.Exit(1)
		}

		reader := bufio.NewReader(os.Stdin)

		initName = promptString(reader, "Server name", initName, "minecraft-server", func(s string) error {
			if s == "" {
				return fmt.Errorf("name cannot be empty")
			}
			return nil
		})

		mcVerRegex := regexp.MustCompile(`^(\d+(\.\d+)+([a-zA-Z0-9_\-\.]+)?|\d+w\d+[a-z]?)$`)
		initMCVersion = promptString(reader, "Minecraft version (e.g., 1.21.1, 26.2)", initMCVersion, "1.21.1", func(s string) error {
			if !mcVerRegex.MatchString(s) {
				return fmt.Errorf("must be a valid version format (e.g. 1.20, 1.21.1, 26.2, 24w14a)")
			}
			return nil
		})

		initLoader = promptString(reader, "Loader (fabric/forge/neoforge/quilt)", initLoader, "fabric", func(s string) error {
			s = strings.ToLower(s)
			if s != "fabric" && s != "forge" && s != "neoforge" && s != "quilt" {
				return fmt.Errorf("must be fabric, forge, neoforge, or quilt")
			}
			return nil
		})

		initSide = promptString(reader, "Side (server/client/both)", initSide, "server", func(s string) error {
			s = strings.ToLower(s)
			if s != "server" && s != "client" && s != "both" {
				return fmt.Errorf("must be server, client, or both")
			}
			return nil
		})

		cfg := config.Config{
			Profile: config.Profile{
				Name:             initName,
				MinecraftVersion: initMCVersion,
				Loader:           strings.ToLower(initLoader),
				Side:             strings.ToLower(initSide),
			},
			Paths: config.Paths{
				ModsDir:   initModsDir,
				ConfigDir: initConfigDir,
			},
		}

		err := config.SaveConfig(configPath, &cfg)
		if err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created %s successfully.\n", configPath)

		// Check for existing mods directory to automatically scan
		modsPath := cfg.Paths.ModsDir
		if modsPath == "" {
			modsPath = "mods"
		}
		if entries, err := os.ReadDir(modsPath); err == nil && len(entries) > 0 {
			jarCount := 0
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".jar") {
					jarCount++
				}
			}
			if jarCount > 0 {
				fmt.Printf("Detected %d existing JAR file(s) in '%s'. Scanning and generating cmm.lock...\n", jarCount, modsPath)
				client, err := modrinth.NewClient("CloudModManager/1.0 (contact: admin@localhost)")
				if err == nil {
					syncEngine := sync.NewLocalSynchronizer(client, configPath, "cmm.lock")
					res, err := syncEngine.Sync(sync.LocalSyncOptions{
						Path: modsPath,
					})
					if err != nil {
						fmt.Printf("Warning: Automatic scan encountered an issue: %v\n", err)
					} else {
						fmt.Printf("Successfully scanned and matched %d mod(s) in cmm.lock.\n", len(res.AddedMods))
					}
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initName, "name", "", "Server name")
	initCmd.Flags().StringVar(&initMCVersion, "mc-version", "", "Minecraft version")
	initCmd.Flags().StringVar(&initLoader, "loader", "", "Mod loader")
	initCmd.Flags().StringVar(&initSide, "side", "", "Side (server/client/both)")
	initCmd.Flags().StringVar(&initModsDir, "mods-dir", "", "Directory where mods are stored")
	initCmd.Flags().StringVar(&initConfigDir, "config-dir", "", "Directory to save cmm.toml")
}
