package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConfig_NonExistentFile(t *testing.T) {
	_, err := LoadConfig("/path/to/definitely/non/existent/file.toml")
	if err == nil {
		t.Error("expected error when loading non-existent file, got nil")
	}
}

func TestConfig_MalformedTOML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cmm-malformed-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	badFile := filepath.Join(tmpDir, "bad.toml")
	if err := os.WriteFile(badFile, []byte("[[invalid toml content --:::"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = LoadConfig(badFile)
	if err == nil {
		t.Error("expected error for malformed TOML, got nil")
	}
}

func TestConfig_NestedDirectorySave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cmm-nested-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	nestedPath := filepath.Join(tmpDir, "sub1", "sub2", "sub3", "cmm.toml")
	cfg := &Config{
		Profile: Profile{
			Name:             "test-server",
			MinecraftVersion: "26.2",
			Loader:           "fabric",
			Side:             "server",
		},
	}

	if err := SaveConfig(nestedPath, cfg); err != nil {
		t.Fatalf("failed to save config to nested directory: %v", err)
	}

	loaded, err := LoadConfig(nestedPath)
	if err != nil {
		t.Fatalf("failed to load saved nested config: %v", err)
	}

	if loaded.Profile.MinecraftVersion != "26.2" {
		t.Errorf("expected 26.2, got %s", loaded.Profile.MinecraftVersion)
	}
}

func TestLockfile_NestedDirectorySave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cmm-lock-nested-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	nestedPath := filepath.Join(tmpDir, "a", "b", "c", "cmm.lock")
	lock := &Lockfile{
		Mods: []LockfileMod{
			{
				Slug:          "sodium",
				Name:          "Sodium",
				VersionNumber: "0.5.8",
			},
		},
	}

	if err := SaveLockfile(nestedPath, lock); err != nil {
		t.Fatalf("failed to save lockfile to nested directory: %v", err)
	}

	loaded, err := LoadLockfile(nestedPath)
	if err != nil {
		t.Fatalf("failed to load saved nested lockfile: %v", err)
	}

	if len(loaded.Mods) != 1 || loaded.Mods[0].Slug != "sodium" {
		t.Errorf("unexpected loaded mods: %+v", loaded.Mods)
	}
}

func TestLockfile_EdgeCases(t *testing.T) {
	lock := &Lockfile{}

	// Non-existent mod operations
	if lock.GetMod("non-existent") != nil {
		t.Error("expected nil for non-existent mod")
	}
	if lock.RemoveMod("non-existent") {
		t.Error("expected RemoveMod to return false for non-existent mod")
	}
	if lock.IsPinned("non-existent") {
		t.Error("expected IsPinned to return false for non-existent mod")
	}
	if lock.SetPinned("non-existent", true) {
		t.Error("expected SetPinned to return false for non-existent mod")
	}

	// Add and update by project ID vs slug
	lock.AddOrUpdateMod(LockfileMod{
		Slug:      "mod-a",
		ProjectID: "proj-1",
		Name:      "Mod A",
		Pinned:    false,
	})

	if mod := lock.GetMod("proj-1"); mod == nil || mod.Slug != "mod-a" {
		t.Errorf("failed to lookup mod by ProjectID: %+v", mod)
	}

	// Update existing by project ID
	lock.AddOrUpdateMod(LockfileMod{
		Slug:      "mod-a-updated",
		ProjectID: "proj-1",
		Name:      "Mod A New",
		Pinned:    true,
	})

	if len(lock.Mods) != 1 {
		t.Errorf("expected 1 mod after update, got %d", len(lock.Mods))
	}
	if !lock.IsPinned("proj-1") {
		t.Error("expected mod to be pinned after update")
	}
}

func TestConfig_ConcurrentRead(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cmm-concurrency-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "cmm.toml")
	cfg := &Config{
		Profile: Profile{
			Name: "stress-server",
		},
	}
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := LoadConfig(path)
			if err != nil || c.Profile.Name != "stress-server" {
				t.Errorf("concurrent read error: %v", err)
			}
		}()
	}
	wg.Wait()
}
