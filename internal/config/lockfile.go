package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

type Lockfile struct {
	Mods []LockfileMod `toml:"mods"`
}

type LockfileMod struct {
	Slug          string `toml:"slug,omitempty" json:"slug,omitempty"`
	Name          string `toml:"name,omitempty" json:"name,omitempty"`
	Source        string `toml:"source,omitempty" json:"source,omitempty"`
	ProjectID     string `toml:"project_id,omitempty" json:"project_id,omitempty"`
	VersionID     string `toml:"version_id,omitempty" json:"version_id,omitempty"`
	VersionNumber string `toml:"version_number,omitempty" json:"version_number,omitempty"`
	Version       string `toml:"version,omitempty" json:"version,omitempty"`
	FileName      string `toml:"file_name,omitempty" json:"file_name,omitempty"`
	SHA512        string `toml:"sha512,omitempty" json:"sha512,omitempty"`
	DownloadURL   string `toml:"download_url,omitempty" json:"download_url,omitempty"`
	ClientSide    string `toml:"client_side,omitempty" json:"client_side,omitempty"`
	ServerSide    string `toml:"server_side,omitempty" json:"server_side,omitempty"`
	Side          string `toml:"side,omitempty" json:"side,omitempty"`
	Pinned        bool   `toml:"pinned" json:"pinned"`
	Disabled      bool   `toml:"disabled,omitempty" json:"disabled,omitempty"`
}

// GetVersion returns VersionNumber if present, else Version.
func (m *LockfileMod) GetVersion() string {
	if m.VersionNumber != "" {
		return m.VersionNumber
	}
	return m.Version
}

var (
	mcRegex  = regexp.MustCompile(`[-_+](mc[-_]?)?1\.[0-9]+(\.[0-9]+)?`)
	verRegex = regexp.MustCompile(`[-_+]((v|ver|build|rev|r)[0-9]+.*|[0-9]+(\.[0-9]+)+([+_.-][a-zA-Z0-9]+)*)$`)
)

// ExtractModSlug normalizes a filename, path, or raw slug into a canonical mod slug.
// E.g., "sodium-fabric-0.5.8.jar" -> "sodium", "lithium-0.11.0" -> "lithium", "fabric-api-0.92.0+1.21.jar.disabled" -> "fabric-api",
// "forge-config-api-port+forge-1.20.4-1.0.0.disabled" -> "forge-config-api-port".
func ExtractModSlug(raw string) string {
	name := filepath.Base(raw)
	name = strings.ToLower(strings.TrimSpace(name))
	for strings.HasSuffix(name, ".disabled") {
		name = strings.TrimSuffix(name, ".disabled")
	}
	for _, ext := range []string{".jar", ".zip", ".mrpack"} {
		if strings.HasSuffix(name, ext) {
			name = strings.TrimSuffix(name, ext)
			break
		}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	// 1. Strip mod loader tags (e.g. -fabric, +forge, _neoforge)
	loaderPatterns := []string{
		"-fabric", "-forge", "-quilt", "-neoforge",
		"+fabric", "+forge", "+quilt", "+neoforge",
		"_fabric", "_forge", "_quilt", "_neoforge",
	}
	for _, lp := range loaderPatterns {
		name = strings.ReplaceAll(name, lp, "")
	}

	// 2. Strip Minecraft version tags (e.g. -mc1.21.1, -1.21.1, +1.21.1, +mc1.21)
	name = mcRegex.ReplaceAllString(name, "")

	// 3. Strip semantic / numerical version suffixes (e.g. -0.5.8, -v1.0.0, -15.0.127, -19.0.0.12, +build.1)
	name = verRegex.ReplaceAllString(name, "")

	return strings.Trim(name, "-_+.")
}

// matchModExact checks if a LockfileMod matches a given slug, project ID, name, or filename exactly (case-insensitive).
func matchModExact(mod *LockfileMod, slugOrID string) bool {
	if mod == nil || slugOrID == "" {
		return false
	}
	// 1. Direct case-insensitive matches for Slug, ProjectID, Name, FileName
	if mod.Slug != "" && strings.EqualFold(mod.Slug, slugOrID) {
		return true
	}
	if mod.ProjectID != "" && strings.EqualFold(mod.ProjectID, slugOrID) {
		return true
	}
	if mod.Name != "" && strings.EqualFold(mod.Name, slugOrID) {
		return true
	}
	if mod.FileName != "" {
		if strings.EqualFold(mod.FileName, slugOrID) {
			return true
		}
		modFileNoDisabled := strings.TrimSuffix(mod.FileName, ".disabled")
		slugNoDisabled := strings.TrimSuffix(slugOrID, ".disabled")
		if modFileNoDisabled != "" && (strings.EqualFold(modFileNoDisabled, slugOrID) || strings.EqualFold(modFileNoDisabled, slugNoDisabled)) {
			return true
		}
		modFileBase := strings.TrimSuffix(modFileNoDisabled, ".jar")
		slugBase := strings.TrimSuffix(slugNoDisabled, ".jar")
		if modFileBase != "" && (strings.EqualFold(modFileBase, slugBase) || strings.EqualFold(modFileBase, slugOrID) || strings.EqualFold(mod.FileName, slugBase)) {
			return true
		}
	}
	return false
}

// matchModNormalized checks if a LockfileMod matches a given query using normalized slug extraction.
func matchModNormalized(mod *LockfileMod, slugOrID string) bool {
	if mod == nil || slugOrID == "" {
		return false
	}
	extracted := ExtractModSlug(slugOrID)
	if extracted == "" {
		return false
	}

	if mod.Slug != "" && strings.EqualFold(mod.Slug, extracted) {
		return true
	}
	if mod.Name != "" && strings.EqualFold(mod.Name, extracted) {
		return true
	}
	if mod.FileName != "" {
		modExtracted := ExtractModSlug(mod.FileName)
		if modExtracted != "" {
			if strings.EqualFold(modExtracted, extracted) {
				if mod.Slug == "" || strings.EqualFold(mod.Slug, extracted) || strings.EqualFold(mod.Slug, modExtracted) {
					return true
				}
			}
			if strings.EqualFold(modExtracted, slugOrID) {
				if mod.Slug == "" || strings.EqualFold(mod.Slug, slugOrID) || strings.EqualFold(mod.Slug, modExtracted) {
					return true
				}
			}
		}
	}
	return false
}

// matchMod checks if a LockfileMod matches a given slug, project ID, name, or filename (case-insensitive).
func matchMod(mod *LockfileMod, slugOrID string) bool {
	return matchModExact(mod, slugOrID) || matchModNormalized(mod, slugOrID)
}

// ParseLockfile unmarshals raw TOML bytes supporting both array-of-tables ([[mods]]) and map-of-tables ([mods.<id>]).
func ParseLockfile(data []byte) (*Lockfile, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "[mods]" {
		return &Lockfile{Mods: []LockfileMod{}}, nil
	}

	// 1. Try decoding array of tables ([[mods]])
	var arrLock struct {
		Mods []LockfileMod `toml:"mods"`
	}
	if _, err := toml.Decode(string(data), &arrLock); err == nil && len(arrLock.Mods) > 0 {
		return &Lockfile{Mods: arrLock.Mods}, nil
	}

	// 2. Try decoding map of tables ([mods.<id>])
	var mapLock struct {
		Mods map[string]LockfileMod `toml:"mods"`
	}
	if _, err := toml.Decode(string(data), &mapLock); err == nil && len(mapLock.Mods) > 0 {
		mods := make([]LockfileMod, 0, len(mapLock.Mods))
		for key, m := range mapLock.Mods {
			if m.Slug == "" {
				m.Slug = key
			}
			if m.Name == "" {
				m.Name = key
			}
			mods = append(mods, m)
		}
		return &Lockfile{Mods: mods}, nil
	}

	// 3. If array decode succeeded without error but 0 mods (e.g. empty table structure)
	var generic map[string]interface{}
	if _, err := toml.Decode(string(data), &generic); err == nil {
		return &Lockfile{Mods: []LockfileMod{}}, nil
	}

	return nil, fmt.Errorf("failed to parse lockfile TOML")
}

// DecodeLockfile decodes a lockfile from a string.
func DecodeLockfile(content string) (*Lockfile, error) {
	return ParseLockfile([]byte(content))
}

// LoadLockfile loads the lockfile from a file, supporting both array and map TOML formats.
func LoadLockfile(path string) (*Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseLockfile(data)
}

// SaveLockfile writes the lockfile to the specified path.
func SaveLockfile(path string, lock *Lockfile) error {
	if lock == nil {
		lock = &Lockfile{Mods: []LockfileMod{}}
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(lock)
}

// Save is a convenience method on Lockfile.
func (l *Lockfile) Save(path string) error {
	return SaveLockfile(path, l)
}

// findModIndexExact searches for a mod index using exact matching only.
func (l *Lockfile) findModIndexExact(slugOrID string) int {
	if l == nil || slugOrID == "" {
		return -1
	}
	for i := range l.Mods {
		if matchModExact(&l.Mods[i], slugOrID) {
			return i
		}
	}
	return -1
}

func (l *Lockfile) findModIndex(slugOrID string) int {
	if l == nil || slugOrID == "" {
		return -1
	}
	// Pass 1: Exact case-insensitive matches for Slug, ProjectID, Name, FileName, FileName-.disabled, FileName-.jar across ALL mods
	for i := range l.Mods {
		if matchModExact(&l.Mods[i], slugOrID) {
			return i
		}
	}
	// Pass 2: Normalized slug fallback ONLY if Pass 1 matched zero mods
	for i := range l.Mods {
		if matchModNormalized(&l.Mods[i], slugOrID) {
			return i
		}
	}
	return -1
}

// GetMod finds a mod by slug, project ID, name, or filename in the lockfile using a two-pass lookup.
func (l *Lockfile) GetMod(slugOrID string) *LockfileMod {
	if l == nil {
		return nil
	}
	idx := l.findModIndex(slugOrID)
	if idx >= 0 {
		return &l.Mods[idx]
	}
	return nil
}

// AddOrUpdateMod adds a mod to the lockfile or updates it if it exists.
// It strictly searches for existing mods using exact matching only (never normalized fuzzy fallback),
// ensuring new numbered mods (e.g. chain-mod-01) never overwrite existing mods (e.g. chain-mod-00).
func (l *Lockfile) AddOrUpdateMod(mod LockfileMod) {
	if l == nil {
		return
	}
	var idx int = -1
	if mod.Slug != "" {
		idx = l.findModIndexExact(mod.Slug)
	}
	if idx < 0 && mod.ProjectID != "" {
		idx = l.findModIndexExact(mod.ProjectID)
	}
	if idx < 0 && mod.Name != "" {
		idx = l.findModIndexExact(mod.Name)
	}
	if idx < 0 && mod.FileName != "" {
		idx = l.findModIndexExact(mod.FileName)
	}

	if idx >= 0 {
		l.Mods[idx] = mod
		return
	}
	l.Mods = append(l.Mods, mod)
}

// RemoveMod removes a mod from the lockfile by slug, project ID, name, or filename.
func (l *Lockfile) RemoveMod(slugOrID string) bool {
	if l == nil {
		return false
	}
	idx := l.findModIndex(slugOrID)
	if idx >= 0 {
		l.Mods = append(l.Mods[:idx], l.Mods[idx+1:]...)
		return true
	}
	return false
}

// IsPinned returns true if the mod is pinned in the lockfile.
func (l *Lockfile) IsPinned(slugOrID string) bool {
	if l == nil {
		return false
	}
	mod := l.GetMod(slugOrID)
	return mod != nil && mod.Pinned
}

// SetPinned sets the pinned status of a mod in the lockfile.
func (l *Lockfile) SetPinned(slugOrID string, pinned bool) bool {
	if l == nil {
		return false
	}
	mod := l.GetMod(slugOrID)
	if mod != nil {
		mod.Pinned = pinned
		return true
	}
	return false
}

// IsDisabled returns true if the mod is disabled in the lockfile.
func (l *Lockfile) IsDisabled(slugOrID string) bool {
	if l == nil {
		return false
	}
	mod := l.GetMod(slugOrID)
	return mod != nil && mod.Disabled
}

// SetDisabled sets the disabled status of a mod in the lockfile.
func (l *Lockfile) SetDisabled(slugOrID string, disabled bool) bool {
	if l == nil {
		return false
	}
	mod := l.GetMod(slugOrID)
	if mod != nil {
		mod.Disabled = disabled
		return true
	}
	return false
}

// FormatDetailedSide evaluates client_side and server_side metadata into granular descriptive tags:
// - client: required, server: required -> "Client & Server"
// - client: optional, server: required -> "Server (Client Opt.)"
// - client: required, server: optional -> "Client (Server Opt.)"
// - client: optional, server: optional -> "Client & Server (Opt.)"
// - client: required, server: unsupported -> "Client Only"
// - client: optional, server: unsupported -> "Client Only (Opt.)"
// - client: unsupported, server: required -> "Server Only"
// - client: unsupported, server: optional -> "Server Only (Opt.)"
func FormatDetailedSide(clientSide, serverSide string) string {
	cs := strings.ToLower(strings.TrimSpace(clientSide))
	ss := strings.ToLower(strings.TrimSpace(serverSide))

	if cs == "required" && ss == "required" {
		return "Client & Server"
	}
	if cs == "optional" && ss == "required" {
		return "Server (Client Opt.)"
	}
	if cs == "required" && ss == "optional" {
		return "Client (Server Opt.)"
	}
	if cs == "optional" && ss == "optional" {
		return "Client & Server (Opt.)"
	}
	if cs == "required" && (ss == "unsupported" || ss == "") {
		return "Client Only"
	}
	if cs == "optional" && (ss == "unsupported" || ss == "") {
		return "Client Only (Opt.)"
	}
	if (cs == "unsupported" || cs == "") && ss == "required" {
		return "Server Only"
	}
	if (cs == "unsupported" || cs == "") && ss == "optional" {
		return "Server Only (Opt.)"
	}
	if cs == "unsupported" && ss == "unsupported" {
		return "Unsupported"
	}

	// Fallback mappings
	if ss == "required" {
		return "Server Only"
	}
	if ss == "optional" {
		return "Server Only (Opt.)"
	}
	if cs == "required" {
		return "Client Only"
	}
	if cs == "optional" {
		return "Client Only (Opt.)"
	}
	return "Client & Server"
}

// DetermineSide evaluates client_side and server_side requirements from Modrinth metadata.
func DetermineSide(clientSide, serverSide string) string {
	return FormatDetailedSide(clientSide, serverSide)
}

// NormalizeSide cleans legacy or raw values ("required", "optional", "both", etc.) into granular descriptive tags.
func NormalizeSide(side string) string {
	s := strings.ToLower(strings.TrimSpace(side))
	switch s {
	case "server", "server-only", "server only":
		return "Server Only"
	case "server only (opt.)", "server only (opt)", "server (opt.)", "server (opt)":
		return "Server Only (Opt.)"
	case "client", "client-only", "client only":
		return "Client Only"
	case "client only (opt.)", "client only (opt)", "client (opt.)", "client (opt)":
		return "Client Only (Opt.)"
	case "server (client opt.)", "server (client opt)":
		return "Server (Client Opt.)"
	case "client (server opt.)", "client (server opt)":
		return "Client (Server Opt.)"
	case "client & server (opt.)", "client & server (opt)", "both (opt)", "both (opt.)":
		return "Client & Server (Opt.)"
	case "both", "client & server", "required", "":
		return "Client & Server"
	case "optional":
		return "Client & Server (Opt.)"
	default:
		return side
	}
}


