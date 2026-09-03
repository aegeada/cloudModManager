package launcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type modrinthProfileJSON struct {
	Name          string      `json:"name"`
	GameVersion   string      `json:"game_version"`
	GameVersionAlt string     `json:"gameVersion,omitempty"`
	Loader        string      `json:"loader"`
	LoaderVersion string      `json:"loader_version"`
	LoaderVerAlt  string      `json:"loaderVersion,omitempty"`
	Icon          string      `json:"icon,omitempty"`
	LastPlayed    interface{} `json:"last_played,omitempty"`
	LastPlayedAlt interface{} `json:"lastPlayed,omitempty"`
}

// ParseModrinthProfiles scans a Modrinth App root directory for profiles.
func ParseModrinthProfiles(rootDir string) ([]Instance, error) {
	profilesDir := filepath.Join(rootDir, "profiles")
	if fi, err := os.Stat(profilesDir); err == nil && fi.IsDir() {
		return scanModrinthProfilesDir(profilesDir)
	}

	if fi, err := os.Stat(rootDir); err == nil && fi.IsDir() {
		return scanModrinthProfilesDir(rootDir)
	}

	return nil, fmt.Errorf("no modrinth profiles directory found in '%s'", rootDir)
}

func scanModrinthProfilesDir(profilesDir string) ([]Instance, error) {
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read modrinth profiles directory '%s': %w", profilesDir, err)
	}

	var instances []Instance
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		profileDir := filepath.Join(profilesDir, entry.Name())
		profileJSONPath := filepath.Join(profileDir, "profile.json")

		data, err := os.ReadFile(profileJSONPath)
		if err != nil {
			continue
		}

		var raw modrinthProfileJSON
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}

		name := raw.Name
		if name == "" {
			name = entry.Name()
		}

		mcVer := raw.GameVersion
		if mcVer == "" {
			mcVer = raw.GameVersionAlt
		}

		loader := strings.ToLower(raw.Loader)
		if loader == "" {
			loader = "vanilla"
		}

		loaderVer := raw.LoaderVersion
		if loaderVer == "" {
			loaderVer = raw.LoaderVerAlt
		}

		inst := Instance{
			ID:               entry.Name(),
			Name:             name,
			Launcher:         LauncherModrinth,
			LauncherName:     "Modrinth App",
			MinecraftVersion: mcVer,
			Loader:           loader,
			LoaderVersion:    loaderVer,
			InstanceDir:      profileDir,
			ModsDir:          filepath.Join(profileDir, "mods"),
			ConfigDir:        filepath.Join(profileDir, "config"),
			Icon:             raw.Icon,
		}

		lp := raw.LastPlayed
		if lp == nil {
			lp = raw.LastPlayedAlt
		}
		if lp != nil {
			inst.LastPlayed = parseGenericTimestamp(lp)
		}

		instances = append(instances, inst)
	}

	return instances, nil
}

func parseGenericTimestamp(val interface{}) *time.Time {
	switch v := val.(type) {
	case float64:
		if v > 0 {
			// If > 1e11 assume epoch milliseconds, else epoch seconds
			if v > 1e11 {
				t := time.UnixMilli(int64(v))
				return &t
			}
			t := time.Unix(int64(v), 0)
			return &t
		}
	case int64:
		if v > 0 {
			if v > 1e11 {
				t := time.UnixMilli(v)
				return &t
			}
			t := time.Unix(v, 0)
			return &t
		}
	case string:
		if num, err := strconv.ParseInt(v, 10, 64); err == nil && num > 0 {
			if num > 1e11 {
				t := time.UnixMilli(num)
				return &t
			}
			t := time.Unix(num, 0)
			return &t
		}
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return &t
		}
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return &t
		}
	}
	return nil
}
