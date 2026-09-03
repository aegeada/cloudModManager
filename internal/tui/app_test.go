package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"cmm/internal/config"
	"cmm/internal/tui/styles"
	"cmm/internal/tui/tea"
)

func setupTestApp(t *testing.T) (AppModel, string) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")

	cfg := config.DefaultConfig()
	cfg.Profile.Name = "TestPack"
	cfg.Profile.MinecraftVersion = "1.21.1"
	cfg.Profile.Loader = "fabric"
	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	lock := &config.Lockfile{Mods: []config.LockfileMod{}}
	lock.AddOrUpdateMod(config.LockfileMod{
		Slug:          "sodium",
		Name:          "Sodium",
		VersionNumber: "0.5.8",
		Side:          "client",
		Pinned:        true,
	})
	if err := config.SaveLockfile(lockPath, lock); err != nil {
		t.Fatalf("Failed to save lockfile: %v", err)
	}

	app, err := NewApp(cfgPath, lockPath)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}

	return app, tmpDir
}

func TestApp_Initialization(t *testing.T) {
	app, _ := setupTestApp(t)

	if app.ActiveTab != TabMods {
		t.Errorf("Expected active tab TabMods (0), got %v", app.ActiveTab)
	}
	if app.Config == nil || app.Config.Profile.Name != "TestPack" {
		t.Errorf("Expected config Profile.Name 'TestPack', got %v", app.Config)
	}

	view := app.View()
	cleanView := styles.StripANSI(view)
	if !strings.Contains(cleanView, "Cloud Mod Manager") {
		t.Errorf("Expected view to contain app header, got %s", cleanView)
	}
	if !strings.Contains(cleanView, "Installed Mods") {
		t.Errorf("Expected view to contain Tab title, got %s", cleanView)
	}
}

func TestApp_TabNavigation(t *testing.T) {
	app, _ := setupTestApp(t)

	// Step from TabMods to TabConfig via '3' key (mods tab is not typing)
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	app = m.(AppModel)
	if app.ActiveTab != TabConfig {
		t.Errorf("Expected ActiveTab TabConfig after '3', got %v", app.ActiveTab)
	}

	// Switch to Tab 4 (Sync) via direct SwitchTabMsg
	m, _ = app.Update(SwitchTabMsg{Tab: TabSync})
	app = m.(AppModel)
	if app.ActiveTab != TabSync {
		t.Errorf("Expected ActiveTab TabSync, got %v", app.ActiveTab)
	}

	// Jump to TabMods via '1'
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	app = m.(AppModel)
	if app.ActiveTab != TabMods {
		t.Errorf("Expected ActiveTab TabMods after '1', got %v", app.ActiveTab)
	}

	// Step to Search Tab via Tab key
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyTab})
	app = m.(AppModel)
	if app.ActiveTab != TabSearch {
		t.Errorf("Expected ActiveTab TabSearch after Tab, got %v", app.ActiveTab)
	}

	// Shift+Tab from TabMods wraps to TabSync
	m, _ = app.Update(SwitchTabMsg{Tab: TabMods})
	app = m.(AppModel)
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	app = m.(AppModel)
	if app.ActiveTab != TabSync {
		t.Errorf("Expected Shift+Tab from TabMods to wrap to TabSync, got %v", app.ActiveTab)
	}
}

func TestApp_HelpModal(t *testing.T) {
	app, _ := setupTestApp(t)

	// Open help with '?'
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	app = m.(AppModel)
	if !app.HelpVisible {
		t.Errorf("Expected HelpVisible to be true")
	}

	view := styles.StripANSI(app.View())
	if !strings.Contains(view, "Help & Keybindings") {
		t.Errorf("Expected Help modal in view, got: %s", view)
	}

	// Close help with Esc
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app = m.(AppModel)
	if app.HelpVisible {
		t.Errorf("Expected HelpVisible to be false after Esc")
	}
}

func TestApp_QuitKeybindings(t *testing.T) {
	app, _ := setupTestApp(t)

	// Quit with 'q'
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("Expected quit command on 'q'")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("Expected QuitMsg, got %T", msg)
	}

	// Quit with Ctrl+C
	_, cmd = app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatalf("Expected quit command on Ctrl+C")
	}
	msg = cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("Expected QuitMsg on Ctrl+C, got %T", msg)
	}
}

func TestApp_WindowResize(t *testing.T) {
	app, _ := setupTestApp(t)

	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = m.(AppModel)

	if app.Width != 120 || app.Height != 40 {
		t.Errorf("Expected dimensions 120x40, got %dx%d", app.Width, app.Height)
	}
	if app.ModsTab.Width != 120 {
		t.Errorf("Expected ModsTab width 120, got %d", app.ModsTab.Width)
	}
}

func TestApp_InvalidConfigPath(t *testing.T) {
	_, err := NewApp("/nonexistent/cmm.toml", "/nonexistent/cmm.lock")
	if err == nil {
		t.Errorf("Expected error for nonexistent config file, got nil")
	}
}
