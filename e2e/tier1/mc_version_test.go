package tier1

import (
	"path/filepath"
	"testing"

	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"cmm/internal/config"
)

func TestMCVersion_Display(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", `[profile]
name = "Test"
minecraft_version = "26.2"
loader = "fabric"
`)

	res := ctx.Run("mc-version")
	res.AssertSuccess()
	res.AssertStdoutContains("Configured Minecraft version: 26.2")
}

func TestMCVersion_UpdateWithConfirmation(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", `[profile]
name = "Test"
minecraft_version = "26.2"
loader = "fabric"
`)

	res := ctx.RunWithStdin("y\n", "mc-version", "26.1.2")
	res.AssertSuccess()
	res.AssertStdoutContains("Minecraft version updated to 26.1.2 in cmm.toml.")

	cfg, err := config.LoadConfig(filepath.Join(ctx.TempDir, "cmm.toml"))
	if err != nil {
		t.Fatalf("failed to load cmm.toml: %v", err)
	}
	if cfg.Profile.MinecraftVersion != "26.1.2" {
		t.Errorf("expected 26.1.2, got %s", cfg.Profile.MinecraftVersion)
	}
}

func TestMCVersion_InvalidFormat(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", `[profile]
name = "Test"
minecraft_version = "26.2"
loader = "fabric"
`)

	res := ctx.Run("mc-version", "invalid_ver_string")
	res.AssertFailure()
	res.AssertStderrContains("invalid Minecraft version format")
}
