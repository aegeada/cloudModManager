package tier2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"cmm/internal/config"
)

// 1. Test Human-Readable Optional Dependency Resolution
func TestAdd_OptionalDependencyHumanReadableNames(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", `[profile]
name = "GliderTest"
minecraft_version = "26.2"
loader = "fabric"
side = "both"
`)

	res := ctx.Run("add", "reliable-gliders")
	res.AssertSuccess()
	res.AssertStdoutContains("Successfully installed Reliable Gliders")

	// Must resolve to human readable mod names and slugs, not raw IDs
	res.AssertStdoutContains("Notice: Optional dependency available: Cloth Config v13 (cloth-config)")
	res.AssertStdoutContains("Notice: Optional dependency available: Architectury API (architectury-api)")
	res.AssertStdoutContains("Notice: Optional dependency available: Cloth Config API (cloth-config-api)")

	// Ensure raw IDs are NOT printed standalone without names
	if strings.Contains(res.Stdout, "Notice: Optional dependency available: 5aaWibi9\n") {
		t.Errorf("expected human readable name instead of raw ID 5aaWibi9")
	}
}

// 2. Realistic Server Directory Environment Auto-Detection & Scanning
func TestScan_ServerDirectoryAutoDetection(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	// Simulate existing real Minecraft server layout
	_ = os.WriteFile(filepath.Join(ctx.TempDir, "fabric-server-launcher.properties"), []byte("serverJar=server.jar\n"), 0644)
	_ = os.WriteFile(filepath.Join(ctx.TempDir, "fabric-loader-0.19.3-1.21.1.jar"), []byte("fabric-jar"), 0644)
	_ = os.WriteFile(filepath.Join(ctx.TempDir, "server.jar"), []byte("server-jar"), 0644)

	// Create config directory
	cfgDir := filepath.Join(ctx.TempDir, "config")
	_ = os.MkdirAll(cfgDir, 0755)
	_ = os.WriteFile(filepath.Join(cfgDir, "sodium-options.json"), []byte("{}"), 0644)

	// Create mods directory with 1 recognized mod and 1 custom mod
	modsDir := filepath.Join(ctx.TempDir, "mods")
	_ = os.MkdirAll(modsDir, 0755)
	_ = os.WriteFile(filepath.Join(modsDir, "sodium-fabric-0.5.8.jar"), mockserver.MockJarContent, 0644)
	_ = os.WriteFile(filepath.Join(modsDir, "MyCustomPrivateMod.jar"), []byte("private-mod-binary-data"), 0644)

	// Execute scan on current directory
	res := ctx.Run("scan", ".")
	res.AssertSuccess()
	res.AssertStdoutContains("Auto-detected Environment -> Loader: fabric, Minecraft: 1.21.1")
	res.AssertStdoutContains("Recognized Mods Added/Updated (1):")
	res.AssertStdoutContains("✓ Sodium")
	res.AssertStdoutContains("Unrecognized / Custom JARs (1):")
	res.AssertStdoutContains("⚠️  MyCustomPrivateMod.jar")
	res.AssertStdoutContains("Config directory detected")

	// Verify cmm.toml was created
	cfg, err := config.LoadConfig(filepath.Join(ctx.TempDir, "cmm.toml"))
	if err != nil {
		t.Fatalf("expected cmm.toml to be created by scan: %v", err)
	}
	if cfg.Profile.Loader != "fabric" || cfg.Profile.MinecraftVersion != "1.21.1" {
		t.Errorf("unexpected profile: loader=%s, mc=%s", cfg.Profile.Loader, cfg.Profile.MinecraftVersion)
	}

	// Verify cmm.lock was created and contains sodium
	lock, err := config.LoadLockfile(filepath.Join(ctx.TempDir, "cmm.lock"))
	if err != nil {
		t.Fatalf("expected cmm.lock to be created by scan: %v", err)
	}
	if lock.GetMod("sodium") == nil {
		t.Errorf("expected sodium to be in cmm.lock")
	}
}

// 3. Realistic Init Auto-Scan of Pre-existing Mods
func TestInit_ExistingModsAutoScan(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	modsDir := filepath.Join(ctx.TempDir, "mods")
	_ = os.MkdirAll(modsDir, 0755)
	_ = os.WriteFile(filepath.Join(modsDir, "sodium-fabric-0.5.8.jar"), mockserver.MockJarContent, 0644)

	res := ctx.Run("init", "--name", "AutoScanServer", "--mc-version", "1.21.1", "--loader", "fabric", "--side", "both")
	res.AssertSuccess()
	res.AssertStdoutContains("Detected 1 existing JAR file(s) in 'mods'")
	res.AssertStdoutContains("Successfully scanned and matched 1 mod(s) in cmm.lock")

	lock, err := config.LoadLockfile(filepath.Join(ctx.TempDir, "cmm.lock"))
	if err != nil || lock.GetMod("sodium") == nil {
		t.Fatalf("expected sodium to be present in cmm.lock after cmm init auto-scan")
	}
}
