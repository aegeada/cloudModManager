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

func setupTestSearchTab(t *testing.T) SearchModel {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")

	cfg := config.DefaultConfig()
	cfg.Profile.MinecraftVersion = "1.21.1"
	cfg.Profile.Loader = "fabric"
	_ = config.SaveConfig(cfgPath, cfg)
	_ = config.SaveLockfile(lockPath, &config.Lockfile{Mods: []config.LockfileMod{}})

	client, _ := modrinth.NewClient("cmm-test/1.0.0")
	mgr := mod.NewManager(client, cfgPath, lockPath)
	return NewSearchModel(client, mgr, cfg)
}

func TestSearchTab_TypingAndDebounce(t *testing.T) {
	m := setupTestSearchTab(t)

	// Type 's','o','d'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	if m.SearchInput.Value() != "sod" {
		t.Errorf("Expected search input 'sod', got '%s'", m.SearchInput.Value())
	}
	if m.DebounceSeq != 3 {
		t.Errorf("Expected DebounceSeq 3, got %d", m.DebounceSeq)
	}

	// Dispatch matching SearchDebounceMsg
	m, cmd := m.Update(SearchDebounceMsg{Query: "sod", Seq: 3})
	if !m.IsSearching {
		t.Errorf("Expected IsSearching to be true")
	}
	if cmd == nil {
		t.Fatalf("Expected batch cmd for search execution and spinner")
	}
}

func TestSearchTab_SearchResults(t *testing.T) {
	m := setupTestSearchTab(t)
	m.DebounceSeq = 1

	hits := []modrinth.SearchHit{
		{
			Title:       "Sodium",
			Slug:        "sodium",
			Author:      "jellysquid",
			Downloads:   25000000,
			Categories:  []string{"fabric", "optimization"},
			Description: "Modern rendering engine for Minecraft",
		},
	}

	m, _ = m.Update(SearchResultsMsg{
		Query:     "sodium",
		Seq:       1,
		Hits:      hits,
		TotalHits: 1,
	})

	if len(m.Hits) != 1 {
		t.Fatalf("Expected 1 hit, got %d", len(m.Hits))
	}
	if m.IsSearching {
		t.Errorf("Expected IsSearching to be false after results")
	}

	view := styles.StripANSI(m.View())
	if !strings.Contains(view, "Sodium") || !strings.Contains(view, "jellysquid") {
		t.Errorf("Expected search results in view, got:\n%s", view)
	}
}

func TestSearchTab_VersionModal(t *testing.T) {
	m := setupTestSearchTab(t)
	hit := &modrinth.SearchHit{
		Title: "Sodium",
		Slug:  "sodium",
	}
	vers := []modrinth.Version{
		{
			VersionNumber: "0.5.8",
			Name:          "Sodium 0.5.8 for 1.21.1",
			GameVersions:  []string{"1.21.1"},
			Loaders:       []string{"fabric"},
		},
	}

	// Deliver versions loaded
	m, _ = m.Update(VersionsLoadedMsg{Hit: hit, Versions: vers})

	if !m.VersionModalVisible {
		t.Fatalf("Expected VersionModalVisible to be true")
	}

	view := styles.StripANSI(m.View())
	if !strings.Contains(view, "Version Picker") || !strings.Contains(view, "0.5.8") {
		t.Errorf("Expected Version Picker in view, got:\n%s", view)
	}

	// Dismiss with Esc
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.VersionModalVisible {
		t.Errorf("Expected VersionModalVisible to be false after Esc")
	}
}
