package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathResolver_Candidates(t *testing.T) {
	tmpDir := t.TempDir()
	opts := DetectorOptions{
		HomeDir:      tmpDir,
		AppData:      filepath.Join(tmpDir, "AppData", "Roaming"),
		LocalAppData: filepath.Join(tmpDir, "AppData", "Local"),
		OS:           "linux",
	}

	resolver := NewPathResolver(opts)

	// Linux candidates
	vanillaLinux := resolver.ResolveCandidatePaths(LauncherVanilla)
	if len(vanillaLinux) == 0 {
		t.Fatalf("expected vanilla linux candidate paths")
	}

	prismLinux := resolver.ResolveCandidatePaths(LauncherPrism)
	if len(prismLinux) == 0 {
		t.Fatalf("expected prism linux candidate paths")
	}

	modrinthLinux := resolver.ResolveCandidatePaths(LauncherModrinth)
	if len(modrinthLinux) == 0 {
		t.Fatalf("expected modrinth linux candidate paths")
	}

	curseLinux := resolver.ResolveCandidatePaths(LauncherCurseForge)
	if len(curseLinux) == 0 {
		t.Fatalf("expected curseforge linux candidate paths")
	}

	// Windows candidates
	optsWin := opts
	optsWin.OS = "windows"
	resolverWin := NewPathResolver(optsWin)
	vanillaWin := resolverWin.ResolveCandidatePaths(LauncherVanilla)
	if len(vanillaWin) == 0 || vanillaWin[0] != filepath.Join(opts.AppData, ".minecraft") {
		t.Errorf("unexpected windows vanilla path: %v", vanillaWin)
	}

	// Darwin candidates
	optsMac := opts
	optsMac.OS = "darwin"
	resolverMac := NewPathResolver(optsMac)
	vanillaMac := resolverMac.ResolveCandidatePaths(LauncherVanilla)
	if len(vanillaMac) == 0 || vanillaMac[0] != filepath.Join(tmpDir, "Library", "Application Support", "minecraft") {
		t.Errorf("unexpected mac vanilla path: %v", vanillaMac)
	}

	// Custom paths injection
	customDir := filepath.Join(tmpDir, "custom-prism")
	optsCustom := DetectorOptions{
		CustomPaths: map[LauncherType][]string{
			LauncherPrism: {customDir},
		},
	}
	resolverCustom := NewPathResolver(optsCustom)
	paths := resolverCustom.ResolveCandidatePaths(LauncherPrism)
	if len(paths) == 0 || paths[0] != customDir {
		t.Errorf("expected custom path '%s' first, got: %v", customDir, paths)
	}
}

func TestVanillaProfilesParser(t *testing.T) {
	tmpDir := t.TempDir()
	mcDir := filepath.Join(tmpDir, ".minecraft")
	if err := os.MkdirAll(mcDir, 0755); err != nil {
		t.Fatal(err)
	}

	customGameDir := filepath.Join(tmpDir, "custom-fabric-gamedir")
	if err := os.MkdirAll(customGameDir, 0755); err != nil {
		t.Fatal(err)
	}

	profilesData := `{
		"profiles": {
			"fabric-1-21-1": {
				"name": "Fabric 1.21.1",
				"lastVersionId": "fabric-loader-0.16.5-1.21.1",
				"gameDir": "` + customGameDir + `",
				"icon": "Furnace",
				"lastPlayed": "2024-08-31T12:00:00Z"
			},
			"quilt-1-21": {
				"name": "Quilt 1.21.1",
				"lastVersionId": "quilt-loader-0.26.1-1.21.1",
				"icon": "Crafting_Table"
			},
			"forge-1-20-1": {
				"name": "Forge 1.20.1",
				"lastVersionId": "1.20.1-forge-47.3.0"
			},
			"neoforge-1-21": {
				"name": "NeoForge 1.21.1",
				"lastVersionId": "1.21.1-neoforge-21.1.48"
			},
			"vanilla-release": {
				"name": "Latest 1.21.1",
				"lastVersionId": "1.21.1"
			}
		},
		"version": 3
	}`

	if err := os.WriteFile(filepath.Join(mcDir, "launcher_profiles.json"), []byte(profilesData), 0644); err != nil {
		t.Fatal(err)
	}

	instances, err := ParseVanillaProfiles(mcDir)
	if err != nil {
		t.Fatalf("ParseVanillaProfiles failed: %v", err)
	}

	if len(instances) != 5 {
		t.Fatalf("expected 5 instances, got %d", len(instances))
	}

	instMap := make(map[string]Instance)
	for _, inst := range instances {
		instMap[inst.ID] = inst
	}

	// Verify Fabric
	fab := instMap["fabric-1-21-1"]
	if fab.Name != "Fabric 1.21.1" || fab.Loader != "fabric" || fab.LoaderVersion != "0.16.5" || fab.MinecraftVersion != "1.21.1" {
		t.Errorf("unexpected fabric instance: %+v", fab)
	}
	if fab.InstanceDir != customGameDir || fab.ModsDir != filepath.Join(customGameDir, "mods") {
		t.Errorf("unexpected fabric instanceDir/modsDir: %s / %s", fab.InstanceDir, fab.ModsDir)
	}
	if fab.LastPlayed == nil || fab.LastPlayed.Year() != 2024 {
		t.Errorf("unexpected fabric lastPlayed: %v", fab.LastPlayed)
	}

	// Verify Quilt
	quilt := instMap["quilt-1-21"]
	if quilt.Loader != "quilt" || quilt.LoaderVersion != "0.26.1" || quilt.MinecraftVersion != "1.21.1" {
		t.Errorf("unexpected quilt instance: %+v", quilt)
	}
	if quilt.InstanceDir != mcDir {
		t.Errorf("expected default mcDir, got %s", quilt.InstanceDir)
	}

	// Verify Forge
	forge := instMap["forge-1-20-1"]
	if forge.Loader != "forge" || forge.LoaderVersion != "47.3.0" || forge.MinecraftVersion != "1.20.1" {
		t.Errorf("unexpected forge instance: %+v", forge)
	}

	// Verify NeoForge
	neoforge := instMap["neoforge-1-21"]
	if neoforge.Loader != "neoforge" || neoforge.LoaderVersion != "21.1.48" || neoforge.MinecraftVersion != "1.21.1" {
		t.Errorf("unexpected neoforge instance: %+v", neoforge)
	}

	// Verify Vanilla
	van := instMap["vanilla-release"]
	if van.Loader != "vanilla" || van.MinecraftVersion != "1.21.1" {
		t.Errorf("unexpected vanilla instance: %+v", van)
	}
}

func TestPrismInstancesParser(t *testing.T) {
	tmpDir := t.TempDir()
	prismDir := filepath.Join(tmpDir, "PrismLauncher")
	inst1Dir := filepath.Join(prismDir, "instances", "Cobblemon", ".minecraft")
	if err := os.MkdirAll(inst1Dir, 0755); err != nil {
		t.Fatal(err)
	}

	// Instance 1: Flatpak / standard layout (.minecraft/mods)
	cfg1 := "name=Cobblemon Adventure\nIntendedVersion=1.21.1\niconKey=cobblemon\nlastPlayed=1725100000000\n"
	if err := os.WriteFile(filepath.Join(prismDir, "instances", "Cobblemon", "instance.cfg"), []byte(cfg1), 0644); err != nil {
		t.Fatal(err)
	}

	pack1 := `{
		"components": [
			{"cachedName": "Minecraft", "cachedVersion": "1.21.1", "uid": "net.minecraft", "version": "1.21.1"},
			{"cachedName": "Fabric Loader", "cachedVersion": "0.16.5", "uid": "net.fabricmc.fabric-loader", "version": "0.16.5"}
		],
		"formatVersion": 1
	}`
	if err := os.WriteFile(filepath.Join(prismDir, "instances", "Cobblemon", "mmc-pack.json"), []byte(pack1), 0644); err != nil {
		t.Fatal(err)
	}

	// Instance 2: Grouped instance in subfolder (minecraft/mods layout)
	inst2Dir := filepath.Join(prismDir, "instances", "Modpacks", "ATM9", "minecraft")
	if err := os.MkdirAll(inst2Dir, 0755); err != nil {
		t.Fatal(err)
	}
	cfg2 := "name=All The Mods 9\nIntendedVersion=1.20.1\n"
	if err := os.WriteFile(filepath.Join(prismDir, "instances", "Modpacks", "ATM9", "instance.cfg"), []byte(cfg2), 0644); err != nil {
		t.Fatal(err)
	}
	pack2 := `{
		"components": [
			{"uid": "net.minecraft", "version": "1.20.1"},
			{"uid": "net.minecraftforge", "version": "47.3.0"}
		],
		"formatVersion": 1
	}`
	if err := os.WriteFile(filepath.Join(prismDir, "instances", "Modpacks", "ATM9", "mmc-pack.json"), []byte(pack2), 0644); err != nil {
		t.Fatal(err)
	}

	instances, err := ParsePrismInstances(prismDir)
	if err != nil {
		t.Fatalf("ParsePrismInstances failed: %v", err)
	}

	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}

	instMap := make(map[string]Instance)
	for _, inst := range instances {
		instMap[inst.Name] = inst
	}

	cobble := instMap["Cobblemon Adventure"]
	if cobble.Loader != "fabric" || cobble.LoaderVersion != "0.16.5" || cobble.MinecraftVersion != "1.21.1" {
		t.Errorf("unexpected cobblemon prism instance: %+v", cobble)
	}
	if cobble.ModsDir != filepath.Join(prismDir, "instances", "Cobblemon", ".minecraft", "mods") {
		t.Errorf("unexpected cobblemon modsDir: %s", cobble.ModsDir)
	}
	if cobble.LastPlayed == nil {
		t.Errorf("expected lastPlayed timestamp, got nil")
	}

	atm9 := instMap["All The Mods 9"]
	if atm9.Loader != "forge" || atm9.LoaderVersion != "47.3.0" || atm9.MinecraftVersion != "1.20.1" {
		t.Errorf("unexpected atm9 prism instance: %+v", atm9)
	}
	if atm9.ModsDir != filepath.Join(prismDir, "instances", "Modpacks", "ATM9", "minecraft", "mods") {
		t.Errorf("unexpected atm9 modsDir: %s", atm9.ModsDir)
	}
}

func TestModrinthProfilesParser(t *testing.T) {
	tmpDir := t.TempDir()
	modrinthDir := filepath.Join(tmpDir, "ModrinthApp")
	p1Dir := filepath.Join(modrinthDir, "profiles", "Fabulously-Optimized")
	if err := os.MkdirAll(p1Dir, 0755); err != nil {
		t.Fatal(err)
	}

	p1JSON := `{
		"name": "Fabulously Optimized",
		"game_version": "1.21.1",
		"loader": "fabric",
		"loader_version": "0.16.5",
		"icon": "icon.png",
		"last_played": 1725000000000
	}`
	if err := os.WriteFile(filepath.Join(p1Dir, "profile.json"), []byte(p1JSON), 0644); err != nil {
		t.Fatal(err)
	}

	p2Dir := filepath.Join(modrinthDir, "profiles", "Neoforge-Pack")
	if err := os.MkdirAll(p2Dir, 0755); err != nil {
		t.Fatal(err)
	}
	p2JSON := `{
		"name": "NeoForge World",
		"gameVersion": "1.21",
		"loader": "neoforge",
		"loaderVersion": "21.0.1",
		"lastPlayed": "2024-08-30T10:00:00Z"
	}`
	if err := os.WriteFile(filepath.Join(p2Dir, "profile.json"), []byte(p2JSON), 0644); err != nil {
		t.Fatal(err)
	}

	instances, err := ParseModrinthProfiles(modrinthDir)
	if err != nil {
		t.Fatalf("ParseModrinthProfiles failed: %v", err)
	}

	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}

	instMap := make(map[string]Instance)
	for _, inst := range instances {
		instMap[inst.Name] = inst
	}

	fab := instMap["Fabulously Optimized"]
	if fab.Loader != "fabric" || fab.LoaderVersion != "0.16.5" || fab.MinecraftVersion != "1.21.1" {
		t.Errorf("unexpected modrinth instance: %+v", fab)
	}
	if fab.ModsDir != filepath.Join(p1Dir, "mods") {
		t.Errorf("unexpected modsDir: %s", fab.ModsDir)
	}

	neo := instMap["NeoForge World"]
	if neo.Loader != "neoforge" || neo.LoaderVersion != "21.0.1" || neo.MinecraftVersion != "1.21" {
		t.Errorf("unexpected modrinth instance: %+v", neo)
	}
}

func TestCurseForgeInstancesParser(t *testing.T) {
	tmpDir := t.TempDir()
	curseDir := filepath.Join(tmpDir, "curseforge", "minecraft", "Instances")
	inst1Dir := filepath.Join(curseDir, "Better-MC")
	if err := os.MkdirAll(inst1Dir, 0755); err != nil {
		t.Fatal(err)
	}

	inst1JSON := `{
		"name": "Better MC [FABRIC]",
		"gameVersion": "1.21.1",
		"baseModLoader": {
			"name": "fabric-0.16.5",
			"type": 4,
			"minecraftVersion": "1.21.1"
		},
		"lastPlayed": "2024-08-30T10:00:00Z"
	}`
	if err := os.WriteFile(filepath.Join(inst1Dir, "minecraftinstance.json"), []byte(inst1JSON), 0644); err != nil {
		t.Fatal(err)
	}

	inst2Dir := filepath.Join(curseDir, "RLCraft")
	if err := os.MkdirAll(inst2Dir, 0755); err != nil {
		t.Fatal(err)
	}
	inst2JSON := `{
		"name": "RLCraft",
		"gameVersion": "1.12.2",
		"manifest": {
			"minecraft": {
				"version": "1.12.2",
				"modLoaders": [
					{"id": "forge-14.23.5.2860", "primary": true}
				]
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(inst2Dir, "minecraftinstance.json"), []byte(inst2JSON), 0644); err != nil {
		t.Fatal(err)
	}

	instances, err := ParseCurseForgeInstances(filepath.Dir(curseDir))
	if err != nil {
		t.Fatalf("ParseCurseForgeInstances failed: %v", err)
	}

	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}

	instMap := make(map[string]Instance)
	for _, inst := range instances {
		instMap[inst.Name] = inst
	}

	bmc := instMap["Better MC [FABRIC]"]
	if bmc.Loader != "fabric" || bmc.LoaderVersion != "0.16.5" || bmc.MinecraftVersion != "1.21.1" {
		t.Errorf("unexpected curseforge instance: %+v", bmc)
	}

	rl := instMap["RLCraft"]
	if rl.Loader != "forge" || rl.LoaderVersion != "14.23.5.2860" || rl.MinecraftVersion != "1.12.2" {
		t.Errorf("unexpected curseforge instance: %+v", rl)
	}
}

func TestDetector_FullTreeAndFindInstance(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Setup mock Vanilla
	mcDir := filepath.Join(tmpDir, ".minecraft")
	_ = os.MkdirAll(mcDir, 0755)
	vProfiles := `{
		"profiles": {
			"vanilla-survival": {
				"name": "Survival 1.21.1",
				"lastVersionId": "fabric-loader-0.16.5-1.21.1"
			}
		}
	}`
	_ = os.WriteFile(filepath.Join(mcDir, "launcher_profiles.json"), []byte(vProfiles), 0644)

	// 2. Setup mock Prism
	prismDir := filepath.Join(tmpDir, ".local", "share", "PrismLauncher")
	prismInstDir := filepath.Join(prismDir, "instances", "PrismFab", ".minecraft")
	_ = os.MkdirAll(prismInstDir, 0755)
	_ = os.WriteFile(filepath.Join(prismDir, "instances", "PrismFab", "instance.cfg"), []byte("name=Prism Fabric\nIntendedVersion=1.21.1\n"), 0644)
	pack := `{"components":[{"uid":"net.minecraft","version":"1.21.1"},{"uid":"net.fabricmc.fabric-loader","version":"0.16.5"}]}`
	_ = os.WriteFile(filepath.Join(prismDir, "instances", "PrismFab", "mmc-pack.json"), []byte(pack), 0644)

	// 3. Setup mock Modrinth
	modrinthDir := filepath.Join(tmpDir, ".config", "ModrinthApp")
	mProfDir := filepath.Join(modrinthDir, "profiles", "ModrinthFab")
	_ = os.MkdirAll(mProfDir, 0755)
	mProf := `{"name":"Modrinth Fabric","game_version":"1.21.1","loader":"fabric","loader_version":"0.16.5"}`
	_ = os.WriteFile(filepath.Join(mProfDir, "profile.json"), []byte(mProf), 0644)

	// 4. Setup mock CurseForge
	cfDir := filepath.Join(tmpDir, "curseforge", "minecraft", "Instances", "CurseFab")
	_ = os.MkdirAll(cfDir, 0755)
	cfJSON := `{"name":"CurseForge Fabric","gameVersion":"1.21.1","baseModLoader":{"name":"fabric-0.16.5"}}`
	_ = os.WriteFile(filepath.Join(cfDir, "minecraftinstance.json"), []byte(cfJSON), 0644)

	detector := NewDetector(DetectorOptions{
		HomeDir: tmpDir,
		OS:      "linux",
	})

	all, err := detector.DetectAll()
	if err != nil {
		t.Fatalf("DetectAll failed: %v", err)
	}

	if len(all) != 4 {
		t.Fatalf("expected 4 instances across 4 launchers, got %d", len(all))
	}

	// Test FindInstance exact match
	found, err := detector.FindInstance("Survival 1.21.1", LauncherVanilla)
	if err != nil || found.Name != "Survival 1.21.1" {
		t.Fatalf("failed to find vanilla instance: %v, found: %+v", err, found)
	}

	// Test FindInstance case-insensitive partial match
	foundPrism, err := detector.FindInstance("prismfab", LauncherAll)
	if err != nil || foundPrism.Name != "Prism Fabric" {
		t.Fatalf("failed to find prism instance: %v, found: %+v", err, foundPrism)
	}

	// Test FindInstance filtered by launcher
	foundModrinth, err := detector.FindInstance("Modrinth Fabric", LauncherModrinth)
	if err != nil || foundModrinth.Launcher != LauncherModrinth {
		t.Fatalf("failed to find modrinth instance: %v", err)
	}

	// Test FindInstance non-existent
	_, err = detector.FindInstance("NonExistent", LauncherAll)
	if err == nil {
		t.Errorf("expected error for non-existent instance, got nil")
	}

	// Test FindInstance empty query
	_, err = detector.FindInstance("", LauncherAll)
	if err == nil {
		t.Errorf("expected error for empty query, got nil")
	}
}

func TestDetector_AmbiguityHandling(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two instances with same name in two different launchers
	mcDir := filepath.Join(tmpDir, ".minecraft")
	_ = os.MkdirAll(mcDir, 0755)
	vProfiles := `{
		"profiles": {
			"cobblemon": {
				"name": "Cobblemon",
				"lastVersionId": "fabric-loader-0.16.5-1.21.1"
			}
		}
	}`
	_ = os.WriteFile(filepath.Join(mcDir, "launcher_profiles.json"), []byte(vProfiles), 0644)

	mProfDir := filepath.Join(tmpDir, ".config", "ModrinthApp", "profiles", "cobblemon")
	_ = os.MkdirAll(mProfDir, 0755)
	mProf := `{"name":"Cobblemon","game_version":"1.21.1","loader":"fabric","loader_version":"0.16.5"}`
	_ = os.WriteFile(filepath.Join(mProfDir, "profile.json"), []byte(mProf), 0644)

	detector := NewDetector(DetectorOptions{
		HomeDir: tmpDir,
		OS:      "linux",
	})

	// Ambiguous search across all launchers
	_, err := detector.FindInstance("Cobblemon", LauncherAll)
	if err == nil {
		t.Fatalf("expected ambiguity error when searching for duplicate name 'Cobblemon', got nil")
	}

	// Disambiguated by launcher type
	vanillaInst, err := detector.FindInstance("Cobblemon", LauncherVanilla)
	if err != nil || vanillaInst.Launcher != LauncherVanilla {
		t.Fatalf("expected vanilla instance, got: %v", err)
	}

	modrinthInst, err := detector.FindInstance("Cobblemon", LauncherModrinth)
	if err != nil || modrinthInst.Launcher != LauncherModrinth {
		t.Fatalf("expected modrinth instance, got: %v", err)
	}
}
