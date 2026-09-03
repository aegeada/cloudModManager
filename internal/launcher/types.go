package launcher

import (
	"strings"
	"time"
)

// LauncherType defines the type of Minecraft launcher.
type LauncherType string

const (
	LauncherVanilla    LauncherType = "vanilla"
	LauncherPrism      LauncherType = "prism"
	LauncherModrinth   LauncherType = "modrinth"
	LauncherCurseForge LauncherType = "curseforge"
	LauncherAll        LauncherType = "all"
)

// NormalizeLauncherType converts a raw string to a canonical LauncherType.
func NormalizeLauncherType(raw string) LauncherType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "vanilla", "official", "mojang", "minecraft":
		return LauncherVanilla
	case "prism", "prismlauncher", "multimc", "polymc":
		return LauncherPrism
	case "modrinth", "modrinthapp", "theseus":
		return LauncherModrinth
	case "curseforge", "curse", "cf":
		return LauncherCurseForge
	case "all", "":
		return LauncherAll
	default:
		return LauncherType(strings.ToLower(strings.TrimSpace(raw)))
	}
}

// Instance represents a detected Minecraft launcher instance or profile.
type Instance struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Launcher         LauncherType `json:"launcher"`
	LauncherName     string     `json:"launcher_name"`
	MinecraftVersion string     `json:"minecraft_version"`
	Loader           string     `json:"loader"`
	LoaderVersion    string     `json:"loader_version,omitempty"`
	InstanceDir      string     `json:"instance_dir"`
	ModsDir          string     `json:"mods_dir"`
	ConfigDir        string     `json:"config_dir"`
	Icon             string     `json:"icon,omitempty"`
	LastPlayed       *time.Time `json:"last_played,omitempty"`
}

// DetectorOptions configures the instance detection engine.
type DetectorOptions struct {
	HomeDir      string                    // User home directory override
	AppData      string                    // Windows %APPDATA% override
	LocalAppData string                    // Windows %LOCALAPPDATA% override
	UserProfile  string                    // Windows %USERPROFILE% override
	OS           string                    // OS override ("windows", "darwin", "linux")
	CustomPaths  map[LauncherType][]string // Explicit directories to probe per launcher
}

// LauncherSyncOptions contains options for synchronizing a modpack to a launcher instance.
type LauncherSyncOptions struct {
	InstanceName string       // Target instance name or ID
	LauncherType LauncherType // Optional launcher filter
	ConfigPath   string       // Path to local cmm.toml (default "cmm.toml")
	LockPath     string       // Path to local cmm.lock (default "cmm.lock")
	Force        bool         // Force sync even if MC version or loader mismatches
	DryRun       bool         // Simulate sync operations without writing to disk
}

// LauncherSyncResult captures the outcome of an instance synchronization.
type LauncherSyncResult struct {
	Instance    Instance `json:"instance"`
	AddedMods   []string `json:"added_mods"`
	UpdatedMods []string `json:"updated_mods"`
	RemovedMods []string `json:"removed_mods"`
	SkippedMods []string `json:"skipped_mods"` // Server-only mods skipped
	UpToDate    bool     `json:"up_to_date"`
	Message     string   `json:"message"`
}
