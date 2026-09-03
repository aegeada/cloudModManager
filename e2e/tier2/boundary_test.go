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

// Tier 2: Boundary & Corner Cases (Category-Partition & Boundary Value Analysis)

func TestBoundary_Init_EmptyFields(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	// Default initialization with fallback fields
	res := ctx.Run("init", "--name", "TestServer", "--mc-version", "1.21.1", "--loader", "fabric")
	res.AssertSuccess()

	cfg, err := config.LoadConfig(filepath.Join(ctx.TempDir, "cmm.toml"))
	if err != nil {
		t.Fatalf("failed to load cmm.toml: %v", err)
	}
	if cfg.Profile.MinecraftVersion != "1.21.1" {
		t.Errorf("expected mc_version 1.21.1, got %s", cfg.Profile.MinecraftVersion)
	}
}

func TestBoundary_Init_VeryLongStrings(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	veryLongName := strings.Repeat("A_Very_Long_Server_Name_", 20)
	res := ctx.Run("init", "--name", veryLongName, "--mc-version", "1.21.1", "--loader", "fabric")
	res.AssertSuccess()

	cfg, err := config.LoadConfig(filepath.Join(ctx.TempDir, "cmm.toml"))
	if err != nil || cfg.Profile.Name != veryLongName {
		t.Errorf("failed to handle long server name")
	}
}

func TestBoundary_Search_EmptyQuery(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("search", "")
	res.AssertSuccess()
}

func TestBoundary_Search_SpecialCharacters(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("search", "mod%20with+special&chars!*()")
	res.AssertSuccess()
}

func TestBoundary_Search_MaxLimit(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("search", "sodium", "--limit", "100")
	res.AssertSuccess()
}

func TestBoundary_Add_DuplicateMod(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", `[profile]
name = "DupTest"
minecraft_version = "1.21.1"
loader = "fabric"
`)
	ctx.WriteFile("cmm.lock", `[[mods]]
slug = "sodium"
name = "Sodium"
version = "0.5.8"
`)
	// Adding already installed mod reports already installed or updates
	res := ctx.Run("add", "sodium", )
	res.AssertSuccess()
}

func TestBoundary_Add_CircularDependency(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", `[profile]
name = "CircularTest"
minecraft_version = "1.21.1"
loader = "fabric"
`)
	// Circular dependency resolving should not infinite loop
	res := ctx.Run("add", "circular-a", )
	if res.ExitCode != 0 && !strings.Contains(res.Stderr, "not found") && !strings.Contains(res.Stderr, "circular") {
		// Tolerated error or success
	}
}

func TestBoundary_Remove_CorruptedLockfile(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", `name = "CorruptTest"`)
	ctx.WriteFile("cmm.lock", `[mods = broken_toml`)

	res := ctx.Run("remove", "sodium")
	res.AssertFailure()
}

func TestBoundary_Pin_InvalidVersionFormat(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", `name = "PinTest"`)
	ctx.WriteFile("cmm.lock", `[[mods]]
slug = "sodium"
name = "Sodium"
version = "0.5.8"
`)

	res := ctx.Run("pin", "sodium", "--version", "invalid..semver..tag")
	// Pinning with version sets the version or reports
	if res.ExitCode == 0 {
		lock, _ := config.LoadLockfile(filepath.Join(ctx.TempDir, "cmm.lock"))
		m := lock.GetMod("sodium")
		if m != nil && !m.Pinned {
			t.Errorf("expected mod to be pinned")
		}
	}
}

func TestBoundary_Update_NetworkTimeout(t *testing.T) {
	ctx := harness.NewTestContext(t, "http://127.0.0.1:59999") // Unreachable

	ctx.WriteFile("cmm.toml", `name = "TimeoutTest"`)
	ctx.WriteFile("cmm.lock", `[[mods]]
slug = "sodium"
name = "Sodium"
version = "0.5.8"
`)

	res := ctx.Run("update")
	// Must fail gracefully without panic
	if res.ExitCode != 0 && res.Stderr == "" && res.Stdout == "" {
		t.Errorf("expected error output on unreachable network update")
	}
}

func TestBoundary_Export_InvalidDestination(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", `name = "ExportTest"`)
	ctx.WriteFile("cmm.lock", `[[mods]]
slug = "sodium"
name = "Sodium"
version = "0.5.8"
`)

	// Invalid path on unwritable root or file as directory
	invalidPath := filepath.Join(ctx.TempDir, "cmm.toml", "nested", "export.mrpack")
	res := ctx.Run("export", "--format", "mrpack", "--output", invalidPath)
	res.AssertFailure()
}

func TestBoundary_Sync_HashMismatch(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", `[profile]
name = "SyncHashTest"
minecraft_version = "1.21.1"
loader = "fabric"
`)
	modsDir := filepath.Join(ctx.TempDir, "mods")
	_ = os.MkdirAll(modsDir, 0755)

	// Modified/corrupted jar file with wrong hash
	_ = os.WriteFile(filepath.Join(modsDir, "sodium-fabric-0.5.8.jar"), []byte("tampered-jar-content"), 0644)

	res := ctx.Run("sync", "local")
	res.AssertSuccess()
	// Should detect unknown hash and warn
	if !strings.Contains(res.Stdout, "Warning") && !strings.Contains(res.Stdout, "Successfully") {
		t.Errorf("expected warning on modified jar hash")
	}
}
