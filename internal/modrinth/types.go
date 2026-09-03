package modrinth

import "time"

// Project represents a Modrinth project.
type Project struct {
	ID               string    `json:"id"`
	Slug             string    `json:"slug"`
	ProjectType      string    `json:"project_type"`
	Team             string    `json:"team"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	Body             string    `json:"body"`
	Published        time.Time `json:"published"`
	Updated          time.Time `json:"updated"`
	Approved         time.Time `json:"approved"`
	Status           string    `json:"status"`
	ClientSide       string    `json:"client_side"`
	ServerSide       string    `json:"server_side"`
	Downloads        int       `json:"downloads"`
	Followers        int       `json:"followers"`
	Categories       []string  `json:"categories"`
	AdditionalCats   []string  `json:"additional_categories"`
	GameVersions     []string  `json:"game_versions"`
	Loaders          []string  `json:"loaders"`
	Versions         []string  `json:"versions"`
	IconURL          string    `json:"icon_url"`
	IssuesURL        string    `json:"issues_url"`
	SourceURL        string    `json:"source_url"`
	WikiURL          string    `json:"wiki_url"`
	DiscordURL       string    `json:"discord_url"`
}

// SearchHit represents a single search result from Modrinth.
type SearchHit struct {
	ProjectID      string   `json:"project_id"`
	ProjectType    string   `json:"project_type"`
	Slug           string   `json:"slug"`
	Author         string   `json:"author"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Categories     []string `json:"categories"`
	DisplayCats    []string `json:"display_categories"`
	Versions       []string `json:"versions"`
	Downloads      int      `json:"downloads"`
	Follows        int      `json:"follows"`
	IconURL        string   `json:"icon_url"`
	DateCreated    string   `json:"date_created"`
	DateModified   string   `json:"date_modified"`
	LatestVersion  string   `json:"latest_version"`
	ClientSide     string   `json:"client_side"`
	ServerSide     string   `json:"server_side"`
	Gallery        []string `json:"gallery"`
	FeaturedCats   []string `json:"featured_categories"`
}

// SearchResponse represents the response from Modrinth search endpoint.
type SearchResponse struct {
	Hits      []SearchHit `json:"hits"`
	Offset    int         `json:"offset"`
	Limit     int         `json:"limit"`
	TotalHits int         `json:"total_hits"`
}

// Dependency represents a version dependency.
type Dependency struct {
	VersionID      *string `json:"version_id"`
	ProjectID      *string `json:"project_id"`
	FileName       *string `json:"file_name"`
	DependencyType string  `json:"dependency_type"` // "required", "optional", "incompatible", "embedded"
}

// VersionFile represents a file in a version.
type VersionFile struct {
	Hashes   map[string]string `json:"hashes"` // "sha1", "sha512"
	URL      string            `json:"url"`
	Filename string            `json:"filename"`
	Primary  bool              `json:"primary"`
	Size     int64             `json:"size"`
	FileType *string           `json:"file_type"`
}

// Version represents a specific release of a Modrinth project.
type Version struct {
	ID              string        `json:"id"`
	ProjectID       string        `json:"project_id"`
	AuthorID        string        `json:"author_id"`
	Featured        bool          `json:"featured"`
	Name            string        `json:"name"`
	VersionNumber   string        `json:"version_number"`
	Changelog       string        `json:"changelog"`
	Dependencies    []Dependency  `json:"dependencies"`
	GameVersions    []string      `json:"game_versions"`
	VersionType     string        `json:"version_type"` // "release", "beta", "alpha"
	Loaders         []string      `json:"loaders"`
	Files           []VersionFile `json:"files"`
	DatePublished   time.Time     `json:"date_published"`
	Downloads       int           `json:"downloads"`
	Status          string        `json:"status"`
	RequestedStatus string        `json:"requested_status"`
}

// LoaderTag represents a loader tag from /v2/tag/loader.
type LoaderTag struct {
	Icon                  string   `json:"icon"`
	Name                  string   `json:"name"`
	SupportedProjectTypes []string `json:"supported_project_types"`
}

// GameVersionTag represents a game version tag from /v2/tag/game_version.
type GameVersionTag struct {
	Version     string    `json:"version"`
	VersionType string    `json:"version_type"`
	Date        time.Time `json:"date"`
	Major       bool      `json:"major"`
}
