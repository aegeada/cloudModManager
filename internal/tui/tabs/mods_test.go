package tabs

import (
	"path/filepath"
	"strings"
	"testing"

	"cmm/internal/config"
	"cmm/internal/mod"
	"cmm/internal/modrinth"
	"cmm/internal/tui/styles"
	"cmm/internal/tui/tea"
)

func setupTestModsTab(t *testing.T) (ModsModel, string, string) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")

	cfg := config.DefaultConfig()
	cfg.Profile.MinecraftVersion = "1.21.1"
	cfg.Profile.Loader = "fabric"
	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	lock := &config.Lockfile{Mods: []config.LockfileMod{}}
	lock.AddOrUpdateMod(config.LockfileMod{
		Slug:          "sodium",
		Name:          "Sodium",
		VersionNumber: "0.5.8",
		Side:          "client",
		Pinned:        true,
	})
	lock.AddOrUpdateMod(config.LockfileMod{
		Slug:          "lithium",
		Name:          "Lithium",
		VersionNumber: "0.12.1",
		Side:          "both",
		Pinned:        false,
	})
	if err := config.SaveLockfile(lockPath, lock); err != nil {
		t.Fatalf("SaveLockfile failed: %v", err)
	}

	client, _ := modrinth.NewClient("cmm-test/1.0.0")
	mgr := mod.NewManager(client, cfgPath, lockPath)
	model := NewModsModel(cfgPath, lockPath, mgr)

	return model, cfgPath, lockPath
}

func TestModsTab_ListAndBadges(t *testing.T) {
	m, _, _ := setupTestModsTab(t)

	// Simulate mods loading
	modsList := []mod.ModStatus{
		{Slug: "sodium", Name: "Sodium", Version: "0.5.8", Side: "client", Pinned: true},
		{Slug: "lithium", Name: "Lithium", Version: "0.12.1", Side: "both", Pinned: false, UpdateAvailable: true, LatestVersion: "0.12.2"},
	}

	m, _ = m.Update(ModsLoadedMsg{Mods: modsList})

	if len(m.FilteredMods) != 2 {
		t.Fatalf("Expected 2 filtered mods, got %d", len(m.FilteredMods))
	}

	view := styles.StripANSI(m.View())
	if !strings.Contains(view, "[PIN]") {
		t.Errorf("Expected [PIN] badge in view for sodium, got:\n%s", view)
	}
	if !strings.Contains(view, "[UPDATE]") {
		t.Errorf("Expected [UPDATE] badge in view for lithium, got:\n%s", view)
	}
	if !strings.Contains(view, "Sodium") || !strings.Contains(view, "Lithium") {
		t.Errorf("Expected mod names in view, got:\n%s", view)
	}
}

func TestModsTab_Navigation(t *testing.T) {
	m, _, _ := setupTestModsTab(t)
	m, _ = m.Update(ModsLoadedMsg{Mods: []mod.ModStatus{
		{Slug: "mod1", Name: "Mod One"},
		{Slug: "mod2", Name: "Mod Two"},
		{Slug: "mod3", Name: "Mod Three"},
	}})

	if m.Table.Cursor() != 0 {
		t.Errorf("Initial cursor should be 0, got %d", m.Table.Cursor())
	}

	// Move Down with 'j'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.Table.Cursor() != 1 {
		t.Errorf("Cursor after 'j' should be 1, got %d", m.Table.Cursor())
	}

	// Move Up with 'k'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.Table.Cursor() != 0 {
		t.Errorf("Cursor after 'k' should be 0, got %d", m.Table.Cursor())
	}
}

func TestModsTab_PinToggle(t *testing.T) {
	m, _, lockPath := setupTestModsTab(t)
	m, _ = m.Update(ModsLoadedMsg{Mods: []mod.ModStatus{
		{Slug: "lithium", Name: "Lithium", Version: "0.12.1", Pinned: false},
	}})

	// Press 'p'
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmd == nil {
		t.Fatalf("Expected cmd on 'p'")
	}

	msg := cmd()
	pinMsg, ok := msg.(PinToggledMsg)
	if !ok || pinMsg.Err != nil || !pinMsg.Pinned {
		t.Fatalf("Expected successful PinToggledMsg, got %+v", msg)
	}

	// Verify lockfile on disk
	lock, err := config.LoadLockfile(lockPath)
	if err != nil || !lock.IsPinned("lithium") {
		t.Errorf("Expected lithium to be pinned in lockfile")
	}
}

func TestModsTab_DeleteModal(t *testing.T) {
	m, _, _ := setupTestModsTab(t)
	m, _ = m.Update(ModsLoadedMsg{Mods: []mod.ModStatus{
		{Slug: "sodium", Name: "Sodium", Version: "0.5.8"},
	}})

	// Press 'd' to open delete modal
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if !m.DeleteModalVisible {
		t.Fatalf("Expected DeleteModalVisible to be true after 'd'")
	}

	view := styles.StripANSI(m.View())
	if !strings.Contains(view, "Delete Mod Confirmation") {
		t.Errorf("Expected delete confirmation modal in view, got:\n%s", view)
	}

	// Cancel with 'n'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.DeleteModalVisible {
		t.Errorf("Expected DeleteModalVisible to be false after 'n'")
	}
}

func TestModsTab_DetailsModal(t *testing.T) {
	m, _, _ := setupTestModsTab(t)
	m, _ = m.Update(ModsLoadedMsg{Mods: []mod.ModStatus{
		{Slug: "sodium", Name: "Sodium", Version: "0.5.8", Side: "client"},
	}})

	// Press Enter to open details modal
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.DetailsModalVisible {
		t.Fatalf("Expected DetailsModalVisible to be true after Enter")
	}

	view := styles.StripANSI(m.View())
	if !strings.Contains(view, "Mod Details") || !strings.Contains(view, "Sodium") {
		t.Errorf("Expected Mod Details in view, got:\n%s", view)
	}

	// Dismiss with Esc
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.DetailsModalVisible {
		t.Errorf("Expected DetailsModalVisible to be false after Esc")
	}
}

func TestModsTab_Filter(t *testing.T) {
	m, _, _ := setupTestModsTab(t)
	m, _ = m.Update(ModsLoadedMsg{Mods: []mod.ModStatus{
		{Slug: "sodium", Name: "Sodium", Version: "0.5.8"},
		{Slug: "lithium", Name: "Lithium", Version: "0.12.1"},
		{Slug: "iris", Name: "Iris Shaders", Version: "1.7.0"},
	}})

	// Press '/' to start filtering
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !m.IsFiltering {
		t.Fatalf("Expected IsFiltering to be true after '/'")
	}

	// Type 's','o','d'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	if len(m.FilteredMods) != 1 || m.FilteredMods[0].Slug != "sodium" {
		t.Errorf("Expected only 'sodium' after filter 'sod', got %v", m.FilteredMods)
	}

	// Clear filter with Esc
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.IsFiltering || len(m.FilteredMods) != 3 {
		t.Errorf("Expected filter to be cleared, got %d filtered mods", len(m.FilteredMods))
	}
}
