package sync

// SyncResult captures the outcome of a synchronization operation.
type SyncResult struct {
	AddedMods   []string
	UpdatedMods []string
	RemovedMods []string
	UnknownJars []string
	UpToDate    bool
	Message     string
}

// LocalSyncOptions contains options for local folder synchronization.
type LocalSyncOptions struct {
	Path string // Directory path override (falls back to config paths.mods_dir or "mods")
}

// ModrinthSyncOptions contains options for Modrinth modpack synchronization.
type ModrinthSyncOptions struct {
	Slug     string // Modrinth modpack project slug or ID
	FilePath string // Path to local .mrpack archive file
}

// GitHubSyncOptions contains options for GitHub repository synchronization.
type GitHubSyncOptions struct {
	Repo   string // GitHub repository in "owner/repo" format
	Branch string // Branch name (defaults to "main")
	Token  string // GitHub personal access token / bearer token
}

// RemoteSyncOptions contains options for remote cmm serve synchronization.
type RemoteSyncOptions struct {
	URL   string // URL of remote server (e.g. "http://127.0.0.1:8081")
	Token string // Bearer auth token for server authentication
}

// ModpackIndex represents the modrinth.index.json structure inside a .mrpack archive.
type ModpackIndex struct {
	FormatVersion int               `json:"formatVersion"`
	Game          string            `json:"game"`
	VersionID     string            `json:"versionId"`
	Name          string            `json:"name"`
	Summary       string            `json:"summary,omitempty"`
	Files         []ModpackFile     `json:"files"`
	Dependencies  map[string]string `json:"dependencies"`
}

// ModpackFile describes a single mod or resource file inside modrinth.index.json.
type ModpackFile struct {
	Path      string            `json:"path"`               // Relative path, e.g. "mods/sodium-fabric-0.5.8.jar"
	Hashes    map[string]string `json:"hashes"`             // Checksums, e.g. "sha512", "sha1"
	Env       *ModpackFileEnv   `json:"env,omitempty"`      // Side environment compatibility
	Downloads []string          `json:"downloads"`          // Direct download URLs
	FileSize  int64             `json:"fileSize,omitempty"` // File size in bytes
}

// ModpackFileEnv specifies client and server support requirements for a modpack file.
type ModpackFileEnv struct {
	Client string `json:"client,omitempty"` // "required", "optional", "unsupported"
	Server string `json:"server,omitempty"` // "required", "optional", "unsupported"
}

// TargetFile is an internal normalized descriptor for a desired mod file.
type TargetFile struct {
	FileName    string
	SHA512      string
	DownloadURL string
	Slug        string
	Name        string
	ProjectID   string
	VersionID   string
	Version     string
	Side        string
}
