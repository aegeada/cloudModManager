package export

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cmm/internal/config"
	"cmm/internal/mod"
)

type MrpackIndex struct {
	FormatVersion int               `json:"formatVersion"`
	Game          string            `json:"game"`
	VersionID     string            `json:"versionId"`
	Name          string            `json:"name"`
	Summary       string            `json:"summary,omitempty"`
	Files         []MrpackFile      `json:"files"`
	Dependencies  map[string]string `json:"dependencies"`
}

type MrpackFile struct {
	Path      string            `json:"path"`
	Hashes    map[string]string `json:"hashes"`
	Env       MrpackEnv         `json:"env"`
	Downloads []string          `json:"downloads"`
	FileSize  int64             `json:"fileSize,omitempty"`
}

type MrpackEnv struct {
	Client string `json:"client"`
	Server string `json:"server"`
}

type MrpackExportOptions struct {
	OutputPath string
	Name       string
	VersionID  string
	ConfigPath string
	LockPath   string
	ModsDir    string
	ConfigDir  string
}

func mapSideToEnv(side string) MrpackEnv {
	switch strings.ToLower(strings.TrimSpace(side)) {
	case "client":
		return MrpackEnv{Client: "required", Server: "unsupported"}
	case "server":
		return MrpackEnv{Client: "unsupported", Server: "required"}
	case "optional":
		return MrpackEnv{Client: "optional", Server: "optional"}
	default:
		return MrpackEnv{Client: "required", Server: "required"}
	}
}

// ExportMrpack builds a standard .mrpack ZIP archive from the active configuration and lockfile.
func ExportMrpack(opts MrpackExportOptions) error {
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = "cmm.toml"
	}
	lockPath := opts.LockPath
	if lockPath == "" {
		lockPath = "cmm.lock"
	}
	modsDir := opts.ModsDir
	if modsDir == "" {
		modsDir = "mods"
	}
	configDir := opts.ConfigDir
	if configDir == "" {
		configDir = "config"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	lock, err := config.LoadLockfile(lockPath)
	if err != nil || lock == nil {
		lock = &config.Lockfile{Mods: []config.LockfileMod{}}
	}

	name := opts.Name
	if name == "" {
		if cfg.Profile.Name != "" {
			name = cfg.Profile.Name
		} else {
			name = "modpack"
		}
	}

	versionID := opts.VersionID
	if versionID == "" {
		versionID = "1.0.0"
	}

	outputPath := opts.OutputPath
	if outputPath == "" {
		outputPath = fmt.Sprintf("%s.mrpack", name)
	}

	// Dependencies
	mcVersion := cfg.Profile.MinecraftVersion
	if mcVersion == "" {
		mcVersion = "1.21.1"
	}
	loader := cfg.Profile.Loader
	if loader == "" {
		loader = "fabric"
	}
	loaderVersion := cfg.Profile.LoaderVersion
	if loaderVersion == "" {
		loaderVersion = "0.19.3"
	}

	deps := map[string]string{
		"minecraft":                mcVersion,
		loader + "-loader":         loaderVersion,
		loader:                     loaderVersion,
	}

	files := make([]MrpackFile, 0, len(lock.Mods))
	for _, m := range lock.Mods {
		fileName := m.FileName
		if fileName == "" {
			if m.Slug != "" {
				fileName = m.Slug + ".jar"
			} else if m.Name != "" {
				fileName = m.Name + ".jar"
			} else {
				fileName = "mod.jar"
			}
		}

		relPath := fmt.Sprintf("mods/%s", fileName)
		hashes := make(map[string]string)
		if m.SHA512 != "" {
			hashes["sha512"] = m.SHA512
		}

		var fileSize int64
		localPath := filepath.Join(modsDir, fileName)
		if fi, err := os.Stat(localPath); err == nil {
			fileSize = fi.Size()
			if h512, err := mod.ComputeSHA512(localPath); err == nil && h512 != "" {
				hashes["sha512"] = h512
			}
			if h1, err := mod.ComputeSHA1(localPath); err == nil && h1 != "" {
				hashes["sha1"] = h1
			}
		}

		downloads := []string{}
		if m.DownloadURL != "" {
			downloads = append(downloads, m.DownloadURL)
		}

		files = append(files, MrpackFile{
			Path:      relPath,
			Hashes:    hashes,
			Env:       mapSideToEnv(m.Side),
			Downloads: downloads,
			FileSize:  fileSize,
		})
	}

	index := MrpackIndex{
		FormatVersion: 1,
		Game:          "minecraft",
		VersionID:     versionID,
		Name:          name,
		Summary:       fmt.Sprintf("Exported modpack for %s", name),
		Files:         files,
		Dependencies:  deps,
	}

	// Create parent directory for output if needed
	outDir := filepath.Dir(outputPath)
	if outDir != "" && outDir != "." {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	zipFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file '%s': %w", outputPath, err)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	// Write modrinth.index.json
	indexJSON, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index JSON: %w", err)
	}

	iw, err := zw.Create("modrinth.index.json")
	if err != nil {
		return fmt.Errorf("failed to create index in zip: %w", err)
	}
	if _, err := iw.Write(indexJSON); err != nil {
		return fmt.Errorf("failed to write index in zip: %w", err)
	}

	// Pack config/ directory into overrides/config/
	if _, err := os.Stat(configDir); err == nil {
		_ = filepath.Walk(configDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(configDir, path)
			if err != nil {
				return nil
			}
			zipEntryPath := filepath.Join("overrides", "config", rel)
			w, err := zw.Create(zipEntryPath)
			if err != nil {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			_, _ = io.Copy(w, f)
			return nil
		})
	}

	// Pack overrides/ directory if present
	if _, err := os.Stat("overrides"); err == nil {
		_ = filepath.Walk("overrides", func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel("overrides", path)
			if err != nil {
				return nil
			}
			zipEntryPath := filepath.Join("overrides", rel)
			w, err := zw.Create(zipEntryPath)
			if err != nil {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			_, _ = io.Copy(w, f)
			return nil
		})
	}

	return nil
}
