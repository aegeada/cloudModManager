package tui

import (
	"cmm/internal/mod"
	"cmm/internal/modrinth"
)

// TabID identifies one of the 4 main application tabs.
type TabID int

const (
	TabMods TabID = iota
	TabSearch
	TabConfig
	TabSync
)

func (t TabID) String() string {
	switch t {
	case TabMods:
		return "Installed Mods"
	case TabSearch:
		return "Modrinth Search"
	case TabConfig:
		return "Configuration"
	case TabSync:
		return "Sync & Server"
	default:
		return "Unknown"
	}
}

// ModalType specifies the active overlay dialog.
type ModalType int

const (
	ModalNone ModalType = iota
	ModalHelp
	ModalModDetails
	ModalDeleteConfirm
	ModalVersionPicker
)

// StatusMsg updates the footer status bar.
type StatusMsg struct {
	Message string
	IsError bool
}

// SwitchTabMsg requests navigation to a specific tab.
type SwitchTabMsg struct {
	Tab TabID
}

// ReloadModsMsg requests Tab 1 to reload installed mods from cmm.lock.
type ReloadModsMsg struct{}

// ModsReloadedMsg supplies the loaded mods list.
type ModsReloadedMsg struct {
	Mods []mod.ModStatus
	Err  error
}

// ModDetailsMsg requests opening the details modal for a mod.
type ModDetailsMsg struct {
	Mod *mod.ModStatus
}

// ModDeletedMsg notifies that a mod was deleted.
type ModDeletedMsg struct {
	Slug string
}

// ModPinnedMsg notifies that a mod pin status changed.
type ModPinnedMsg struct {
	Slug   string
	Pinned bool
}

// ModInstalledMsg notifies that a mod was successfully installed.
type ModInstalledMsg struct {
	Slug    string
	Version string
}

// VersionPickerMsg requests opening the version picker modal for a Modrinth search hit.
type VersionPickerMsg struct {
	Hit      *modrinth.SearchHit
	Versions []modrinth.Version
}
