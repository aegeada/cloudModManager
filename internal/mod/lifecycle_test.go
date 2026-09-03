package mod

import (
	"os"
	"path/filepath"
	"testing"

	"cmm/internal/config"
)

func TestLifecycle_DisableAndEnable(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cmm-lifecycle-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	modsDir := filepath.Join(tmpDir, "mods")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	jarFile := filepath.Join(modsDir, "sodium-fabric-0.5.8.jar")
	if err := os.WriteFile(jarFile, []byte("dummy-jar-content"), 0644); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")

	cfg := &config.Config{
		Profile: config.Profile{
			Name:             "test",
			MinecraftVersion: "1.21.1",
			Loader:           "fabric",
		},
		Paths: config.Paths{
			ModsDir: modsDir,
		},
	}
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	lock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:     "sodium",
				Name:     "Sodium",
				FileName: "sodium-fabric-0.5.8.jar",
				Disabled: false,
			},
		},
	}
	if err := config.SaveLockfile(lockPath, lock); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(nil, configPath, lockPath)

	// 1. Disable mod
	res, err := mgr.DisableMod("sodium", false)
	if err != nil {
		t.Fatalf("DisableMod failed: %v", err)
	}
	if !res.Disabled {
		t.Errorf("expected disabled true, got false")
	}

	// Verify file renamed on disk
	disabledFile := filepath.Join(modsDir, "sodium-fabric-0.5.8.jar.disabled")
	if _, err := os.Stat(disabledFile); err != nil {
		t.Errorf("expected disabled jar file at %s: %v", disabledFile, err)
	}
	if _, err := os.Stat(jarFile); err == nil {
		t.Errorf("expected original jar file to be renamed away")
	}

	// Verify lockfile updated
	loadedLock, _ := config.LoadLockfile(lockPath)
	modEntry := loadedLock.GetMod("sodium")
	if modEntry == nil || !modEntry.Disabled || modEntry.FileName != "sodium-fabric-0.5.8.jar.disabled" {
		t.Errorf("unexpected lockfile mod state after disable: %+v", modEntry)
	}

	// 2. Disable already disabled mod -> should return warning gracefully
	res2, err := mgr.DisableMod("sodium", false)
	if err != nil {
		t.Fatalf("subsequent DisableMod failed: %v", err)
	}
	if res2.Warning == "" {
		t.Errorf("expected warning when disabling already disabled mod")
	}

	// 3. Enable mod
	res3, err := mgr.EnableMod("sodium")
	if err != nil {
		t.Fatalf("EnableMod failed: %v", err)
	}
	if res3.Disabled {
		t.Errorf("expected disabled false after enable")
	}

	// Verify file restored on disk
	if _, err := os.Stat(jarFile); err != nil {
		t.Errorf("expected restored jar file at %s: %v", jarFile, err)
	}
	if _, err := os.Stat(disabledFile); err == nil {
		t.Errorf("expected disabled jar file to be renamed away")
	}

	// Verify lockfile restored
	loadedLock2, _ := config.LoadLockfile(lockPath)
	modEntry2 := loadedLock2.GetMod("sodium")
	if modEntry2 == nil || modEntry2.Disabled || modEntry2.FileName != "sodium-fabric-0.5.8.jar" {
		t.Errorf("unexpected lockfile mod state after enable: %+v", modEntry2)
	}
}
