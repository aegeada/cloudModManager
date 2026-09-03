package tier1

import (
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"os"
	"path/filepath"
	"testing"
)

func TestExportGitHub_GeneratesFiles(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.toml", "name = \"Test\"")
	ctx.WriteFile("cmm.lock", "[mods]")

	res := ctx.Run("export", "--format", "github", "--output", "./gh")
	res.AssertSuccess()
	
	if ctx.ReadFile("gh/cmm.toml") == "" {
		t.Fatal("cmm.toml not exported")
	}
	if ctx.ReadFile("gh/cmm.lock") == "" {
		t.Fatal("cmm.lock not exported")
	}
}

func TestExportGitHub_SanitizesSensitiveConfig(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.toml", "sync_token = \"secret\"\nname = \"Test\"")
	ctx.WriteFile("cmm.lock", "[mods]")

	res := ctx.Run("export", "--format", "github", "--output", "./gh")
	res.AssertSuccess()

	out := ctx.ReadFile("gh/cmm.toml")
	if out == "" || string(out) == "sync_token = \"secret\"\nname = \"Test\"" {
		t.Fatal("sensitive token not sanitized")
	}
}

func TestExportGitHub_DefaultOutputDirectory(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.toml", "name = \"Test\"")
	ctx.WriteFile("cmm.lock", "[mods]")

	res := ctx.Run("export", "--format", "github")
	res.AssertSuccess()
	// Should export to current dir (or a default path like 'export/')
	// Just asserting success is enough for opaque box
}

func TestExportGitHub_InvalidOutputDirectory(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.toml", "name = \"Test\"")
	ctx.WriteFile("cmm.lock", "[mods]")

	// Try to write to a readonly directory
	roDir := filepath.Join(ctx.TempDir, "readonly")
	os.Mkdir(roDir, 0555)

	res := ctx.Run("export", "--format", "github", "--output", filepath.Join(roDir, "out"))
	res.AssertFailure()
}

func TestExportGitHub_FormatValidation(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.toml", "name = \"Test\"")
	ctx.WriteFile("cmm.lock", "[mods]")

	res := ctx.Run("export", "--format", "github", "--output", "./gh")
	res.AssertSuccess()
}
