package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Profile struct {
	Name             string `toml:"name"`
	MinecraftVersion string `toml:"minecraft_version"`
	Loader           string `toml:"loader"`
	LoaderVersion    string `toml:"loader_version"`
	Side             string `toml:"side"`
}

type Modrinth struct {
	Token string `toml:"token"`
}

type SyncSource struct {
	Type  string `toml:"type"`
	URL   string `toml:"url,omitempty"`
	Token string `toml:"token,omitempty"`
	Slug  string `toml:"slug,omitempty"`
	Repo  string `toml:"repo,omitempty"`
	Path  string `toml:"path,omitempty"`
}

type Paths struct {
	MinecraftDir string `toml:"minecraft_dir,omitempty"`
	ModsDir      string `toml:"mods_dir,omitempty"`
	ConfigDir    string `toml:"config_dir,omitempty"`
}

type Config struct {
	Profile     Profile      `toml:"profile"`
	Paths       Paths        `toml:"paths,omitempty"`
	Modrinth    Modrinth     `toml:"modrinth"`
	SyncSources []SyncSource `toml:"sync_sources"`
	SyncToken   string       `toml:"sync_token,omitempty"`
}

type rawConfig struct {
	Name             string `toml:"name"`
	MinecraftVersion string `toml:"minecraft_version"`
	MCVersion        string `toml:"mc_version"`
	Loader           string `toml:"loader"`
	LoaderVersion    string `toml:"loader_version"`
	Side             string `toml:"side"`
	ModsDir          string `toml:"mods_dir"`
	ConfigDir        string `toml:"config_dir"`
	Server           *struct {
		Name             string `toml:"name"`
		MinecraftVersion string `toml:"minecraft_version"`
		MCVersion        string `toml:"mc_version"`
		Loader           string `toml:"loader"`
		LoaderVersion    string `toml:"loader_version"`
		Side             string `toml:"side"`
	} `toml:"server"`
	Profile     *Profile     `toml:"profile"`
	Paths       *Paths       `toml:"paths"`
	Modrinth    *Modrinth    `toml:"modrinth"`
	SyncSources []SyncSource `toml:"sync_sources"`
	SyncToken   string       `toml:"sync_token"`
}

// DefaultConfig returns a Config initialized with default profile and path settings.
func DefaultConfig() *Config {
	return &Config{
		Profile: Profile{
			Name:             "minecraft-server",
			MinecraftVersion: "1.21.1",
			Loader:           "fabric",
			Side:             "server",
		},
		Paths: Paths{
			ModsDir:   "mods",
			ConfigDir: "config",
		},
	}
}

// SaveConfig saves the config to a file.
func SaveConfig(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(cfg)
}

// LoadConfig loads the config from a file, handling top-level and legacy properties gracefully.
func LoadConfig(path string) (*Config, error) {
	var raw rawConfig
	_, err := toml.DecodeFile(path, &raw)
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()

	if raw.Profile != nil {
		if raw.Profile.Name != "" {
			cfg.Profile.Name = raw.Profile.Name
		}
		if raw.Profile.MinecraftVersion != "" {
			cfg.Profile.MinecraftVersion = raw.Profile.MinecraftVersion
		}
		if raw.Profile.Loader != "" {
			cfg.Profile.Loader = raw.Profile.Loader
		}
		if raw.Profile.LoaderVersion != "" {
			cfg.Profile.LoaderVersion = raw.Profile.LoaderVersion
		}
		if raw.Profile.Side != "" {
			cfg.Profile.Side = raw.Profile.Side
		}
	}

	if raw.Server != nil {
		if raw.Server.Name != "" {
			cfg.Profile.Name = raw.Server.Name
		}
		if raw.Server.MinecraftVersion != "" {
			cfg.Profile.MinecraftVersion = raw.Server.MinecraftVersion
		}
		if raw.Server.MCVersion != "" {
			cfg.Profile.MinecraftVersion = raw.Server.MCVersion
		}
		if raw.Server.Loader != "" {
			cfg.Profile.Loader = raw.Server.Loader
		}
		if raw.Server.LoaderVersion != "" {
			cfg.Profile.LoaderVersion = raw.Server.LoaderVersion
		}
		if raw.Server.Side != "" {
			cfg.Profile.Side = raw.Server.Side
		}
	}

	if raw.Name != "" {
		cfg.Profile.Name = raw.Name
	}
	if raw.MinecraftVersion != "" {
		cfg.Profile.MinecraftVersion = raw.MinecraftVersion
	}
	if raw.MCVersion != "" {
		cfg.Profile.MinecraftVersion = raw.MCVersion
	}
	if raw.Loader != "" {
		cfg.Profile.Loader = raw.Loader
	}
	if raw.LoaderVersion != "" {
		cfg.Profile.LoaderVersion = raw.LoaderVersion
	}
	if raw.Side != "" {
		cfg.Profile.Side = raw.Side
	}

	if raw.Paths != nil {
		if raw.Paths.MinecraftDir != "" {
			cfg.Paths.MinecraftDir = raw.Paths.MinecraftDir
		}
		if raw.Paths.ModsDir != "" {
			cfg.Paths.ModsDir = raw.Paths.ModsDir
		}
		if raw.Paths.ConfigDir != "" {
			cfg.Paths.ConfigDir = raw.Paths.ConfigDir
		}
	}
	if raw.ModsDir != "" {
		cfg.Paths.ModsDir = raw.ModsDir
	}
	if raw.ConfigDir != "" {
		cfg.Paths.ConfigDir = raw.ConfigDir
	}

	if raw.Modrinth != nil {
		cfg.Modrinth = *raw.Modrinth
	}
	if raw.SyncSources != nil {
		cfg.SyncSources = raw.SyncSources
	}
	if raw.SyncToken != "" {
		cfg.SyncToken = raw.SyncToken
	}

	return cfg, nil
}
