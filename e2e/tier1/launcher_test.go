package tier1

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"cmm/internal/launcher"
)

func setupSyntheticLauncherDirectories(t *testing.T, ctx *harness.TestContext) {
	t.Helper()

	// 1. Vanilla Launcher
	vanillaProfiles := `{
  "profiles": {
    "profile-1": {
      "name": "Vanilla-Survival",
      "type": "custom",
      "lastVersionId": "1.21.1"
    }
  }
}`
	ctx.WriteFile(".minecraft/launcher_profiles.json", vanillaProfiles)
	_ = os.MkdirAll(filepath.Join(ctx.TempDir, ".minecraft", "mods"), 0755)

	// 2. Prism Launcher
	prismInstanceCfg := "name=Prism-Modded\nIntendedVersion=1.21.1\n"
	prismPack := `{
  "components": [
    {"uid": "net.minecraft", "version": "1.21.1"},
    {"uid": "net.fabricmc.fabric-loader", "version": "0.16.5"}
  ]
}`
	ctx.WriteFile(".local/share/PrismLauncher/instances/Prism-Modded/instance.cfg", prismInstanceCfg)
	ctx.WriteFile(".local/share/PrismLauncher/instances/Prism-Modded/mmc-pack.json", prismPack)
	_ = os.MkdirAll(filepath.Join(ctx.TempDir, ".local/share/PrismLauncher/instances/Prism-Modded/.minecraft/mods"), 0755)

	// 3. Modrinth App
	modrinthProfile := `{
  "name": "Modrinth-Fab",
  "game_version": "1.21.1",
  "loader": "fabric",
  "loader_version": "0.16.5"
}`
	ctx.WriteFile(".config/ModrinthApp/profiles/Modrinth-Fab/profile.json", modrinthProfile)
	_ = os.MkdirAll(filepath.Join(ctx.TempDir, ".config/ModrinthApp/profiles/Modrinth-Fab/mods"), 0755)

	// 4. CurseForge
	curseProfile := `{
  "name": "Curse-Test",
  "gameVersion": "1.21.1",
  "baseModLoader": {
    "name": "fabric-0.16.5"
  }
}`
	ctx.WriteFile("curseforge/minecraft/Instances/Curse-Test/minecraftinstance.json", curseProfile)
	_ = os.MkdirAll(filepath.Join(ctx.TempDir, "curseforge/minecraft/Instances/Curse-Test/mods"), 0755)
}

func TestLauncher_ListDetected(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	setupSyntheticLauncherDirectories(t, ctx)

	// 1. Table output (all launchers)
	resTable := ctx.Run("launcher", "list")
	resTable.AssertSuccess()
	resTable.AssertStdoutContains("LAUNCHER", "NAME", "MC VERSION", "LOADER", "MODS DIRECTORY")
	resTable.AssertStdoutContains("Vanilla-Survival", "Vanilla")
	resTable.AssertStdoutContains("Prism-Modded", "Prism Launcher")
	resTable.AssertStdoutContains("Modrinth-Fab", "Modrinth App")
	resTable.AssertStdoutContains("Curse-Test", "CurseForge App")

	// 2. JSON output
	resJSON := ctx.Run("launcher", "list", "--format", "json")
	resJSON.AssertSuccess()

	var instances []launcher.Instance
	if err := json.Unmarshal([]byte(resJSON.Stdout), &instances); err != nil {
		t.Fatalf("failed to parse launcher list JSON: %v\nOutput: %s", err, resJSON.Stdout)
	}

	if len(instances) != 4 {
		t.Fatalf("expected 4 detected instances, got %d", len(instances))
	}

	foundMap := make(map[string]launcher.Instance)
	for _, inst := range instances {
		foundMap[inst.Name] = inst
	}

	if _, ok := foundMap["Vanilla-Survival"]; !ok {
		t.Errorf("missing Vanilla-Survival in detected list")
	}
	if inst, ok := foundMap["Prism-Modded"]; !ok || inst.Loader != "fabric" || inst.MinecraftVersion != "1.21.1" {
		t.Errorf("invalid Prism-Modded data: %+v", inst)
	}
	if inst, ok := foundMap["Modrinth-Fab"]; !ok || inst.Loader != "fabric" {
		t.Errorf("invalid Modrinth-Fab data: %+v", inst)
	}
	if inst, ok := foundMap["Curse-Test"]; !ok || inst.Loader != "fabric" {
		t.Errorf("invalid Curse-Test data: %+v", inst)
	}

	// 3. Filter by launcher type
	resPrismOnly := ctx.Run("launcher", "list", "--launcher", "prism")
	resPrismOnly.AssertSuccess()
	resPrismOnly.AssertStdoutContains("Prism-Modded")
	if strings.Contains(resPrismOnly.Stdout, "Vanilla-Survival") || strings.Contains(resPrismOnly.Stdout, "Modrinth-Fab") {
		t.Errorf("filter --launcher prism output contained other launchers:\n%s", resPrismOnly.Stdout)
	}
}

func TestLauncher_SyncDirect(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	setupSyntheticLauncherDirectories(t, ctx)

	// Setup local modpack in ctx.TempDir
	ctx.WriteFile("cmm.toml", `[profile]
name = "my-sync-pack"
minecraft_version = "1.21.1"
loader = "fabric"
loader_version = "0.16.5"
side = "both"
`)

	clientLock := fmt.Sprintf(`[[mods]]
name = "Sodium"
slug = "sodium"
file_name = "sodium-fabric-0.5.8.jar"
side = "both"
download_url = "%s/download/sodium-fabric-0.5.8.jar"

[[mods]]
name = "Server Dynmap"
slug = "dynmap"
file_name = "dynmap-server-only.jar"
side = "server"
download_url = "%s/download/dynmap-server-only.jar"
`, ms.URL(), ms.URL())
	ctx.WriteFile("cmm.lock", clientLock)

	// Test 1: Dry run
	resDry := ctx.Run("launcher", "sync", "Prism-Modded", "--dry-run")
	resDry.AssertSuccess()
	resDry.AssertStdoutContains("[DRY RUN]", "sodium-fabric-0.5.8.jar")

	targetModsDir := filepath.Join(ctx.TempDir, ".local/share/PrismLauncher/instances/Prism-Modded/.minecraft/mods")
	if _, err := os.Stat(filepath.Join(targetModsDir, "sodium-fabric-0.5.8.jar")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote files to target instance mods directory")
	}

	// Test 2: Real sync
	resSync := ctx.Run("launcher", "sync", "Prism-Modded")
	resSync.AssertSuccess()
	resSync.AssertStdoutContains("Successfully synchronized modpack into Prism Launcher instance 'Prism-Modded'")

	// Verify sodium was downloaded into target instance mods dir
	sodiumPath := filepath.Join(targetModsDir, "sodium-fabric-0.5.8.jar")
	if _, err := os.Stat(sodiumPath); err != nil {
		t.Fatalf("expected sodium jar in launcher instance mods dir '%s': %v", sodiumPath, err)
	}

	// Verify server-only mod was omitted
	dynmapPath := filepath.Join(targetModsDir, "dynmap-server-only.jar")
	if _, err := os.Stat(dynmapPath); !os.IsNotExist(err) {
		t.Fatalf("server-only mod was unexpectedly synced into launcher instance: %s", dynmapPath)
	}
}
