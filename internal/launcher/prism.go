package launcher

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type mmcPackJSON struct {
	Components    []mmcComponent `json:"components"`
	FormatVersion int            `json:"formatVersion"`
}

type mmcComponent struct {
	CachedName    string `json:"cachedName,omitempty"`
	CachedVersion string `json:"cachedVersion,omitempty"`
	UID           string `json:"uid"`
	Version       string `json:"version,omitempty"`
	Important     bool   `json:"important,omitempty"`
}

// ParsePrismInstances scans a PrismLauncher / MultiMC directory for instances.
func ParsePrismInstances(rootDir string) ([]Instance, error) {
	instancesDir := filepath.Join(rootDir, "instances")
	if fi, err := os.Stat(instancesDir); err == nil && fi.IsDir() {
		return scanPrismInstancesDir(instancesDir)
	}

	// Check if rootDir itself is the instances directory
	if fi, err := os.Stat(rootDir); err == nil && fi.IsDir() {
		return scanPrismInstancesDir(rootDir)
	}

	return nil, fmt.Errorf("no prism instances directory found in '%s'", rootDir)
}

func scanPrismInstancesDir(instancesDir string) ([]Instance, error) {
	var instances []Instance

	// Walk up to depth 3 to find folders containing instance.cfg or mmc-pack.json
	err := filepath.Walk(instancesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		if path == instancesDir {
			return nil
		}

		rel, err := filepath.Rel(instancesDir, path)
		if err != nil {
			return nil
		}
		depth := len(strings.Split(filepath.ToSlash(rel), "/"))
		if depth > 3 {
			return filepath.SkipDir
		}

		cfgPath := filepath.Join(path, "instance.cfg")
		packPath := filepath.Join(path, "mmc-pack.json")

		hasCfg := false
		if fi, err := os.Stat(cfgPath); err == nil && !fi.IsDir() {
			hasCfg = true
		}
		hasPack := false
		if fi, err := os.Stat(packPath); err == nil && !fi.IsDir() {
			hasPack = true
		}

		if hasCfg || hasPack {
			inst, err := parseSinglePrismInstance(path, cfgPath, packPath, rel)
			if err == nil {
				instances = append(instances, *inst)
			}
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error scanning prism instances: %w", err)
	}

	return instances, nil
}

func parseSinglePrismInstance(instanceDir, cfgPath, packPath, relID string) (*Instance, error) {
	cfgMap := make(map[string]string)
	if data, err := os.ReadFile(cfgPath); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				cfgMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	name := cfgMap["name"]
	if name == "" {
		name = filepath.Base(instanceDir)
	}

	mcVer := cfgMap["IntendedVersion"]
	loader := "vanilla"
	loaderVer := ""

	// Parse mmc-pack.json if present
	if data, err := os.ReadFile(packPath); err == nil {
		var pack mmcPackJSON
		if err := json.Unmarshal(data, &pack); err == nil {
			for _, comp := range pack.Components {
				uid := strings.ToLower(comp.UID)
				compVer := comp.Version
				if compVer == "" {
					compVer = comp.CachedVersion
				}

				switch {
				case uid == "net.minecraft":
					mcVer = compVer
				case strings.Contains(uid, "fabric-loader") || uid == "net.fabricmc.fabric-loader":
					loader = "fabric"
					loaderVer = compVer
				case strings.Contains(uid, "quilt-loader") || uid == "org.quiltmc.quilt-loader":
					loader = "quilt"
					loaderVer = compVer
				case strings.Contains(uid, "neoforge") || uid == "net.neoforged" || uid == "net.neoforged.neoforge":
					loader = "neoforge"
					loaderVer = compVer
				case strings.Contains(uid, "forge") || uid == "net.minecraftforge":
					loader = "forge"
					loaderVer = compVer
				}
			}
		}
	}

	// Resolve mods & config directory (.minecraft vs minecraft)
	modsDir := filepath.Join(instanceDir, ".minecraft", "mods")
	configDir := filepath.Join(instanceDir, ".minecraft", "config")

	if fi, err := os.Stat(filepath.Join(instanceDir, "minecraft", "mods")); err == nil && fi.IsDir() {
		modsDir = filepath.Join(instanceDir, "minecraft", "mods")
		configDir = filepath.Join(instanceDir, "minecraft", "config")
	} else if fi, err := os.Stat(filepath.Join(instanceDir, "minecraft")); err == nil && fi.IsDir() {
		modsDir = filepath.Join(instanceDir, "minecraft", "mods")
		configDir = filepath.Join(instanceDir, "minecraft", "config")
	}

	inst := &Instance{
		ID:               relID,
		Name:             name,
		Launcher:         LauncherPrism,
		LauncherName:     "Prism Launcher",
		MinecraftVersion: mcVer,
		Loader:           loader,
		LoaderVersion:    loaderVer,
		InstanceDir:      instanceDir,
		ModsDir:          modsDir,
		ConfigDir:        configDir,
		Icon:             cfgMap["iconKey"],
	}

	if lpStr, ok := cfgMap["lastPlayed"]; ok && lpStr != "" {
		if epochMs, err := strconv.ParseInt(lpStr, 10, 64); err == nil && epochMs > 0 {
			t := time.UnixMilli(epochMs)
			inst.LastPlayed = &t
		}
	}

	return inst, nil
}
