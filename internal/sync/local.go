package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cmm/internal/config"
	"cmm/internal/mod"
	"cmm/internal/modrinth"
)

type LocalSynchronizer struct {
	Client     *modrinth.Client
	ConfigPath string
	LockPath   string
}

func NewLocalSynchronizer(client *modrinth.Client, configPath, lockPath string) *LocalSynchronizer {
	if configPath == "" {
		configPath = "cmm.toml"
	}
	if lockPath == "" {
		lockPath = "cmm.lock"
	}
	return &LocalSynchronizer{
		Client:     client,
		ConfigPath: configPath,
		LockPath:   lockPath,
	}
}

func (s *LocalSynchronizer) Sync(opts LocalSyncOptions) (*SyncResult, error) {
	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	modsDir := opts.Path
	if modsDir == "" {
		modsDir = cfg.Paths.ModsDir
	}
	if modsDir == "" {
		modsDir = "mods"
	}
	if !filepath.IsAbs(modsDir) && opts.Path == "" && s.ConfigPath != "" {
		cfgDir := filepath.Dir(s.ConfigPath)
		if cfgDir != "" && cfgDir != "." {
			modsDir = filepath.Join(cfgDir, modsDir)
		}
	}

	// 1. Validate target directory existence
	stat, err := os.Stat(modsDir)
	if err != nil {
		if opts.Path != "" {
			return nil, fmt.Errorf("directory '%s' does not exist", modsDir)
		}
		// If default directory does not exist, treat as empty mods directory
		return &SyncResult{Message: "No mods found"}, nil
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("path '%s' is not a directory", modsDir)
	}

	// 2. Discover .jar and .jar.disabled files
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory '%s': %w", modsDir, err)
	}

	var jarFiles []string
	for _, e := range entries {
		lowerName := strings.ToLower(e.Name())
		if !e.IsDir() && (strings.HasSuffix(lowerName, ".jar") || strings.HasSuffix(lowerName, ".jar.disabled")) {
			jarFiles = append(jarFiles, e.Name())
		}
	}

	if len(jarFiles) == 0 {
		return &SyncResult{Message: "No mods found"}, nil
	}

	result := &SyncResult{}
	unreadableJars := make(map[string]bool)
	emptyJars := make(map[string]bool)

	// 3. Compute SHA-512 and SHA-1 checksums
	sha512ToName := make(map[string]string)
	nameToSha512 := make(map[string]string)
	var sha512List []string

	sha1ToName := make(map[string]string)
	var sha1List []string

	for _, jar := range jarFiles {
		fullPath := filepath.Join(modsDir, jar)
		fi, err := os.Stat(fullPath)
		if err != nil {
			result.UnreadableFiles = append(result.UnreadableFiles, jar)
			unreadableJars[jar] = true
			continue
		}
		if fi.Size() == 0 {
			result.EmptyJars = append(result.EmptyJars, jar)
			emptyJars[jar] = true
			continue
		}

		h512, err512 := mod.ComputeSHA512(fullPath)
		if err512 != nil || h512 == "" {
			result.UnreadableFiles = append(result.UnreadableFiles, jar)
			unreadableJars[jar] = true
			continue
		}
		sha512ToName[h512] = jar
		nameToSha512[jar] = h512
		sha512List = append(sha512List, h512)

		h1, err1 := mod.ComputeSHA1(fullPath)
		if err1 == nil && h1 != "" {
			sha1ToName[h1] = jar
			sha1List = append(sha1List, h1)
		}
	}

	// 4. Batch query Modrinth /version_files
	versionMap := make(map[string]modrinth.Version)
	if s.Client != nil && len(sha512List) > 0 {
		vm, err := s.Client.GetMultipleVersionsFromHashes(sha512List, "sha512")
		if err == nil && vm != nil {
			versionMap = vm
		}
	}

	// 5. Load existing lockfile to preserve pinned states
	lock, err := config.LoadLockfile(s.LockPath)
	if err != nil || lock == nil {
		lock = &config.Lockfile{Mods: []config.LockfileMod{}}
	}

	matchedJars := make(map[string]bool)

	// Process matched versions
	for hash, ver := range versionMap {
		jarName := sha512ToName[hash]
		if jarName == "" {
			jarName = sha1ToName[hash]
		}
		if jarName != "" {
			matchedJars[jarName] = true
		}

		// Look up existing mod to preserve Pinned status and metadata
		var existing *config.LockfileMod
		if ver.ProjectID != "" {
			existing = lock.GetMod(ver.ProjectID)
		}
		if existing == nil && jarName != "" {
			existing = lock.GetMod(config.ExtractModSlug(jarName))
		}
		if existing == nil && jarName != "" {
			existing = lock.GetMod(jarName)
		}
		if existing == nil && ver.Name != "" {
			existing = lock.GetMod(ver.Name)
		}

		pinned := false
		if existing != nil {
			pinned = existing.Pinned
		}

		slug := ver.ProjectID
		name := ver.Name
		side := "both"

		clientSide := ""
		serverSide := ""

		if s.Client != nil && ver.ProjectID != "" {
			if proj, err := s.Client.GetProject(ver.ProjectID); err == nil && proj != nil {
				if proj.Slug != "" {
					slug = proj.Slug
				}
				if proj.Title != "" {
					name = proj.Title
				}
				clientSide = proj.ClientSide
				serverSide = proj.ServerSide
				if proj.ServerSide != "" || proj.ClientSide != "" {
					side = config.FormatDetailedSide(proj.ClientSide, proj.ServerSide)
				}
			}
		}

		if existing != nil {
			if existing.Slug != "" {
				slug = existing.Slug
			}
			if existing.Name != "" {
				name = existing.Name
			}
			if existing.ClientSide != "" {
				clientSide = existing.ClientSide
			}
			if existing.ServerSide != "" {
				serverSide = existing.ServerSide
			}
			if existing.Side != "" {
				side = config.NormalizeSide(existing.Side)
			}
		}

		if slug == "" {
			if jarName != "" {
				slug = config.ExtractModSlug(jarName)
			} else {
				slug = ver.ProjectID
			}
		}
		if name == "" {
			name = slug
		}

		var downloadURL string
		for _, f := range ver.Files {
			if f.Primary || downloadURL == "" {
				downloadURL = f.URL
			}
		}

		disabled := strings.HasSuffix(strings.ToLower(jarName), ".jar.disabled")

		entry := config.LockfileMod{
			Slug:          slug,
			Name:          name,
			Source:        "modrinth",
			ProjectID:     ver.ProjectID,
			VersionID:     ver.ID,
			VersionNumber: ver.VersionNumber,
			Version:       ver.VersionNumber,
			FileName:      jarName,
			SHA512:        hash,
			DownloadURL:   downloadURL,
			ClientSide:    clientSide,
			ServerSide:    serverSide,
			Side:          side,
			Pinned:        pinned,
			Disabled:      disabled,
		}
		lock.AddOrUpdateMod(entry)
		result.AddedMods = append(result.AddedMods, name)
	}

	// 6. Identify unrecognized JAR files
	for _, jar := range jarFiles {
		if !matchedJars[jar] && !unreadableJars[jar] && !emptyJars[jar] {
			result.UnknownJars = append(result.UnknownJars, jar)
		}
	}

	// 7. Save updated lockfile
	if err := config.SaveLockfile(s.LockPath, lock); err != nil {
		return nil, fmt.Errorf("failed to save lockfile '%s': %w", s.LockPath, err)
	}

	return result, nil
}
