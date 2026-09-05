package mod

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cmm/internal/config"
	"cmm/internal/modrinth"
)

type Manager struct {
	Client     *modrinth.Client
	ConfigPath string
	LockPath   string
	Resolver   *DependencyResolver
}

func NewManager(client *modrinth.Client, configPath, lockPath string) *Manager {
	if configPath == "" {
		configPath = "cmm.toml"
	}
	if lockPath == "" {
		lockPath = "cmm.lock"
	}
	return &Manager{
		Client:     client,
		ConfigPath: configPath,
		LockPath:   lockPath,
		Resolver:   NewDependencyResolver(client),
	}
}

func (m *Manager) loadConfig() *config.Config {
	cfg, err := config.LoadConfig(m.ConfigPath)
	if err != nil {
		return config.DefaultConfig()
	}
	return cfg
}

func (m *Manager) loadLockfile() *config.Lockfile {
	lock, err := config.LoadLockfile(m.LockPath)
	if err != nil {
		return &config.Lockfile{Mods: []config.LockfileMod{}}
	}
	return lock
}

// Add installs a mod and its required dependencies recursively using default release channel.
func (m *Manager) Add(slugOrID string, targetVersion string) (*AddResult, error) {
	return m.AddWithChannelAndReplace(slugOrID, targetVersion, "release", false)
}

// AddWithChannelAndReplace installs a mod with stability channel filter and replacement control.
func (m *Manager) AddWithChannelAndReplace(slugOrID string, targetVersion string, channel string, allowReplace bool) (*AddResult, error) {
	cfg := m.loadConfig()
	lock := m.loadLockfile()

	// Check if mod is already installed
	existing := lock.GetMod(slugOrID)
	wasExisting := (existing != nil)
	if wasExisting {
		if !allowReplace {
			return &AddResult{
				AlreadyInstalled: true,
				InstalledMod:     existing,
			}, nil
		}
	}

	resolver := NewDependencyResolver(m.Client)
	installQueue, optionalDeps, targetProj, err := resolver.ResolvePlanWithChannel(slugOrID, targetVersion, channel, cfg.Profile, lock)
	if err != nil {
		return nil, err
	}

	// Server-side check skipped client-only mod
	if len(installQueue) == 0 && targetProj != nil && strings.EqualFold(cfg.Profile.Side, "server") && strings.EqualFold(targetProj.ServerSide, "unsupported") {
		ver := targetVersion
		if ver == "" {
			if vs, err := m.Client.GetProjectVersions(targetProj.ID, nil, nil, nil); err == nil && len(vs) > 0 {
				ver = vs[0].VersionNumber
			}
		}
		if wasExisting && allowReplace && existing != nil && existing.FileName != "" {
			modsDir := cfg.Paths.ModsDir
			if modsDir == "" {
				modsDir = "mods"
			}
			_ = os.Remove(filepath.Join(modsDir, existing.FileName))
		}
		return &AddResult{
			SkippedClientOnly: true,
			Replaced:          wasExisting && allowReplace,
			InstalledMod: &config.LockfileMod{
				Slug:          targetProj.Slug,
				Name:          targetProj.Title,
				VersionNumber: ver,
				Version:       ver,
			},
		}, nil
	}

	modsDir := cfg.Paths.ModsDir
	if modsDir == "" {
		modsDir = "mods"
	}
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create mods directory %s: %w", modsDir, err)
	}

	res := &AddResult{
		OptionalDeps: optionalDeps,
	}

	var filesToCleanup []string
	var downloadedTmpFiles []string
	defer func() {
		for _, tmp := range downloadedTmpFiles {
			_ = os.Remove(tmp)
		}
	}()

	for _, item := range installQueue {
		isPinned := false
		isDisabled := false
		var oldFileName string
		if existing := lock.GetMod(item.Project.Slug); existing != nil {
			isPinned = existing.Pinned
			isDisabled = existing.Disabled
			oldFileName = existing.FileName
		} else if existing := lock.GetMod(item.Project.ID); existing != nil {
			isPinned = existing.Pinned
			isDisabled = existing.Disabled
			oldFileName = existing.FileName
		}

		targetFileName := item.File.Filename
		if isDisabled && !strings.HasSuffix(strings.ToLower(targetFileName), ".disabled") {
			targetFileName += ".disabled"
		}

		destPath := filepath.Join(modsDir, targetFileName)
		tmpDestPath := filepath.Join(modsDir, targetFileName+".tmp")
		downloadedTmpFiles = append(downloadedTmpFiles, tmpDestPath)
		sha512 := item.File.Hashes["sha512"]

		// Download to .tmp first and verify hash
		if err := m.Client.DownloadFile(item.File.URL, tmpDestPath, sha512); err != nil {
			return nil, fmt.Errorf("failed to download file '%s': %w", item.File.Filename, err)
		}

		// Atomically rename .tmp to destination
		if err := os.Rename(tmpDestPath, destPath); err != nil {
			return nil, fmt.Errorf("failed to install file '%s': %w", targetFileName, err)
		}

		downloadedTmpFiles = downloadedTmpFiles[:len(downloadedTmpFiles)-1]

		if oldFileName != "" && oldFileName != targetFileName {
			filesToCleanup = append(filesToCleanup, filepath.Join(modsDir, oldFileName))
		}

		clientSide := item.Project.ClientSide
		serverSide := item.Project.ServerSide
		side := config.FormatDetailedSide(clientSide, serverSide)

		entry := config.LockfileMod{
			Slug:          item.Project.Slug,
			Name:          item.Project.Title,
			Source:        "modrinth",
			ProjectID:     item.Project.ID,
			VersionID:     item.Version.ID,
			VersionNumber: item.Version.VersionNumber,
			Version:       item.Version.VersionNumber,
			FileName:      targetFileName,
			SHA512:        sha512,
			DownloadURL:   item.File.URL,
			ClientSide:    clientSide,
			ServerSide:    serverSide,
			Side:          side,
			Pinned:        isPinned,
			Disabled:      isDisabled,
		}

		lock.AddOrUpdateMod(entry)

		if strings.EqualFold(item.Project.Slug, targetProj.Slug) || strings.EqualFold(item.Project.ID, targetProj.ID) {
			res.InstalledMod = &entry
		} else {
			res.InstalledDeps = append(res.InstalledDeps, &entry)
		}
	}

	if wasExisting && allowReplace {
		res.Replaced = true
	}
	for _, oldPath := range filesToCleanup {
		_ = os.Remove(oldPath)
	}

	if err := config.SaveLockfile(m.LockPath, lock); err != nil {
		return nil, fmt.Errorf("failed to save lockfile %s: %w", m.LockPath, err)
	}

	return res, nil
}

// GetCompatibleVersions returns versions matching project, loader, game version, and channel.
func (m *Manager) GetCompatibleVersions(slugOrID string, channel string) ([]modrinth.Version, *modrinth.Project, error) {
	cfg := m.loadConfig()
	proj, err := m.Client.GetProject(slugOrID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve project '%s': %w", slugOrID, err)
	}

	var loaders []string
	if cfg.Profile.Loader != "" {
		loaders = []string{cfg.Profile.Loader}
	}
	var gameVersions []string
	if cfg.Profile.MinecraftVersion != "" {
		gameVersions = []string{cfg.Profile.MinecraftVersion}
	}

	versions, err := m.Client.GetProjectVersions(proj.ID, loaders, gameVersions, nil)
	if err != nil || len(versions) == 0 {
		versions, err = m.Client.GetProjectVersions(proj.ID, nil, nil, nil)
		if err != nil {
			return nil, proj, err
		}
	}

	filtered := modrinth.FilterVersionsByChannel(versions, channel)
	return filtered, proj, nil
}

// Remove removes a mod and identifies orphaned dependencies.
func (m *Manager) Remove(slugOrID string, dryRun bool) (*RemoveResult, error) {
	cfg := m.loadConfig()
	lock := m.loadLockfile()

	mod := lock.GetMod(slugOrID)
	if mod == nil {
		return nil, fmt.Errorf("mod '%s' is not installed", slugOrID)
	}

	resolver := NewDependencyResolver(m.Client)
	orphans, _ := resolver.DetectOrphans(slugOrID, lock)

	modsDir := cfg.Paths.ModsDir
	if modsDir == "" {
		modsDir = "mods"
	}

	res := &RemoveResult{
		RemovedMod:   mod.Name,
		RemovedFiles: []string{filepath.Join(modsDir, mod.FileName)},
		OrphanedDeps: orphans,
		DryRun:       dryRun,
	}

	if dryRun {
		return res, nil
	}

	if mod.FileName != "" {
		_ = os.Remove(filepath.Join(modsDir, mod.FileName))
	}

	lock.RemoveMod(slugOrID)
	if err := config.SaveLockfile(m.LockPath, lock); err != nil {
		return nil, fmt.Errorf("failed to save lockfile: %w", err)
	}

	return res, nil
}

// RemoveOrphan removes an orphaned mod file and lockfile entry.
func (m *Manager) RemoveOrphan(orphanSlugOrName string) error {
	cfg := m.loadConfig()
	lock := m.loadLockfile()

	mod := lock.GetMod(orphanSlugOrName)
	if mod == nil {
		return nil
	}

	modsDir := cfg.Paths.ModsDir
	if modsDir == "" {
		modsDir = "mods"
	}

	if mod.FileName != "" {
		_ = os.Remove(filepath.Join(modsDir, mod.FileName))
	}

	lock.RemoveMod(orphanSlugOrName)
	return config.SaveLockfile(m.LockPath, lock)
}

// List returns the status of all installed mods.
func (m *Manager) List() ([]ModStatus, error) {
	cfg := m.loadConfig()
	lock := m.loadLockfile()

	if len(lock.Mods) == 0 {
		return []ModStatus{}, nil
	}

	var loaders []string
	if cfg.Profile.Loader != "" {
		loaders = []string{cfg.Profile.Loader}
	}
	var gameVersions []string
	if cfg.Profile.MinecraftVersion != "" {
		gameVersions = []string{cfg.Profile.MinecraftVersion}
	}

	var result []ModStatus
	for _, mod := range lock.Mods {
		st := ModStatus{
			Name:            mod.Name,
			Slug:            mod.Slug,
			Version:         mod.GetVersion(),
			Side:            config.NormalizeSide(mod.Side),
			Pinned:          mod.Pinned,
			Disabled:        mod.Disabled,
			UpdateAvailable: false,
		}
		if st.Name == "" {
			st.Name = mod.Slug
		}
		if st.Slug == "" {
			st.Slug = mod.Name
		}

		// Check Modrinth for updates
		if m.Client != nil && mod.Slug != "" {
			if versions, err := m.Client.GetProjectVersions(mod.Slug, loaders, gameVersions, nil); err == nil && len(versions) > 0 {
				latest := versions[0]
				if latest.VersionNumber != "" && IsNewerVersion(latest.VersionNumber, mod.GetVersion()) {
					st.UpdateAvailable = true
					st.LatestVersion = latest.VersionNumber
				}
			}
		}

		result = append(result, st)
	}

	return result, nil
}

// Pin sets a mod's pinned state to true in cmm.lock.
func (m *Manager) Pin(slugOrID string, targetVersion string) error {
	lock := m.loadLockfile()

	mod := lock.GetMod(slugOrID)
	if mod == nil {
		return fmt.Errorf("Mod not found in lockfile: %s", slugOrID)
	}

	mod.Pinned = true
	if targetVersion != "" {
		mod.VersionNumber = targetVersion
		mod.Version = targetVersion
	}

	return config.SaveLockfile(m.LockPath, lock)
}

// Unpin sets a mod's pinned state to false in cmm.lock.
func (m *Manager) Unpin(slugOrID string) (alreadyUnpinned bool, err error) {
	lock := m.loadLockfile()

	mod := lock.GetMod(slugOrID)
	if mod == nil {
		return false, fmt.Errorf("Mod not found in lockfile: %s", slugOrID)
	}

	if !mod.Pinned {
		return true, nil
	}

	mod.Pinned = false
	if err := config.SaveLockfile(m.LockPath, lock); err != nil {
		return false, err
	}
	return false, nil
}

// CheckUpdates finds update candidates for installed mods.
func (m *Manager) CheckUpdates(slugOrID string, force bool) ([]UpdateCandidate, []string, error) {
	var slugs []string
	if slugOrID != "" {
		slugs = []string{slugOrID}
	}
	return m.CheckUpdatesMulti(slugs, "release", force)
}

// CheckUpdatesMulti checks updates for specified slugs (or all if empty) on the given stability channel.
func (m *Manager) CheckUpdatesMulti(slugs []string, channel string, force bool) ([]UpdateCandidate, []string, error) {
	cfg := m.loadConfig()
	lock := m.loadLockfile()

	var targets []config.LockfileMod
	if len(slugs) > 0 {
		slugMap := make(map[string]bool)
		for _, s := range slugs {
			slugMap[strings.ToLower(strings.TrimSpace(s))] = true
		}
		for _, mod := range lock.Mods {
			if slugMap[strings.ToLower(mod.Slug)] || slugMap[strings.ToLower(mod.Name)] || (mod.ProjectID != "" && slugMap[strings.ToLower(mod.ProjectID)]) {
				targets = append(targets, mod)
			}
		}
	} else {
		targets = lock.Mods
	}

	var loaders []string
	if cfg.Profile.Loader != "" {
		loaders = []string{cfg.Profile.Loader}
	}
	var gameVersions []string
	if cfg.Profile.MinecraftVersion != "" {
		gameVersions = []string{cfg.Profile.MinecraftVersion}
	}

	var candidates []UpdateCandidate
	var skippedPinned []string

	for _, mod := range targets {
		if mod.Pinned && !force {
			skippedPinned = append(skippedPinned, mod.Slug)
			continue
		}

		slug := mod.Slug
		if slug == "" {
			slug = mod.Name
		}

		versions, err := m.Client.GetProjectVersions(slug, loaders, gameVersions, nil)
		if err != nil || len(versions) == 0 {
			continue
		}

		filtered := modrinth.FilterVersionsByChannel(versions, channel)
		if len(filtered) == 0 {
			continue
		}

		// Find index of currently installed version in the full project versions list
		currVer := mod.GetVersion()
		currVerID := mod.VersionID
		currIndex := -1
		for i, v := range versions {
			if (currVerID != "" && strings.EqualFold(v.ID, currVerID)) || strings.EqualFold(v.VersionNumber, currVer) {
				currIndex = i
				break
			}
		}

		// Look for the newest version in filtered channel that is strictly newer than current
		var bestCandidate *modrinth.Version
		for _, fv := range filtered {
			if fv.VersionNumber == "" || strings.EqualFold(fv.VersionNumber, currVer) {
				continue
			}

			// If current version was found in project versions list, candidate must be chronologically newer (index < currIndex)
			if currIndex != -1 {
				candIndex := -1
				for i, v := range versions {
					if v.ID == fv.ID {
						candIndex = i
						break
					}
				}
				if candIndex >= currIndex {
					// candidate was published before or at the same time as current; not an update!
					continue
				}
			}

			// SemVer comparison: candidate must be strictly newer than currVer
			if !IsNewerVersion(fv.VersionNumber, currVer) {
				continue
			}

			bestCandidate = &fv
			break
		}

		if bestCandidate != nil {
			candidates = append(candidates, UpdateCandidate{
				Mod:           mod,
				TargetVersion: *bestCandidate,
				Changelog:     bestCandidate.Changelog,
			})
		}
	}

	return candidates, skippedPinned, nil
}

// ApplyUpdates downloads new version JARs and updates cmm.lock.
func (m *Manager) ApplyUpdates(candidates []UpdateCandidate) error {
	cfg := m.loadConfig()
	lock := m.loadLockfile()

	modsDir := cfg.Paths.ModsDir
	if modsDir == "" {
		modsDir = "mods"
	}
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		return err
	}

	var downloadedTmpFiles []string
	defer func() {
		for _, tmp := range downloadedTmpFiles {
			_ = os.Remove(tmp)
		}
	}()

	for _, c := range candidates {
		file, err := selectFile(&c.TargetVersion)
		if err != nil {
			return err
		}

		targetFileName := file.Filename
		if c.Mod.Disabled && !strings.HasSuffix(strings.ToLower(targetFileName), ".disabled") {
			targetFileName += ".disabled"
		}

		destPath := filepath.Join(modsDir, targetFileName)
		tmpDestPath := filepath.Join(modsDir, targetFileName+".tmp")
		downloadedTmpFiles = append(downloadedTmpFiles, tmpDestPath)
		sha512 := file.Hashes["sha512"]

		// Stage into .tmp first and verify hash
		if err := m.Client.DownloadFile(file.URL, tmpDestPath, sha512); err != nil {
			return fmt.Errorf("failed to download update for %s: %w", c.Mod.Slug, err)
		}

		// Atomically move .tmp to destPath
		if err := os.Rename(tmpDestPath, destPath); err != nil {
			return fmt.Errorf("failed to install update for %s: %w", c.Mod.Slug, err)
		}

		downloadedTmpFiles = downloadedTmpFiles[:len(downloadedTmpFiles)-1]

		// Only remove previous file after download succeeds
		if c.Mod.FileName != "" && c.Mod.FileName != targetFileName {
			_ = os.Remove(filepath.Join(modsDir, c.Mod.FileName))
		}

		entry := c.Mod
		entry.VersionID = c.TargetVersion.ID
		entry.VersionNumber = c.TargetVersion.VersionNumber
		entry.Version = c.TargetVersion.VersionNumber
		entry.FileName = targetFileName
		entry.SHA512 = sha512
		entry.DownloadURL = file.URL

		lock.AddOrUpdateMod(entry)
	}

	return config.SaveLockfile(m.LockPath, lock)
}
