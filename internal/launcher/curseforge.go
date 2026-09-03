package launcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type curseforgeInstanceJSON struct {
	Name          string `json:"name"`
	GameVersion   string `json:"gameVersion"`
	BaseModLoader *struct {
		Name             string `json:"name"`
		Type             int    `json:"type,omitempty"`
		MinecraftVersion string `json:"minecraftVersion,omitempty"`
		ForgeVersion     string `json:"forgeVersion,omitempty"`
	} `json:"baseModLoader,omitempty"`
	Manifest *struct {
		Minecraft struct {
			Version    string `json:"version"`
			ModLoaders []struct {
				ID      string `json:"id"`
				Primary bool   `json:"primary"`
			} `json:"modLoaders"`
		} `json:"minecraft"`
	} `json:"manifest,omitempty"`
	LastPlayed string `json:"lastPlayed,omitempty"`
}

// ParseCurseForgeInstances scans a CurseForge directory for Minecraft instances.
func ParseCurseForgeInstances(rootDir string) ([]Instance, error) {
	candidates := []string{
		filepath.Join(rootDir, "Instances"),
		filepath.Join(rootDir, "instances"),
		rootDir,
	}

	var instancesDir string
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			// Check if this dir directly contains instance folders with minecraftinstance.json
			// or if it's the Instances folder
			if filepath.Base(c) == "Instances" || filepath.Base(c) == "instances" {
				instancesDir = c
				break
			}
			// Check if rootDir contains minecraftinstance.json (is a single instance)
			if _, err := os.Stat(filepath.Join(c, "minecraftinstance.json")); err == nil {
				instancesDir = filepath.Dir(c)
				break
			}
			// Check if subdirectories have minecraftinstance.json
			if entries, err := os.ReadDir(c); err == nil {
				for _, e := range entries {
					if e.IsDir() {
						if _, err := os.Stat(filepath.Join(c, e.Name(), "minecraftinstance.json")); err == nil {
							instancesDir = c
							break
						}
					}
				}
				if instancesDir != "" {
					break
				}
			}
		}
	}

	if instancesDir == "" {
		return nil, fmt.Errorf("no curseforge instances directory found in '%s'", rootDir)
	}

	return scanCurseForgeInstancesDir(instancesDir)
}

func scanCurseForgeInstancesDir(instancesDir string) ([]Instance, error) {
	entries, err := os.ReadDir(instancesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read curseforge instances directory '%s': %w", instancesDir, err)
	}

	var instances []Instance
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		instanceDir := filepath.Join(instancesDir, entry.Name())
		jsonPath := filepath.Join(instanceDir, "minecraftinstance.json")

		data, err := os.ReadFile(jsonPath)
		if err != nil {
			continue
		}

		var raw curseforgeInstanceJSON
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}

		name := raw.Name
		if name == "" {
			name = entry.Name()
		}

		mcVer := raw.GameVersion
		if mcVer == "" && raw.Manifest != nil && raw.Manifest.Minecraft.Version != "" {
			mcVer = raw.Manifest.Minecraft.Version
		}
		if mcVer == "" && raw.BaseModLoader != nil && raw.BaseModLoader.MinecraftVersion != "" {
			mcVer = raw.BaseModLoader.MinecraftVersion
		}

		loader, loaderVer := parseCurseForgeLoader(&raw)

		inst := Instance{
			ID:               entry.Name(),
			Name:             name,
			Launcher:         LauncherCurseForge,
			LauncherName:     "CurseForge App",
			MinecraftVersion: mcVer,
			Loader:           loader,
			LoaderVersion:    loaderVer,
			InstanceDir:      instanceDir,
			ModsDir:          filepath.Join(instanceDir, "mods"),
			ConfigDir:        filepath.Join(instanceDir, "config"),
		}

		if raw.LastPlayed != "" {
			if t, err := time.Parse(time.RFC3339Nano, raw.LastPlayed); err == nil {
				inst.LastPlayed = &t
			} else if t, err := time.Parse(time.RFC3339, raw.LastPlayed); err == nil {
				inst.LastPlayed = &t
			}
		}

		instances = append(instances, inst)
	}

	return instances, nil
}

func parseCurseForgeLoader(raw *curseforgeInstanceJSON) (loader, loaderVer string) {
	loaderStr := ""
	if raw.BaseModLoader != nil && raw.BaseModLoader.Name != "" {
		loaderStr = raw.BaseModLoader.Name
	} else if raw.Manifest != nil && len(raw.Manifest.Minecraft.ModLoaders) > 0 {
		for _, ml := range raw.Manifest.Minecraft.ModLoaders {
			if ml.Primary {
				loaderStr = ml.ID
				break
			}
		}
		if loaderStr == "" {
			loaderStr = raw.Manifest.Minecraft.ModLoaders[0].ID
		}
	}

	if loaderStr == "" {
		return "vanilla", ""
	}

	lower := strings.ToLower(loaderStr)
	switch {
	case strings.HasPrefix(lower, "fabric-"):
		return "fabric", strings.TrimPrefix(loaderStr, "fabric-")
	case strings.HasPrefix(lower, "quilt-"):
		return "quilt", strings.TrimPrefix(loaderStr, "quilt-")
	case strings.HasPrefix(lower, "neoforge-"):
		return "neoforge", strings.TrimPrefix(loaderStr, "neoforge-")
	case strings.HasPrefix(lower, "forge-"):
		return "forge", strings.TrimPrefix(loaderStr, "forge-")
	case lower == "forge" || strings.Contains(lower, "forge"):
		if raw.BaseModLoader != nil && raw.BaseModLoader.ForgeVersion != "" {
			return "forge", raw.BaseModLoader.ForgeVersion
		}
		return "forge", ""
	case lower == "fabric":
		return "fabric", ""
	case lower == "neoforge":
		return "neoforge", ""
	case lower == "quilt":
		return "quilt", ""
	default:
		return "vanilla", ""
	}
}
