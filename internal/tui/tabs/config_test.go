package tabs

import (
	"path/filepath"
	"strings"
	"testing"

	"cmm/internal/config"
	"cmm/internal/tui/styles"
	"cmm/internal/tui/tea"
)

func setupTestConfigTab(t *testing.T) (ConfigModel, string) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")

	cfg := config.DefaultConfig()
	cfg.Profile.Name = "TestPack"
	cfg.Profile.MinecraftVersion = "1.21.1"
	cfg.Profile.Loader = "fabric"
	cfg.Profile.Side = "both"
	cfg.Paths.ModsDir = "mods"
	_ = config.SaveConfig(cfgPath, cfg)

	model := NewConfigModel(cfgPath, cfg)
	return model, cfgPath
}

func TestConfigTab_Initialization(t *testing.T) {
	m, _ := setupTestConfigTab(t)

	if m.Inputs[FieldProfileName].Value() != "TestPack" {
		t.Errorf("Expected ProfileName 'TestPack', got '%s'", m.Inputs[FieldProfileName].Value())
	}
	if m.Inputs[FieldMCVersion].Value() != "1.21.1" {
		t.Errorf("Expected MCVersion '1.21.1', got '%s'", m.Inputs[FieldMCVersion].Value())
	}
	if m.Inputs[FieldLoader].Value() != "fabric" {
		t.Errorf("Expected Loader 'fabric', got '%s'", m.Inputs[FieldLoader].Value())
	}

	view := styles.StripANSI(m.View())
	if !strings.Contains(view, "Profile Name:") || !strings.Contains(view, "TestPack") {
		t.Errorf("Expected Profile form in view, got:\n%s", view)
	}
}

func TestConfigTab_FieldNavigation(t *testing.T) {
	m, _ := setupTestConfigTab(t)

	if m.ActiveField != FieldProfileName {
		t.Errorf("Expected initial field FieldProfileName (0), got %v", m.ActiveField)
	}

	// Navigate Down with KeyDown
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.ActiveField != FieldMCVersion {
		t.Errorf("Expected active field FieldMCVersion (1), got %v", m.ActiveField)
	}

	// Navigate with Tab
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.ActiveField != FieldLoader {
		t.Errorf("Expected active field FieldLoader (2), got %v", m.ActiveField)
	}
}

func TestConfigTab_SpaceCycleOptions(t *testing.T) {
	m, _ := setupTestConfigTab(t)

	// Move to Loader field
	m.ActiveField = FieldLoader
	m.updateFocus()

	// Press Space to cycle loader: fabric -> forge
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if m.Inputs[FieldLoader].Value() != "forge" {
		t.Errorf("Expected loader 'forge' after Space, got '%s'", m.Inputs[FieldLoader].Value())
	}

	// Move to Side field
	m.ActiveField = FieldSide
	m.updateFocus()

	// Press Space to cycle side: both -> server
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if m.Inputs[FieldSide].Value() != "server" {
		t.Errorf("Expected side 'server' after Space, got '%s'", m.Inputs[FieldSide].Value())
	}
}

func TestConfigTab_SaveValidationAndWrite(t *testing.T) {
	m, cfgPath := setupTestConfigTab(t)

	// Set invalid empty Profile Name
	m.Inputs[FieldProfileName].SetValue("")
	m.ActiveField = FieldSaveButton

	cmd := m.SaveConfigCmd()
	msg := cmd()
	savedMsg, ok := msg.(ConfigSavedMsg)
	if !ok || savedMsg.Err == nil {
		t.Fatalf("Expected validation error for empty profile name, got: %+v", msg)
	}

	// Set valid fields
	m.Inputs[FieldProfileName].SetValue("UpdatedPack")
	m.Inputs[FieldMCVersion].SetValue("1.21.0")
	m.Inputs[FieldLoader].SetValue("neoforge")
	m.Inputs[FieldSide].SetValue("both")

	cmd = m.SaveConfigCmd()
	msg = cmd()
	savedMsg, ok = msg.(ConfigSavedMsg)
	if !ok || savedMsg.Err != nil {
		t.Fatalf("Expected successful save, got: %+v", msg)
	}

	// Verify cmm.toml on disk
	diskCfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("Failed to load disk config: %v", err)
	}
	if diskCfg.Profile.Name != "UpdatedPack" || diskCfg.Profile.Loader != "neoforge" {
		t.Errorf("Disk config not updated: %+v", diskCfg)
	}
}

func TestConfigTab_Reload(t *testing.T) {
	m, cfgPath := setupTestConfigTab(t)

	// Modify field in UI
	m.Inputs[FieldProfileName].SetValue("TemporaryName")

	// Update cmm.toml directly on disk
	diskCfg, _ := config.LoadConfig(cfgPath)
	diskCfg.Profile.Name = "DiskAuthoritativeName"
	_ = config.SaveConfig(cfgPath, diskCfg)

	// Trigger Reload cmd
	cmd := m.ReloadConfigCmd()
	msg := cmd()
	m, _ = m.Update(msg)

	if m.Inputs[FieldProfileName].Value() != "DiskAuthoritativeName" {
		t.Errorf("Expected reloaded profile name 'DiskAuthoritativeName', got '%s'", m.Inputs[FieldProfileName].Value())
	}
}
