package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cmm/internal/config"
	"cmm/internal/modrinth"
)

type ModrinthSynchronizer struct {
	Client     *modrinth.Client
	ConfigPath string
	LockPath   string
}

func NewModrinthSynchronizer(client *modrinth.Client, configPath, lockPath string) *ModrinthSynchronizer {
	if configPath == "" {
		configPath = "cmm.toml"
	}
	if lockPath == "" {
		lockPath = "cmm.lock"
	}
	return &ModrinthSynchronizer{
		Client:     client,
		ConfigPath: configPath,
		LockPath:   lockPath,
	}
}

func (s *ModrinthSynchronizer) Sync(opts ModrinthSyncOptions) (*SyncResult, error) {
	if opts.Slug == "" && opts.FilePath == "" {
		return nil, fmt.Errorf("either --slug or --file is required for modrinth sync")
	}

	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	modsDir := cfg.Paths.ModsDir
	if modsDir == "" {
		modsDir = "mods"
	}

	mrpackPath := opts.FilePath

	// 1. Resolve remote slug if specified
	if opts.Slug != "" {
		if s.Client == nil {
			return nil, fmt.Errorf("Modrinth client is required for remote slug sync")
		}

		proj, err := s.Client.GetProject(opts.Slug)
		if err != nil || proj == nil {
			return nil, fmt.Errorf("modpack '%s' not found on Modrinth (404)", opts.Slug)
		}

		var loaders []string
		if cfg.Profile.Loader != "" {
			loaders = []string{cfg.Profile.Loader}
		}
		var gameVersions []string
		if cfg.Profile.MinecraftVersion != "" {
			gameVersions = []string{cfg.Profile.MinecraftVersion}
		}

		versions, err := s.Client.GetProjectVersions(opts.Slug, loaders, gameVersions, nil)
		if err != nil || len(versions) == 0 {
			// Fallback: try without loaders/gameVersions filters
			versions, err = s.Client.GetProjectVersions(opts.Slug, nil, nil, nil)
			if err != nil || len(versions) == 0 {
				return nil, fmt.Errorf("no compatible versions found for modpack '%s'", opts.Slug)
			}
		}

		latest := versions[0]
		var mrpackURL string
		var expectedSHA512 string
		for _, f := range latest.Files {
			if strings.HasSuffix(f.Filename, ".mrpack") || f.Primary {
				mrpackURL = f.URL
				expectedSHA512 = f.Hashes["sha512"]
				break
			}
		}

		if mrpackURL == "" && len(latest.Files) > 0 {
			mrpackURL = latest.Files[0].URL
			expectedSHA512 = latest.Files[0].Hashes["sha512"]
		}

		tmpFile, err := os.CreateTemp("", "pack-*.mrpack")
		if err != nil {
			return nil, fmt.Errorf("failed to create temporary mrpack file: %w", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		if err := s.Client.DownloadFile(mrpackURL, tmpFile.Name(), expectedSHA512); err != nil {
			return nil, fmt.Errorf("failed to download mrpack file: %w", err)
		}
		mrpackPath = tmpFile.Name()
	}

	// 2. Parse modrinth.index.json
	index, err := ParseMrpack(mrpackPath)
	if err != nil {
		return nil, err
	}

	// 3. Side Filtering
	side := cfg.Profile.Side
	if side == "" {
		side = "server"
	}

	var targets []TargetFile
	for _, f := range index.Files {
		// Environment side filtering
		if f.Env != nil {
			if strings.EqualFold(side, "server") && strings.EqualFold(f.Env.Server, "unsupported") {
				// Client-only mod, skip on server
				continue
			}
			if strings.EqualFold(side, "client") && strings.EqualFold(f.Env.Client, "unsupported") {
				// Server-only mod, skip on client
				continue
			}
		}

		filename := filepath.Base(f.Path)
		sha512 := f.Hashes["sha512"]
		var downloadURL string
		if len(f.Downloads) > 0 {
			downloadURL = f.Downloads[0]
		}

		slug := config.ExtractModSlug(filename)
		name := slug

		// Extract project ID from CDN URL if present (e.g. https://cdn.modrinth.com/data/<project_id>/versions/...)
		var projectID string
		if downloadURL != "" {
			if idx := strings.Index(downloadURL, "/data/"); idx != -1 {
				sub := downloadURL[idx+6:]
				if vIdx := strings.Index(sub, "/versions/"); vIdx != -1 {
					projectID = sub[:vIdx]
				}
			}
		}

		targets = append(targets, TargetFile{
			FileName:    filename,
			SHA512:      sha512,
			DownloadURL: downloadURL,
			Slug:        slug,
			Name:        name,
			ProjectID:   projectID,
			Side:        side,
		})
	}

	// 4. Delta Synchronization
	localLock, _ := config.LoadLockfile(s.LockPath)
	engine := NewDeltaEngine(s.Client, cfg, localLock, modsDir, s.LockPath)
	return engine.ApplyTargetFiles(targets)
}
