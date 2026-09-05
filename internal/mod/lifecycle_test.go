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
	if res.AlreadyDisabled || res.HasDependentsWarning || res.NewFileName != "sodium-fabric-0.5.8.jar.disabled" {
		t.Errorf("expected successful disable, got: %+v", res)
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

	// 2. Disable already disabled mod -> should return AlreadyDisabled gracefully
	res2, err := mgr.DisableMod("sodium", false)
	if err != nil {
		t.Fatalf("subsequent DisableMod failed: %v", err)
	}
	if !res2.AlreadyDisabled {
		t.Errorf("expected already disabled when disabling already disabled mod")
	}

	// 3. Enable mod
	res3, err := mgr.EnableMod("sodium")
	if err != nil {
		t.Fatalf("EnableMod failed: %v", err)
	}
	if res3.AlreadyEnabled || res3.NewFileName != "sodium-fabric-0.5.8.jar" {
		t.Errorf("expected enabled mod, got: %+v", res3)
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

func TestLifecycle_DisableMod_ActiveDependentsWarningAndForce(t *testing.T) {
	server, client := setupMockModrinthServer(t)
	defer server.Close()

	tmpDir, err := os.MkdirTemp("", "cmm-dependents-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	modsDir := filepath.Join(tmpDir, "mods")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	fapiFile := filepath.Join(modsDir, "fabric-api-0.100.0.jar")
	_ = os.WriteFile(fapiFile, []byte("fapi-content"), 0644)
	sodiumFile := filepath.Join(modsDir, "sodium-0.5.8.jar")
	_ = os.WriteFile(sodiumFile, []byte("sodium-content"), 0644)

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
	_ = config.SaveConfig(configPath, cfg)

	lock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "fabric-api",
				Name:          "Fabric API",
				ProjectID:     "P7dR8mSH",
				VersionNumber: "0.100.0",
				FileName:      "fabric-api-0.100.0.jar",
				Disabled:      false,
			},
			{
				Slug:          "sodium",
				Name:          "Sodium",
				ProjectID:     "AANobbMI",
				VersionNumber: "0.5.8",
				FileName:      "sodium-0.5.8.jar",
				Disabled:      false,
			},
		},
	}
	_ = config.SaveLockfile(lockPath, lock)

	mgr := NewManager(client, configPath, lockPath)

	// 1. Without force -> must return HasDependentsWarning = true and NOT rename or change lockfile
	res, err := mgr.DisableMod("fabric-api", false)
	if err != nil {
		t.Fatalf("DisableMod failed: %v", err)
	}
	if !res.HasDependentsWarning {
		t.Errorf("expected HasDependentsWarning true, got false")
	}
	if len(res.ActiveDependents) == 0 || res.ActiveDependents[0] != "sodium" {
		t.Errorf("expected active dependent 'sodium', got: %v", res.ActiveDependents)
	}

	// Verify fabric-api is NOT renamed on disk
	if _, err := os.Stat(fapiFile); err != nil {
		t.Errorf("expected fabric-api to remain intact on disk, but not found: %v", err)
	}
	if _, err := os.Stat(fapiFile + ".disabled"); err == nil {
		t.Errorf("fabric-api.disabled should not exist")
	}

	// Verify lockfile NOT updated
	checkLock, _ := config.LoadLockfile(lockPath)
	fapiMod := checkLock.GetMod("fabric-api")
	if fapiMod == nil || fapiMod.Disabled {
		t.Errorf("fabric-api should not be disabled in lockfile: %+v", fapiMod)
	}

	// 2. With force=true -> must successfully disable
	resForced, err := mgr.DisableMod("fabric-api", true)
	if err != nil {
		t.Fatalf("forced DisableMod failed: %v", err)
	}
	if resForced.HasDependentsWarning {
		t.Errorf("expected HasDependentsWarning false when forced")
	}
	if len(resForced.ActiveDependents) == 0 {
		t.Errorf("expected ActiveDependents to still be reported for notice")
	}

	// Verify file is renamed to .disabled
	if _, err := os.Stat(fapiFile + ".disabled"); err != nil {
		t.Errorf("expected fabric-api.disabled to exist on disk: %v", err)
	}
	if _, err := os.Stat(fapiFile); err == nil {
		t.Errorf("expected original fabric-api.jar to be renamed away")
	}

	// Verify lockfile updated to disabled
	checkLock2, _ := config.LoadLockfile(lockPath)
	fapiMod2 := checkLock2.GetMod("fabric-api")
	if fapiMod2 == nil || !fapiMod2.Disabled || fapiMod2.FileName != "fabric-api-0.100.0.jar.disabled" {
		t.Errorf("expected fabric-api disabled in lockfile, got: %+v", fapiMod2)
	}
}

func TestLifecycle_DisableModWithOptions_DryRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cmm-dryrun-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	modsDir := filepath.Join(tmpDir, "mods")
	_ = os.MkdirAll(modsDir, 0755)

	jarFile := filepath.Join(modsDir, "sodium-0.5.8.jar")
	_ = os.WriteFile(jarFile, []byte("jar"), 0644)

	configPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")

	cfg := &config.Config{
		Profile: config.Profile{Name: "test", MinecraftVersion: "1.21.1", Loader: "fabric"},
		Paths:   config.Paths{ModsDir: modsDir},
	}
	_ = config.SaveConfig(configPath, cfg)

	lock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{Slug: "sodium", Name: "Sodium", FileName: "sodium-0.5.8.jar", Disabled: false},
		},
	}
	_ = config.SaveLockfile(lockPath, lock)

	mgr := NewManager(nil, configPath, lockPath)

	res, err := mgr.DisableModWithOptions("sodium", DisableOptions{DryRun: true})
	if err != nil {
		t.Fatalf("DisableModWithOptions dry-run failed: %v", err)
	}
	if !res.DryRun {
		t.Errorf("expected DryRun=true, got false")
	}

	// Disk must NOT be touched
	if _, err := os.Stat(jarFile); err != nil {
		t.Errorf("expected original jar file to remain in dry-run")
	}
	if _, err := os.Stat(jarFile + ".disabled"); err == nil {
		t.Errorf("disabled jar should not exist in dry-run")
	}

	// Lockfile must NOT be touched
	loadedLock, _ := config.LoadLockfile(lockPath)
	modEntry := loadedLock.GetMod("sodium")
	if modEntry == nil || modEntry.Disabled {
		t.Errorf("lockfile should remain untouched in dry-run")
	}
}
