package config

import (
	"os"
	"testing"
)

func TestSaveAndLoadConfig(t *testing.T) {
	tempFile, err := os.CreateTemp("", "cmm_test_*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	cfg := &Config{
		Profile: Profile{
			Name:             "TestServer",
			MinecraftVersion: "1.21.1",
			Loader:           "fabric",
			Side:             "server",
		},
		Modrinth: Modrinth{
			Token: "test-token",
		},
	}

	err = SaveConfig(tempFile.Name(), cfg)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	loaded, err := LoadConfig(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loaded.Profile.Name != "TestServer" {
		t.Errorf("Expected Name 'TestServer', got %v", loaded.Profile.Name)
	}
	if loaded.Profile.MinecraftVersion != "1.21.1" {
		t.Errorf("Expected MinecraftVersion '1.21.1', got %v", loaded.Profile.MinecraftVersion)
	}
	if loaded.Profile.Loader != "fabric" {
		t.Errorf("Expected Loader 'fabric', got %v", loaded.Profile.Loader)
	}
	if loaded.Modrinth.Token != "test-token" {
		t.Errorf("Expected Modrinth Token 'test-token', got %v", loaded.Modrinth.Token)
	}
}

func TestLoadConfig_TopLevelAndLegacyProperties(t *testing.T) {
	tempFile, err := os.CreateTemp("", "cmm_legacy_*.toml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name())

	content := `
name = "LegacyServer"
side = "server"
mods_dir = "custom_mods"
[server]
mc_version = "1.21.1"
loader = "fabric"
`
	if _, err := tempFile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tempFile.Close()

	cfg, err := LoadConfig(tempFile.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Profile.Name != "LegacyServer" {
		t.Errorf("expected Name LegacyServer, got %s", cfg.Profile.Name)
	}
	if cfg.Profile.Side != "server" {
		t.Errorf("expected Side server, got %s", cfg.Profile.Side)
	}
	if cfg.Profile.MinecraftVersion != "1.21.1" {
		t.Errorf("expected MinecraftVersion 1.21.1, got %s", cfg.Profile.MinecraftVersion)
	}
	if cfg.Profile.Loader != "fabric" {
		t.Errorf("expected Loader fabric, got %s", cfg.Profile.Loader)
	}
	if cfg.Paths.ModsDir != "custom_mods" {
		t.Errorf("expected ModsDir custom_mods, got %s", cfg.Paths.ModsDir)
	}
}

