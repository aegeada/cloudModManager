package tier2

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
)

func TestAdversarial_Launcher_CorruptedConfigs(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	// 1. Vanilla: Invalid JSON syntax
	ctx.WriteFile(".minecraft/launcher_profiles.json", `{"profiles": { "bad": { "name": "Broken", "lastVersionId": `)

	// 2. Prism: Corrupted mmc-pack.json and empty instance.cfg
	ctx.WriteFile(".local/share/PrismLauncher/instances/Corrupt-Prism/instance.cfg", "")
	ctx.WriteFile(".local/share/PrismLauncher/instances/Corrupt-Prism/mmc-pack.json", `{"components": [ corrupt_json `)

	// 3. Modrinth: Truncated profile.json
	ctx.WriteFile(".config/ModrinthApp/profiles/Corrupt-Modrinth/profile.json", `{"name": "Trun`)

	// 4. CurseForge: Binary garbage in minecraftinstance.json
	ctx.WriteFile("curseforge/minecraft/Instances/Corrupt-Curse/minecraftinstance.json", "\x00\xff\xfe\x00GARBAGE")

	// 5. One Valid Prism Instance to ensure discovery resilience
	validCfg := "name=Resilient-Valid\nIntendedVersion=1.21.1\n"
	validPack := `{"components":[{"uid":"net.minecraft","version":"1.21.1"},{"uid":"net.fabricmc.fabric-loader","version":"0.16.5"}]}`
	ctx.WriteFile(".local/share/PrismLauncher/instances/Valid-Inst/instance.cfg", validCfg)
	ctx.WriteFile(".local/share/PrismLauncher/instances/Valid-Inst/mmc-pack.json", validPack)
	_ = os.MkdirAll(filepath.Join(ctx.TempDir, ".local/share/PrismLauncher/instances/Valid-Inst/.minecraft/mods"), 0755)

	// Verify command does not panic and successfully recovers valid instance
	res := ctx.Run("launcher", "list")
	res.AssertSuccess()
	res.AssertStdoutContains("Resilient-Valid")

	resJSON := ctx.Run("launcher", "list", "--format", "json")
	resJSON.AssertSuccess()
	resJSON.AssertStdoutContains("Resilient-Valid")
}

func TestAdversarial_Launcher_NonExistentAndMissingDirectories(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	// No launchers configured at all
	resList := ctx.Run("launcher", "list")
	resList.AssertSuccess()
	resList.AssertStdoutContains("No Minecraft launcher instances detected.")

	// Search / Sync non-existent instance
	ctx.WriteFile("cmm.toml", `[profile]
name = "pack"
minecraft_version = "1.21.1"
loader = "fabric"
side = "both"
`)
	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"mod-a\"\n")

	resSync := ctx.Run("launcher", "sync", "non-existent-instance-xyz")
	resSync.AssertFailure()
	resSync.AssertStderrContains("no launcher instances detected", "non-existent-instance-xyz")
}

func TestAdversarial_Launcher_VersionAndLoaderMismatch(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	// 1. Setup Prism Instance with MC 1.20.1 Fabric
	prismCfg := "name=Old-Version-Inst\nIntendedVersion=1.20.1\n"
	prismPack := `{"components":[{"uid":"net.minecraft","version":"1.20.1"},{"uid":"net.fabricmc.fabric-loader","version":"0.15.11"}]}`
	ctx.WriteFile(".local/share/PrismLauncher/instances/Old-Version-Inst/instance.cfg", prismCfg)
	ctx.WriteFile(".local/share/PrismLauncher/instances/Old-Version-Inst/mmc-pack.json", prismPack)
	instModsDir := filepath.Join(ctx.TempDir, ".local/share/PrismLauncher/instances/Old-Version-Inst/.minecraft/mods")
	_ = os.MkdirAll(instModsDir, 0755)

	// 2. Setup Forge Instance with MC 1.21.1 Forge
	forgeCfg := "name=Forge-Inst\nIntendedVersion=1.21.1\n"
	forgePack := `{"components":[{"uid":"net.minecraft","version":"1.21.1"},{"uid":"net.minecraftforge","version":"51.0.33"}]}`
	ctx.WriteFile(".local/share/PrismLauncher/instances/Forge-Inst/instance.cfg", forgeCfg)
	ctx.WriteFile(".local/share/PrismLauncher/instances/Forge-Inst/mmc-pack.json", forgePack)
	forgeModsDir := filepath.Join(ctx.TempDir, ".local/share/PrismLauncher/instances/Forge-Inst/.minecraft/mods")
	_ = os.MkdirAll(forgeModsDir, 0755)

	// 3. Setup Local Modpack (MC 1.21.1 Fabric)
	ctx.WriteFile("cmm.toml", `[profile]
name = "mismatch-pack"
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
`, ms.URL())
	ctx.WriteFile("cmm.lock", clientLock)

	// Scenario A: Minecraft version mismatch (1.20.1 instance vs 1.21.1 pack)
	// Without --force: MUST fail
	resVerMismatch := ctx.Run("launcher", "sync", "Old-Version-Inst")
	resVerMismatch.AssertFailure()
	resVerMismatch.AssertStderrContains("uses Minecraft 1.20.1, but cmm is configured for 1.21.1", "use --force to override")

	// With --force: MUST succeed
	resVerForced := ctx.Run("launcher", "sync", "Old-Version-Inst", "--force")
	resVerForced.AssertSuccess()
	if _, err := os.Stat(filepath.Join(instModsDir, "sodium-fabric-0.5.8.jar")); err != nil {
		t.Fatalf("expected forced sync to download mod into instance mods dir: %v", err)
	}

	// Scenario B: Mod Loader mismatch (Forge instance vs Fabric pack)
	// Without --force: MUST fail
	resLoaderMismatch := ctx.Run("launcher", "sync", "Forge-Inst")
	resLoaderMismatch.AssertFailure()
	resLoaderMismatch.AssertStderrContains("uses loader 'forge', but cmm is configured for 'fabric'", "use --force to override")

	// With --force: MUST succeed
	resLoaderForced := ctx.Run("launcher", "sync", "Forge-Inst", "--force")
	resLoaderForced.AssertSuccess()
	if _, err := os.Stat(filepath.Join(forgeModsDir, "sodium-fabric-0.5.8.jar")); err != nil {
		t.Fatalf("expected forced sync to download mod into forge instance mods dir: %v", err)
	}
}
