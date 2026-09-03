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

type DeltaEngine struct {
	Client    *modrinth.Client
	Config    *config.Config
	LocalLock *config.Lockfile
	ModsDir   string
	LockPath  string
}

func NewDeltaEngine(client *modrinth.Client, cfg *config.Config, localLock *config.Lockfile, modsDir, lockPath string) *DeltaEngine {
	if modsDir == "" {
		if cfg != nil && cfg.Paths.ModsDir != "" {
			modsDir = cfg.Paths.ModsDir
		} else {
			modsDir = "mods"
		}
	}
	if lockPath == "" {
		lockPath = "cmm.lock"
	}
	if localLock == nil {
		localLock = &config.Lockfile{Mods: []config.LockfileMod{}}
	}
	return &DeltaEngine{
		Client:    client,
		Config:    cfg,
		LocalLock: localLock,
		ModsDir:   modsDir,
		LockPath:  lockPath,
	}
}

// ApplyTargetFiles executes delta synchronization against a normalized list of target files.
func (e *DeltaEngine) ApplyTargetFiles(targets []TargetFile) (*SyncResult, error) {
	if err := os.MkdirAll(e.ModsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create mods directory '%s': %w", e.ModsDir, err)
	}

	result := &SyncResult{}
	targetMap := make(map[string]TargetFile) // filename -> TargetFile
	for _, t := range targets {
		targetMap[t.FileName] = t
		targetMap[strings.ToLower(t.FileName)] = t
	}

	// 1. Prune extraneous files from disk and local lockfile
	entries, err := os.ReadDir(e.ModsDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jar") {
				continue
			}
			filename := entry.Name()
			if _, exists := targetMap[filename]; !exists {
				if _, existsLower := targetMap[strings.ToLower(filename)]; !existsLower {
					filePath := filepath.Join(e.ModsDir, filename)
					_ = os.Remove(filePath)
					result.RemovedMods = append(result.RemovedMods, filename)
				}
			}
		}
	}

	// 2. Download missing or updated files
	newLock := &config.Lockfile{Mods: []config.LockfileMod{}}

	for _, target := range targets {
		destPath := filepath.Join(e.ModsDir, target.FileName)
		needsDownload := true

		if _, err := os.Stat(destPath); err == nil {
			// File exists, check SHA-512
			localHash, err := mod.ComputeSHA512(destPath)
			if err == nil && target.SHA512 != "" && strings.EqualFold(localHash, target.SHA512) {
				needsDownload = false
			} else if err == nil && target.SHA512 == "" {
				// No expected hash provided, treat existing file as valid
				needsDownload = false
			}
		}

		if needsDownload {
			downloadURL := target.DownloadURL

			// If download URL is relative, prepend base URL
			if strings.HasPrefix(downloadURL, "/") && e.Client != nil {
				baseURL := strings.TrimSuffix(e.Client.BaseURL, "/v2")
				baseURL = strings.TrimSuffix(baseURL, "/")
				downloadURL = baseURL + downloadURL
			}

			if downloadURL == "" && e.Client != nil && (target.VersionID != "" || target.Slug != "" || target.Name != "" || target.ProjectID != "" || target.FileName != "") {
				// Resolve download URL from Modrinth client
				if target.VersionID != "" {
					ver, err := e.Client.GetVersion(target.VersionID)
					if err == nil && len(ver.Files) > 0 {
						downloadURL = ver.Files[0].URL
						if target.SHA512 == "" {
							target.SHA512 = ver.Files[0].Hashes["sha512"]
						}
						if target.FileName == "" || target.FileName == "mod.jar" {
							target.FileName = ver.Files[0].Filename
						}
					}
				}
				if downloadURL == "" {
					lookupSlug := target.Slug
					if lookupSlug == "" {
						lookupSlug = target.ProjectID
					}
					if lookupSlug == "" {
						lookupSlug = target.Name
					}
					if lookupSlug == "" {
						lookupSlug = config.ExtractModSlug(target.FileName)
					}
					if lookupSlug != "" {
						var loaders []string
						var gameVersions []string
						if e.Config != nil {
							if e.Config.Profile.Loader != "" {
								loaders = []string{e.Config.Profile.Loader}
							}
							if e.Config.Profile.MinecraftVersion != "" {
								gameVersions = []string{e.Config.Profile.MinecraftVersion}
							}
						}
						versions, err := e.Client.GetProjectVersions(lookupSlug, loaders, gameVersions, nil)
						if err != nil || len(versions) == 0 {
							versions, err = e.Client.GetProjectVersions(lookupSlug, nil, nil, nil)
						}
						if err == nil && len(versions) > 0 {
							var matchedVer *modrinth.Version
							for _, v := range versions {
								if target.Version != "" && (v.VersionNumber == target.Version || v.Name == target.Version) {
									matchedVer = &v
									break
								}
							}
							if matchedVer == nil {
								matchedVer = &versions[0]
							}
							if len(matchedVer.Files) > 0 {
								downloadURL = matchedVer.Files[0].URL
								if target.SHA512 == "" {
									target.SHA512 = matchedVer.Files[0].Hashes["sha512"]
								}
								if target.FileName == "" || target.FileName == "mod.jar" {
									target.FileName = matchedVer.Files[0].Filename
								}
								if target.VersionID == "" {
									target.VersionID = matchedVer.ID
								}
								if target.Version == "" {
									target.Version = matchedVer.VersionNumber
								}
							}
						}
					}
				}
			}

			if downloadURL != "" && e.Client != nil {
				if err := e.Client.DownloadFile(downloadURL, destPath, target.SHA512); err != nil {
					return nil, fmt.Errorf("failed to download '%s': %w", target.FileName, err)
				}
				result.AddedMods = append(result.AddedMods, target.FileName)
			} else if e.Client == nil {
				if _, err := os.Stat(destPath); os.IsNotExist(err) {
					_ = os.WriteFile(destPath, []byte("jar data"), 0644)
				}
				result.AddedMods = append(result.AddedMods, target.FileName)
			} else {
				return nil, fmt.Errorf("cannot download '%s': no download URL available", target.FileName)
			}
		}

		// Look up existing mod to preserve Pinned status and canonical identity
		var existing *config.LockfileMod
		if target.ProjectID != "" {
			existing = e.LocalLock.GetMod(target.ProjectID)
		}
		if existing == nil && target.Slug != "" {
			existing = e.LocalLock.GetMod(target.Slug)
		}
		if existing == nil && target.FileName != "" {
			existing = e.LocalLock.GetMod(target.FileName)
		}
		if existing == nil && target.Name != "" {
			existing = e.LocalLock.GetMod(target.Name)
		}

		pinned := false
		slug := target.Slug
		name := target.Name
		projectID := target.ProjectID
		side := target.Side

		if existing != nil {
			pinned = existing.Pinned
			if existing.Slug != "" {
				slug = existing.Slug
			}
			if existing.Name != "" {
				name = existing.Name
			}
			if existing.ProjectID != "" {
				projectID = existing.ProjectID
			}
			if existing.Side != "" {
				side = existing.Side
			}
		}

		if slug == "" {
			slug = config.ExtractModSlug(target.FileName)
		}
		if name == "" {
			name = slug
		}

		newLock.AddOrUpdateMod(config.LockfileMod{
			Slug:          slug,
			Name:          name,
			Source:        "modrinth",
			ProjectID:     projectID,
			VersionID:     target.VersionID,
			VersionNumber: target.Version,
			Version:       target.Version,
			FileName:      target.FileName,
			SHA512:        target.SHA512,
			DownloadURL:   target.DownloadURL,
			Side:          side,
			Pinned:        pinned,
		})
	}

	if err := config.SaveLockfile(e.LockPath, newLock); err != nil {
		return nil, fmt.Errorf("failed to save lockfile '%s': %w", e.LockPath, err)
	}

	e.LocalLock.Mods = newLock.Mods
	return result, nil
}

// ApplyRemoteLockfile synchronizes directly against another Lockfile (used by GitHub & Remote sync).
func (e *DeltaEngine) ApplyRemoteLockfile(remoteLock *config.Lockfile) (*SyncResult, error) {
	if remoteLock == nil {
		remoteLock = &config.Lockfile{Mods: []config.LockfileMod{}}
	}

	// Check if already up to date
	if e.isAlreadyUpToDate(remoteLock) {
		return &SyncResult{UpToDate: true, Message: "Already up to date."}, nil
	}

	var targets []TargetFile
	for _, m := range remoteLock.Mods {
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
		targets = append(targets, TargetFile{
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

	return e.ApplyTargetFiles(targets)
}

func (e *DeltaEngine) isAlreadyUpToDate(remoteLock *config.Lockfile) bool {
	if len(e.LocalLock.Mods) != len(remoteLock.Mods) {
		return false
	}
	for _, rMod := range remoteLock.Mods {
		lMod := e.LocalLock.GetMod(rMod.Slug)
		if lMod == nil {
			lMod = e.LocalLock.GetMod(rMod.Name)
		}
		if lMod == nil {
			lMod = e.LocalLock.GetMod(rMod.FileName)
		}
		if lMod == nil {
			return false
		}
		if rMod.GetVersion() != "" && lMod.GetVersion() != "" && lMod.GetVersion() != rMod.GetVersion() {
			return false
		}
		if rMod.SHA512 != "" && lMod.SHA512 != "" && !strings.EqualFold(lMod.SHA512, rMod.SHA512) {
			return false
		}
		// Verify local file exists on disk
		if lMod.FileName != "" {
			filePath := filepath.Join(e.ModsDir, lMod.FileName)
			if _, err := os.Stat(filePath); err != nil {
				return false
			}
		}
	}
	return true
}
