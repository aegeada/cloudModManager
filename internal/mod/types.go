package mod

import "cmm/internal/config"
import "cmm/internal/modrinth"

type AddResult struct {
	InstalledMod      *config.LockfileMod
	InstalledDeps     []*config.LockfileMod
	OptionalDeps      []string
	SkippedClientOnly bool
	AlreadyInstalled  bool
	Replaced          bool
}

type RemoveResult struct {
	RemovedMod   string
	RemovedFiles []string
	OrphanedDeps []string
	DryRun       bool
}

type DisableOptions struct {
	Force  bool
	DryRun bool
}

type EnableOptions struct {
	DryRun bool
}

type DisableResult struct {
	Slug                 string   `json:"slug"`
	Name                 string   `json:"name"`
	OldFileName          string   `json:"old_file_name"`
	NewFileName          string   `json:"new_file_name"`
	AlreadyDisabled      bool     `json:"already_disabled"`
	DryRun               bool     `json:"dry_run"`
	HasDependentsWarning bool     `json:"has_dependents_warning"`
	ActiveDependents     []string `json:"active_dependents,omitempty"`
}

type EnableResult struct {
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	OldFileName    string `json:"old_file_name"`
	NewFileName    string `json:"new_file_name"`
	AlreadyEnabled bool   `json:"already_enabled"`
	DryRun         bool   `json:"dry_run"`
}

type ModStatus struct {
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	Version         string `json:"version"`
	Side            string `json:"side"`
	Pinned          bool   `json:"pinned"`
	Disabled        bool   `json:"disabled"`
	UpdateAvailable bool   `json:"update_available"`
	LatestVersion   string `json:"latest_version,omitempty"`
}

type UpdateCandidate struct {
	Mod           config.LockfileMod
	TargetVersion modrinth.Version
	Changelog     string
}
