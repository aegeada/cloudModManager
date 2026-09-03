package loader

import (
	"fmt"
	"strings"

	"cmm/internal/config"
)

var SupportedLoaders = []string{"fabric", "forge", "neoforge", "quilt"}

// ListLoaders returns available loader versions or supported loaders.
func ListLoaders(loaderName string) ([]string, error) {
	loaderName = strings.ToLower(strings.TrimSpace(loaderName))

	if loaderName == "" || loaderName == "all" {
		return SupportedLoaders, nil
	}

	switch loaderName {
	case "fabric":
		versions, err := GetFabricLoaderVersions()
		if err != nil {
			return nil, err
		}
		var result []string
		for _, v := range versions {
			tag := "beta"
			if v.Stable {
				tag = "stable"
			}
			result = append(result, fmt.Sprintf("%s (%s)", v.Version, tag))
		}
		return result, nil
	case "forge", "neoforge", "quilt":
		return []string{
			fmt.Sprintf("%s (recommended)", loaderName),
			fmt.Sprintf("%s (latest)", loaderName),
		}, nil
	default:
		return nil, fmt.Errorf("unknown loader '%s'. Supported loaders: %s", loaderName, strings.Join(SupportedLoaders, ", "))
	}
}

// InstallLoader sets and validates the loader and version in cmm.toml.
func InstallLoader(configPath string, loaderName string, version string) error {
	loaderName = strings.ToLower(strings.TrimSpace(loaderName))
	version = strings.TrimSpace(version)

	isSupported := false
	for _, l := range SupportedLoaders {
		if l == loaderName {
			isSupported = true
			break
		}
	}
	if !isSupported {
		return fmt.Errorf("unsupported loader '%s'. Supported: %s", loaderName, strings.Join(SupportedLoaders, ", "))
	}

	if loaderName == "fabric" {
		if version != "" {
			valid, err := ValidateFabricVersion(version)
			if err != nil || !valid {
				return fmt.Errorf("unsupported or invalid fabric loader version '%s'", version)
			}
		} else {
			versions, err := GetFabricLoaderVersions()
			if err != nil {
				return err
			}
			for _, v := range versions {
				if v.Stable {
					version = v.Version
					break
				}
			}
			if version == "" && len(versions) > 0 {
				version = versions[0].Version
			}
		}
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	cfg.Profile.Loader = loaderName
	cfg.Profile.LoaderVersion = version

	if err := config.SaveConfig(configPath, cfg); err != nil {
		return fmt.Errorf("failed to save config to %s: %w", configPath, err)
	}

	return nil
}

// CheckLatestLoaderVersion checks if there is a newer stable version for the given loader.
func CheckLatestLoaderVersion(loaderName string, currentVersion string) (latestVer string, updateAvailable bool, err error) {
	loaderName = strings.ToLower(strings.TrimSpace(loaderName))
	currentVersion = strings.TrimSpace(strings.TrimPrefix(currentVersion, "v"))

	switch loaderName {
	case "fabric":
		versions, err := GetFabricLoaderVersions()
		if err != nil || len(versions) == 0 {
			return "", false, err
		}
		for _, v := range versions {
			if v.Stable {
				latestVer = strings.TrimPrefix(v.Version, "v")
				break
			}
		}
		if latestVer == "" && len(versions) > 0 {
			latestVer = strings.TrimPrefix(versions[0].Version, "v")
		}
		if latestVer != "" && (currentVersion == "" || currentVersion != latestVer) {
			return latestVer, true, nil
		}
		return latestVer, false, nil
	default:
		return "", false, nil
	}
}

