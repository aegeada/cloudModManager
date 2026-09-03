package launcher

import (
	"os"
	"path/filepath"
	"testing"

	"cmm/internal/config"
)

func TestSyncInstance_CompatibilityValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Setup mock Prism instance with MC 1.20.1 and Fabric
	prismDir := filepath.Join(tmpDir, "PrismLauncher")
	instDir := filepath.Join(prismDir, "instances", "TestInst", ".minecraft")
	_ = os.MkdirAll(instDir, 0755)
	_ = os.WriteFile(filepath.Join(prismDir, "instances", "TestInst", "instance.cfg"), []byte("name=Test Instance\nIntendedVersion=1.20.1\n"), 0644)
	pack := `{"components":[{"uid":"net.minecraft","version":"1.20.1"},{"uid":"net.fabricmc.fabric-loader","version":"0.15.11"}]}`
	_ = os.WriteFile(filepath.Join(prismDir, "instances", "TestInst", "mmc-pack.json"), []byte(pack), 0644)

	// 2. Setup cmm.toml with MC 1.21.1 and Fabric
	cfgFile := filepath.Join(tmpDir, "cmm.toml")
	cfgContent := `
[profile]
name = "my-pack"
minecraft_version = "1.21.1"
loader = "fabric"
loader_version = "0.16.5"
side = "both"
`
	_ = os.WriteFile(cfgFile, []byte(cfgContent), 0644)

	// 3. Setup cmm.lock
	lockFile := filepath.Join(tmpDir, "cmm.lock")
	lockContent := `
[[mods]]
name = "Fabric API"
slug = "fabric-api"
file_name = "fabric-api-0.100.0.jar"
side = "both"
`
	_ = os.WriteFile(lockFile, []byte(lockContent), 0644)

	syncer := NewSyncer(nil, DetectorOptions{
		HomeDir: tmpDir,
		CustomPaths: map[LauncherType][]string{
			LauncherPrism: {prismDir},
		},
	})

	// Case A: Mismatched MC Version without Force -> Must Fail
	_, err := syncer.SyncInstance(LauncherSyncOptions{
		InstanceName: "Test Instance",
		ConfigPath:   cfgFile,
		LockPath:     lockFile,
		Force:        false,
	})
	if err == nil {
		t.Fatalf("expected error due to MC version mismatch (1.20.1 vs 1.21.1), got nil")
	}

	// Case B: Mismatched MC Version with Force -> Must Succeed
	res, err := syncer.SyncInstance(LauncherSyncOptions{
		InstanceName: "Test Instance",
		ConfigPath:   cfgFile,
		LockPath:     lockFile,
		Force:        true,
	})
	if err != nil {
		t.Fatalf("expected success with Force=true, got: %v", err)
	}
	if len(res.AddedMods) != 1 || res.AddedMods[0] != "fabric-api-0.100.0.jar" {
		t.Errorf("unexpected added mods: %v", res.AddedMods)
	}

	// Case C: Mismatched Loader without Force
	cfgContentForge := `
[profile]
name = "my-pack"
minecraft_version = "1.20.1"
loader = "forge"
`
	_ = os.WriteFile(cfgFile, []byte(cfgContentForge), 0644)
	_, err = syncer.SyncInstance(LauncherSyncOptions{
		InstanceName: "Test Instance",
		ConfigPath:   cfgFile,
		LockPath:     lockFile,
		Force:        false,
	})
	if err == nil {
		t.Fatalf("expected error due to loader mismatch (fabric vs forge), got nil")
	}

	// Case D: Mismatched Loader with Force
	_, err = syncer.SyncInstance(LauncherSyncOptions{
		InstanceName: "Test Instance",
		ConfigPath:   cfgFile,
		LockPath:     lockFile,
		Force:        true,
	})
	if err != nil {
		t.Fatalf("expected success with Force=true on loader mismatch, got: %v", err)
	}
}

func TestSyncInstance_ClientSideFiltering(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup mock Modrinth App profile
	modrinthDir := filepath.Join(tmpDir, "ModrinthApp")
	pDir := filepath.Join(modrinthDir, "profiles", "ClientTest")
	_ = os.MkdirAll(pDir, 0755)
	pJSON := `{"name":"Client Profile","game_version":"1.21.1","loader":"fabric","loader_version":"0.16.5"}`
	_ = os.WriteFile(filepath.Join(pDir, "profile.json"), []byte(pJSON), 0644)

	cfgFile := filepath.Join(tmpDir, "cmm.toml")
	cfgContent := `
[profile]
name = "pack"
minecraft_version = "1.21.1"
loader = "fabric"
`
	_ = os.WriteFile(cfgFile, []byte(cfgContent), 0644)

	// Lockfile with client, both, default, and server-only mods
	lockFile := filepath.Join(tmpDir, "cmm.lock")
	lockContent := `
[[mods]]
name = "Sodium"
slug = "sodium"
file_name = "sodium-fabric-0.5.8.jar"
side = "client"

[[mods]]
name = "Fabric API"
slug = "fabric-api"
file_name = "fabric-api-0.100.0.jar"
side = "both"

[[mods]]
name = "Cloth Config"
slug = "cloth-config"
file_name = "cloth-config-15.0.127.jar"

[[mods]]
name = "Spark"
slug = "spark"
file_name = "spark-server-1.10.5.jar"
side = "server"

[[mods]]
name = "LuckPerms"
slug = "luckperms"
file_name = "luckperms-5.4.102.jar"
side = "SERVER"
`
	_ = os.WriteFile(lockFile, []byte(lockContent), 0644)

	syncer := NewSyncer(nil, DetectorOptions{
		CustomPaths: map[LauncherType][]string{
			LauncherModrinth: {modrinthDir},
		},
	})

	res, err := syncer.SyncInstance(LauncherSyncOptions{
		InstanceName: "Client Profile",
		ConfigPath:   cfgFile,
		LockPath:     lockFile,
	})
	if err != nil {
		t.Fatalf("SyncInstance failed: %v", err)
	}

	// Verify skipped server-only mods
	if len(res.SkippedMods) != 2 {
		t.Errorf("expected 2 skipped server mods, got %d: %v", len(res.SkippedMods), res.SkippedMods)
	}

	// Verify added client mods
	if len(res.AddedMods) != 3 {
		t.Errorf("expected 3 added client mods, got %d: %v", len(res.AddedMods), res.AddedMods)
	}

	// Check files created on disk in instance modsDir
	modsDir := filepath.Join(pDir, "mods")
	if _, err := os.Stat(filepath.Join(modsDir, "sodium-fabric-0.5.8.jar")); err != nil {
		t.Errorf("sodium jar missing on disk")
	}
	if _, err := os.Stat(filepath.Join(modsDir, "fabric-api-0.100.0.jar")); err != nil {
		t.Errorf("fabric-api jar missing on disk")
	}
	if _, err := os.Stat(filepath.Join(modsDir, "cloth-config-15.0.127.jar")); err != nil {
		t.Errorf("cloth-config jar missing on disk")
	}
	if _, err := os.Stat(filepath.Join(modsDir, "spark-server-1.10.5.jar")); err == nil {
		t.Errorf("server mod spark should NOT exist in client instance mods dir")
	}
	if _, err := os.Stat(filepath.Join(modsDir, "luckperms-5.4.102.jar")); err == nil {
		t.Errorf("server mod luckperms should NOT exist in client instance mods dir")
	}
}

func TestSyncInstance_DryRun(t *testing.T) {
	tmpDir := t.TempDir()

	cfDir := filepath.Join(tmpDir, "curseforge", "minecraft", "Instances", "DryTest")
	modsDir := filepath.Join(cfDir, "mods")
	_ = os.MkdirAll(modsDir, 0755)

	cfJSON := `{"name":"Dry Run Test","gameVersion":"1.21.1","baseModLoader":{"name":"fabric-0.16.5"}}`
	_ = os.WriteFile(filepath.Join(cfDir, "minecraftinstance.json"), []byte(cfJSON), 0644)

	// Existing unwanted mod on disk
	_ = os.WriteFile(filepath.Join(modsDir, "obsolete-mod.jar"), []byte("data"), 0644)

	cfgFile := filepath.Join(tmpDir, "cmm.toml")
	_ = os.WriteFile(cfgFile, []byte("[profile]\nname=\"pack\"\nminecraft_version=\"1.21.1\"\nloader=\"fabric\"\n"), 0644)

	lockFile := filepath.Join(tmpDir, "cmm.lock")
	lockContent := `
[[mods]]
name = "Iris Shaders"
slug = "iris"
file_name = "iris-1.7.0.jar"
side = "client"
`
	_ = os.WriteFile(lockFile, []byte(lockContent), 0644)

	syncer := NewSyncer(nil, DetectorOptions{
		CustomPaths: map[LauncherType][]string{
			LauncherCurseForge: {filepath.Dir(cfDir)},
		},
	})

	res, err := syncer.SyncInstance(LauncherSyncOptions{
		InstanceName: "Dry Run Test",
		ConfigPath:   cfgFile,
		LockPath:     lockFile,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("DryRun failed: %v", err)
	}

	if len(res.AddedMods) != 1 || res.AddedMods[0] != "iris-1.7.0.jar" {
		t.Errorf("expected 1 added mod in dry run, got: %v", res.AddedMods)
	}
	if len(res.RemovedMods) != 1 || res.RemovedMods[0] != "obsolete-mod.jar" {
		t.Errorf("expected 1 removed mod in dry run, got: %v", res.RemovedMods)
	}

	// Verify disk was NOT modified
	if _, err := os.Stat(filepath.Join(modsDir, "obsolete-mod.jar")); err != nil {
		t.Errorf("obsolete-mod.jar should STILL exist on disk after dry run")
	}
	if _, err := os.Stat(filepath.Join(modsDir, "iris-1.7.0.jar")); err == nil {
		t.Errorf("iris-1.7.0.jar should NOT be created on disk during dry run")
	}
}

func TestSyncInstance_DeltaPruningAndLockfile(t *testing.T) {
	tmpDir := t.TempDir()

	mcDir := filepath.Join(tmpDir, ".minecraft")
	modsDir := filepath.Join(mcDir, "mods")
	_ = os.MkdirAll(modsDir, 0755)

	vProfiles := `{"profiles":{"vanilla-pack":{"name":"Vanilla Pack","lastVersionId":"fabric-loader-0.16.5-1.21.1"}}}`
	_ = os.WriteFile(filepath.Join(mcDir, "launcher_profiles.json"), []byte(vProfiles), 0644)

	// Create existing obsolete mod in mods dir
	_ = os.WriteFile(filepath.Join(modsDir, "old-mod.jar"), []byte("data"), 0644)

	cfgFile := filepath.Join(tmpDir, "cmm.toml")
	_ = os.WriteFile(cfgFile, []byte("[profile]\nname=\"pack\"\nminecraft_version=\"1.21.1\"\nloader=\"fabric\"\n"), 0644)

	lockFile := filepath.Join(tmpDir, "cmm.lock")
	lockContent := `
[[mods]]
name = "Lithium"
slug = "lithium"
file_name = "lithium-fabric-0.11.2.jar"
side = "both"
`
	_ = os.WriteFile(lockFile, []byte(lockContent), 0644)

	syncer := NewSyncer(nil, DetectorOptions{
		CustomPaths: map[LauncherType][]string{
			LauncherVanilla: {mcDir},
		},
	})

	// 1. Initial Sync
	res, err := syncer.SyncInstance(LauncherSyncOptions{
		InstanceName: "Vanilla Pack",
		ConfigPath:   cfgFile,
		LockPath:     lockFile,
	})
	if err != nil {
		t.Fatalf("initial sync failed: %v", err)
	}

	if len(res.AddedMods) != 1 || res.AddedMods[0] != "lithium-fabric-0.11.2.jar" {
		t.Errorf("expected lithium added, got: %v", res.AddedMods)
	}
	if len(res.RemovedMods) != 1 || res.RemovedMods[0] != "old-mod.jar" {
		t.Errorf("expected old-mod removed, got: %v", res.RemovedMods)
	}

	// Verify old-mod.jar was deleted from disk
	if _, err := os.Stat(filepath.Join(modsDir, "old-mod.jar")); !os.IsNotExist(err) {
		t.Errorf("old-mod.jar was not deleted from disk")
	}

	// Verify instance cmm.lock was created
	instLock, err := config.LoadLockfile(filepath.Join(mcDir, "cmm.lock"))
	if err != nil || len(instLock.Mods) != 1 {
		t.Fatalf("instance cmm.lock missing or invalid: %v", err)
	}

	// 2. Second Sync (should be up to date)
	res2, err := syncer.SyncInstance(LauncherSyncOptions{
		InstanceName: "Vanilla Pack",
		ConfigPath:   cfgFile,
		LockPath:     lockFile,
	})
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}
	if len(res2.AddedMods) != 0 || len(res2.RemovedMods) != 0 {
		t.Errorf("expected no changes on second sync, got added=%v, removed=%v", res2.AddedMods, res2.RemovedMods)
	}
}
