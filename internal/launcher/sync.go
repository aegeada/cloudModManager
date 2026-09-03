package launcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cmm/internal/config"
	"cmm/internal/mod"
	"cmm/internal/modrinth"
	"cmm/internal/sync"
)

// Syncer manages synchronizing modpack definitions into Minecraft launcher instances.
type Syncer struct {
	Client   *modrinth.Client
	Detector *Detector
}

// NewSyncer creates a new Syncer instance.
func NewSyncer(client *modrinth.Client, opts DetectorOptions) *Syncer {
	return &Syncer{
		Client:   client,
		Detector: NewDetector(opts),
	}
}

// SyncInstance synchronizes a modpack (from cmm.toml & cmm.lock) directly into a launcher instance.
func (s *Syncer) SyncInstance(opts LauncherSyncOptions) (*LauncherSyncResult, error) {
	if opts.InstanceName == "" {
		return nil, fmt.Errorf("instance name or identifier must be provided")
	}

	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = "cmm.toml"
	}

	lockPath := opts.LockPath
	if lockPath == "" {
		lockPath = "cmm.lock"
	}

	// 1. Resolve target instance
	inst, err := s.Detector.FindInstance(opts.InstanceName, opts.LauncherType)
	if err != nil {
		return nil, err
	}

	result := &LauncherSyncResult{
		Instance:    *inst,
		AddedMods:   []string{},
		UpdatedMods: []string{},
		RemovedMods: []string{},
		SkippedMods: []string{},
	}

	// 2. Load cmm.toml (if present) for compatibility validation
	var cfg *config.Config
	if _, err := os.Stat(configPath); err == nil {
		loadedCfg, err := config.LoadConfig(configPath)
		if err == nil {
			cfg = loadedCfg
		}
	}

	if cfg != nil && !opts.Force {
		// Validate Minecraft Version compatibility
		if inst.MinecraftVersion != "" && cfg.Profile.MinecraftVersion != "" {
			if !strings.EqualFold(inst.MinecraftVersion, cfg.Profile.MinecraftVersion) {
				return nil, fmt.Errorf("instance '%s' uses Minecraft %s, but cmm is configured for %s (use --force to override)",
					inst.Name, inst.MinecraftVersion, cfg.Profile.MinecraftVersion)
			}
		}

		// Validate Mod Loader compatibility
		if inst.Loader != "" && cfg.Profile.Loader != "" && !strings.EqualFold(inst.Loader, "vanilla") {
			if !strings.EqualFold(inst.Loader, cfg.Profile.Loader) {
				return nil, fmt.Errorf("instance '%s' uses loader '%s', but cmm is configured for '%s' (use --force to override)",
					inst.Name, inst.Loader, cfg.Profile.Loader)
			}
		}
	}

	// 3. Load cmm.lock
	lockfile, err := config.LoadLockfile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load lockfile '%s': %w", lockPath, err)
	}

	// 4. Client-side filtering: skip server-only mods
	var clientMods []config.LockfileMod
	for _, m := range lockfile.Mods {
		if strings.EqualFold(m.Side, "server") {
			name := m.Name
			if name == "" {
				name = m.Slug
			}
			result.SkippedMods = append(result.SkippedMods, name)
			continue
		}
		clientMods = append(clientMods, m)
	}

	// 5. Build target files list
	var targets []sync.TargetFile
	for _, m := range clientMods {
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

		targets = append(targets, sync.TargetFile{
			FileName:    fileName,
			SHA512:      m.SHA512,
			DownloadURL: m.DownloadURL,
			Slug:        m.Slug,
			Name:        m.Name,
			ProjectID:   m.ProjectID,
			VersionID:   m.VersionID,
			Version:     m.GetVersion(),
			Side:        m.Side,
		})
	}

	// 6. Handle Dry Run
	if opts.DryRun {
		targetMap := make(map[string]sync.TargetFile)
		for _, t := range targets {
			targetMap[t.FileName] = t
			targetMap[strings.ToLower(t.FileName)] = t
		}

		// Check for mods to remove
		if entries, err := os.ReadDir(inst.ModsDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jar") {
					continue
				}
				if _, ok := targetMap[entry.Name()]; !ok {
					if _, okLower := targetMap[strings.ToLower(entry.Name())]; !okLower {
						result.RemovedMods = append(result.RemovedMods, entry.Name())
					}
				}
			}
		}

		// Check for mods to add or update
		for _, target := range targets {
			destPath := filepath.Join(inst.ModsDir, target.FileName)
			if fi, err := os.Stat(destPath); err == nil && !fi.IsDir() {
				if target.SHA512 != "" {
					localHash, err := mod.ComputeSHA512(destPath)
					if err != nil || !strings.EqualFold(localHash, target.SHA512) {
						result.UpdatedMods = append(result.UpdatedMods, target.FileName)
					}
				}
			} else {
				result.AddedMods = append(result.AddedMods, target.FileName)
			}
		}

		if len(result.AddedMods) == 0 && len(result.UpdatedMods) == 0 && len(result.RemovedMods) == 0 {
			result.UpToDate = true
			result.Message = "Instance is already up to date (dry run)."
		} else {
			result.Message = fmt.Sprintf("Dry run complete: %d to add, %d to update, %d to remove.",
				len(result.AddedMods), len(result.UpdatedMods), len(result.RemovedMods))
		}

		return result, nil
	}

	// 7. Apply Target Files with DeltaEngine
	if err := os.MkdirAll(inst.ModsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create instance mods directory '%s': %w", inst.ModsDir, err)
	}

	instanceLockPath := filepath.Join(inst.InstanceDir, "cmm.lock")
	var instanceLock *config.Lockfile
	if l, err := config.LoadLockfile(instanceLockPath); err == nil {
		instanceLock = l
	} else {
		instanceLock = &config.Lockfile{Mods: []config.LockfileMod{}}
	}

	engine := sync.NewDeltaEngine(s.Client, cfg, instanceLock, inst.ModsDir, instanceLockPath)
	syncRes, err := engine.ApplyTargetFiles(targets)
	if err != nil {
		return nil, fmt.Errorf("failed applying delta sync to instance: %w", err)
	}

	result.AddedMods = syncRes.AddedMods
	result.UpdatedMods = syncRes.UpdatedMods
	result.RemovedMods = syncRes.RemovedMods
	result.UpToDate = syncRes.UpToDate
	result.Message = syncRes.Message

	return result, nil
}
