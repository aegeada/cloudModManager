package launcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type vanillaProfilesJSON struct {
	Profiles map[string]vanillaProfile `json:"profiles"`
	Version  int                       `json:"version,omitempty"`
}

type vanillaProfile struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	GameDir       string `json:"gameDir,omitempty"`
	Icon          string `json:"icon,omitempty"`
	LastPlayed    string `json:"lastPlayed,omitempty"`
	LastVersionID string `json:"lastVersionId,omitempty"`
	Created       string `json:"created,omitempty"`
}

// ParseVanillaProfiles reads and parses a launcher_profiles.json file.
func ParseVanillaProfiles(rootDir string) ([]Instance, error) {
	candidates := []string{
		filepath.Join(rootDir, "launcher_profiles.json"),
		filepath.Join(rootDir, "launcher_profiles_microsoft_store.json"),
	}

	var foundFile string
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			foundFile = c
			break
		}
	}

	if foundFile == "" {
		return nil, fmt.Errorf("no vanilla launcher profiles found in '%s'", rootDir)
	}

	data, err := os.ReadFile(foundFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read '%s': %w", foundFile, err)
	}

	var raw vanillaProfilesJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse '%s': %w", foundFile, err)
	}

	var instances []Instance
	for id, prof := range raw.Profiles {
		name := prof.Name
		if name == "" {
			name = id
		}

		instanceDir := rootDir
		if prof.GameDir != "" {
			instanceDir = prof.GameDir
		}

		mcVer, loader, loaderVer := parseVanillaVersion(prof.LastVersionID)

		inst := Instance{
			ID:               id,
			Name:             name,
			Launcher:         LauncherVanilla,
			LauncherName:     "Vanilla Launcher",
			MinecraftVersion: mcVer,
			Loader:           loader,
			LoaderVersion:    loaderVer,
			InstanceDir:      instanceDir,
			ModsDir:          filepath.Join(instanceDir, "mods"),
			ConfigDir:        filepath.Join(instanceDir, "config"),
			Icon:             prof.Icon,
		}

		if prof.LastPlayed != "" {
			if t, err := time.Parse(time.RFC3339Nano, prof.LastPlayed); err == nil {
				inst.LastPlayed = &t
			} else if t, err := time.Parse(time.RFC3339, prof.LastPlayed); err == nil {
				inst.LastPlayed = &t
			}
		}

		instances = append(instances, inst)
	}

	return instances, nil
}

var (
	fabricRegex   = regexp.MustCompile(`(?i)^fabric-loader-([0-9a-zA-Z.-]+)-(1\.[0-9]+(?:\.[0-9]+)?(?:-[a-zA-Z0-9.-]+)?)$`)
	quiltRegex    = regexp.MustCompile(`(?i)^quilt-loader-([0-9a-zA-Z.-]+)-(1\.[0-9]+(?:\.[0-9]+)?(?:-[a-zA-Z0-9.-]+)?)$`)
	forgeRegex1   = regexp.MustCompile(`(?i)^(1\.[0-9]+(?:\.[0-9]+)?)-forge-?([0-9a-zA-Z.-]+)?$`)
	forgeRegex2   = regexp.MustCompile(`(?i)^forge-([0-9a-zA-Z.-]+)-(1\.[0-9]+(?:\.[0-9]+)?)$`)
	neoforgeRegex = regexp.MustCompile(`(?i)^(?:(1\.[0-9]+(?:\.[0-9]+)?)-)?neoforge-?([0-9a-zA-Z.-]+)?$`)
	mcVerRegex    = regexp.MustCompile(`1\.[0-9]+(?:\.[0-9]+)?`)
)

func parseVanillaVersion(lastVer string) (mcVer, loader, loaderVer string) {
	if lastVer == "" {
		return "", "vanilla", ""
	}

	// 1. Fabric: fabric-loader-<loader_ver>-<mc_ver>
	if matches := fabricRegex.FindStringSubmatch(lastVer); len(matches) == 3 {
		return matches[2], "fabric", matches[1]
	}
	if strings.HasPrefix(strings.ToLower(lastVer), "fabric-loader-") {
		parts := strings.Split(strings.TrimPrefix(lastVer, "fabric-loader-"), "-")
		if len(parts) >= 2 {
			return parts[1], "fabric", parts[0]
		}
		return "", "fabric", parts[0]
	}

	// 2. Quilt: quilt-loader-<loader_ver>-<mc_ver>
	if matches := quiltRegex.FindStringSubmatch(lastVer); len(matches) == 3 {
		return matches[2], "quilt", matches[1]
	}
	if strings.HasPrefix(strings.ToLower(lastVer), "quilt-loader-") {
		parts := strings.Split(strings.TrimPrefix(lastVer, "quilt-loader-"), "-")
		if len(parts) >= 2 {
			return parts[1], "quilt", parts[0]
		}
		return "", "quilt", parts[0]
	}

	// 3. NeoForge: 1.21.1-neoforge-21.1.48 or neoforge-21.1.48
	if matches := neoforgeRegex.FindStringSubmatch(lastVer); len(matches) == 3 {
		mc := matches[1]
		lVer := matches[2]
		if mc == "" && lVer != "" {
			// Infer MC from NeoForge version if formatted as Major.Minor (e.g. 21.1.48 -> 1.21.1, 20.4.80 -> 1.20.4, 21.0.1 -> 1.21)
			mc = inferNeoForgeMC(lVer)
		}
		return mc, "neoforge", lVer
	}

	// 4. Forge: 1.20.1-forge-47.3.0 or forge-47.3.0-1.20.1
	if matches := forgeRegex1.FindStringSubmatch(lastVer); len(matches) == 3 {
		return matches[1], "forge", matches[2]
	}
	if matches := forgeRegex2.FindStringSubmatch(lastVer); len(matches) == 3 {
		return matches[2], "forge", matches[1]
	}
	if strings.Contains(strings.ToLower(lastVer), "forge") {
		mc := mcVerRegex.FindString(lastVer)
		return mc, "forge", ""
	}

	// 5. Vanilla: 1.21.1, 1.20.4, etc.
	if mcVerRegex.MatchString(lastVer) {
		return mcVerRegex.FindString(lastVer), "vanilla", ""
	}

	return lastVer, "vanilla", ""
}

func inferNeoForgeMC(neoVer string) string {
	parts := strings.Split(neoVer, ".")
	if len(parts) >= 2 {
		major := parts[0]
		minor := parts[1]
		if major == "20" {
			return "1.20." + minor
		}
		if major == "21" {
			if minor == "0" {
				return "1.21"
			}
			return "1.21." + minor
		}
	}
	return ""
}
