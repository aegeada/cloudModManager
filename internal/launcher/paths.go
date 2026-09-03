package launcher

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// PathResolver resolves candidate directory paths for different Minecraft launchers.
type PathResolver struct {
	opts DetectorOptions
}

// NewPathResolver creates a new PathResolver with the provided options.
func NewPathResolver(opts DetectorOptions) *PathResolver {
	return &PathResolver{opts: opts}
}

// TargetOS returns the target operating system (either from opts.OS or runtime.GOOS).
func (r *PathResolver) TargetOS() string {
	if r.opts.OS != "" {
		return strings.ToLower(r.opts.OS)
	}
	return runtime.GOOS
}

// HomeDir returns the resolved user home directory.
func (r *PathResolver) HomeDir() string {
	if r.opts.HomeDir != "" {
		return r.opts.HomeDir
	}
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	if r.opts.UserProfile != "" {
		return r.opts.UserProfile
	}
	if u := os.Getenv("USERPROFILE"); u != "" {
		return u
	}
	if dir, err := os.UserHomeDir(); err == nil && dir != "" {
		return dir
	}
	return "."
}

// AppData returns the resolved Windows APPDATA directory.
func (r *PathResolver) AppData() string {
	if r.opts.AppData != "" {
		return r.opts.AppData
	}
	if a := os.Getenv("APPDATA"); a != "" {
		return a
	}
	return filepath.Join(r.HomeDir(), "AppData", "Roaming")
}

// LocalAppData returns the resolved Windows LOCALAPPDATA directory.
func (r *PathResolver) LocalAppData() string {
	if r.opts.LocalAppData != "" {
		return r.opts.LocalAppData
	}
	if a := os.Getenv("LOCALAPPDATA"); a != "" {
		return a
	}
	return filepath.Join(r.HomeDir(), "AppData", "Local")
}

// ResolveCandidatePaths returns all candidate root paths for a given launcher type.
func (r *PathResolver) ResolveCandidatePaths(launcherType LauncherType) []string {
	var candidates []string

	// 1. Check custom paths from options
	if r.opts.CustomPaths != nil {
		if paths, ok := r.opts.CustomPaths[launcherType]; ok && len(paths) > 0 {
			candidates = append(candidates, paths...)
		}
		if paths, ok := r.opts.CustomPaths[LauncherAll]; ok && len(paths) > 0 {
			candidates = append(candidates, paths...)
		}
	}

	home := r.HomeDir()
	appData := r.AppData()
	localAppData := r.LocalAppData()
	targetOS := r.TargetOS()

	switch launcherType {
	case LauncherVanilla:
		// Environment overrides
		if env := os.Getenv("CMM_MINECRAFT_DIR"); env != "" {
			candidates = append(candidates, env)
		}
		if env := os.Getenv("MINECRAFT_DIR"); env != "" {
			candidates = append(candidates, env)
		}

		switch targetOS {
		case "windows":
			candidates = append(candidates, filepath.Join(appData, ".minecraft"))
			candidates = append(candidates, filepath.Join(home, ".minecraft"))
		case "darwin":
			candidates = append(candidates, filepath.Join(home, "Library", "Application Support", "minecraft"))
			candidates = append(candidates, filepath.Join(home, ".minecraft"))
		default: // linux / other
			candidates = append(candidates, filepath.Join(home, ".minecraft"))
			candidates = append(candidates, filepath.Join(home, ".var", "app", "com.mojang.Minecraft", ".minecraft"))
			candidates = append(candidates, filepath.Join(home, ".local", "share", "minecraft"))
		}

	case LauncherPrism:
		// Environment overrides
		if env := os.Getenv("CMM_PRISM_DIR"); env != "" {
			candidates = append(candidates, env)
		}
		if env := os.Getenv("PRISMLAUNCHER_DIR"); env != "" {
			candidates = append(candidates, env)
		}
		if env := os.Getenv("CMM_MULTIMC_DIR"); env != "" {
			candidates = append(candidates, env)
		}
		if env := os.Getenv("MULTIMC_DIR"); env != "" {
			candidates = append(candidates, env)
		}

		switch targetOS {
		case "windows":
			candidates = append(candidates, filepath.Join(appData, "PrismLauncher"))
			candidates = append(candidates, filepath.Join(appData, "MultiMC"))
			candidates = append(candidates, filepath.Join(localAppData, "Programs", "PrismLauncher"))
			candidates = append(candidates, filepath.Join(home, "AppData", "Roaming", "PrismLauncher"))
		case "darwin":
			candidates = append(candidates, filepath.Join(home, "Library", "Application Support", "PrismLauncher"))
			candidates = append(candidates, filepath.Join(home, "Library", "Application Support", "MultiMC"))
			candidates = append(candidates, filepath.Join(home, "Library", "Application Support", "PolyMC"))
		default: // linux
			candidates = append(candidates, filepath.Join(home, ".local", "share", "PrismLauncher"))
			candidates = append(candidates, filepath.Join(home, ".local", "share", "multimc"))
			candidates = append(candidates, filepath.Join(home, ".local", "share", "PolyMC"))
			candidates = append(candidates, filepath.Join(home, ".var", "app", "org.prismlauncher.PrismLauncher", "data", "PrismLauncher"))
			candidates = append(candidates, filepath.Join(home, ".var", "app", "org.multimc.MultiMC", "data", "MultiMC"))
		}

	case LauncherModrinth:
		// Environment overrides
		if env := os.Getenv("CMM_MODRINTH_DIR"); env != "" {
			candidates = append(candidates, env)
		}
		if env := os.Getenv("MODRINTH_APP_DIR"); env != "" {
			candidates = append(candidates, env)
		}

		switch targetOS {
		case "windows":
			candidates = append(candidates, filepath.Join(appData, "ModrinthApp"))
			candidates = append(candidates, filepath.Join(appData, "com.modrinth.theseus"))
			candidates = append(candidates, filepath.Join(localAppData, "ModrinthApp"))
			candidates = append(candidates, filepath.Join(home, "AppData", "Roaming", "ModrinthApp"))
		case "darwin":
			candidates = append(candidates, filepath.Join(home, "Library", "Application Support", "ModrinthApp"))
			candidates = append(candidates, filepath.Join(home, "Library", "Application Support", "com.modrinth.theseus"))
		default: // linux
			candidates = append(candidates, filepath.Join(home, ".config", "ModrinthApp"))
			candidates = append(candidates, filepath.Join(home, ".config", "com.modrinth.theseus"))
			candidates = append(candidates, filepath.Join(home, ".local", "share", "ModrinthApp"))
			candidates = append(candidates, filepath.Join(home, ".var", "app", "com.modrinth.ModrinthApp", "config", "ModrinthApp"))
			candidates = append(candidates, filepath.Join(home, ".var", "app", "com.modrinth.ModrinthApp", "data", "ModrinthApp"))
		}

	case LauncherCurseForge:
		// Environment overrides
		if env := os.Getenv("CMM_CURSEFORGE_DIR"); env != "" {
			candidates = append(candidates, env)
		}
		if env := os.Getenv("CURSEFORGE_DIR"); env != "" {
			candidates = append(candidates, env)
		}

		switch targetOS {
		case "windows":
			candidates = append(candidates, filepath.Join(home, "curseforge", "minecraft", "Instances"))
			candidates = append(candidates, filepath.Join(home, "curseforge", "minecraft"))
			candidates = append(candidates, filepath.Join(home, "Documents", "curseforge", "minecraft", "Instances"))
			candidates = append(candidates, filepath.Join(appData, "curseforge", "minecraft", "Instances"))
		case "darwin":
			candidates = append(candidates, filepath.Join(home, "Documents", "curseforge", "minecraft", "Instances"))
			candidates = append(candidates, filepath.Join(home, "curseforge", "minecraft", "Instances"))
			candidates = append(candidates, filepath.Join(home, "Library", "Application Support", "curseforge", "minecraft", "Instances"))
		default: // linux
			candidates = append(candidates, filepath.Join(home, "curseforge", "minecraft", "Instances"))
			candidates = append(candidates, filepath.Join(home, "curseforge", "minecraft"))
			candidates = append(candidates, filepath.Join(home, ".curseforge", "minecraft", "Instances"))
			candidates = append(candidates, filepath.Join(home, ".config", "curseforge", "minecraft", "Instances"))
			candidates = append(candidates, filepath.Join(home, "Documents", "curseforge", "minecraft", "Instances"))
		}
	}

	// Deduplicate candidates while preserving order
	seen := make(map[string]bool)
	var deduped []string
	for _, p := range candidates {
		cleaned := filepath.Clean(p)
		if !seen[cleaned] {
			seen[cleaned] = true
			deduped = append(deduped, cleaned)
		}
	}

	return deduped
}
