package tier2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
)

func TestSelfInstall_Command(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	// Test manual self-install command
	res := ctx.Run("self-install")
	res.AssertSuccess()
	res.AssertStdoutContains("Successfully installed cmm to")

	// Verify ~/.local/bin/cmm exists
	installedPath := filepath.Join(ctx.TempDir, ".local", "bin", "cmm")
	if _, err := os.Stat(installedPath); err != nil {
		t.Fatalf("expected installed cmm binary at %s: %v", installedPath, err)
	}
}

func TestSelfInstall_AutomaticOnFirstRun(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	// Enable auto self-install explicitly by overriding CMM_NO_SELF_INSTALL=""
	env := map[string]string{
		"CMM_NO_SELF_INSTALL": "",
		"HOME":                ctx.TempDir,
		"PATH":                "/usr/bin:/bin", // without ~/.local/bin
	}

	// Create a dummy ~/.bashrc
	bashrcPath := filepath.Join(ctx.TempDir, ".bashrc")
	_ = os.WriteFile(bashrcPath, []byte("# existing bashrc\n"), 0644)

	res := ctx.RunWithEnv(env, "", "--help")
	res.AssertSuccess()
	res.AssertStdoutContains("[Auto-Setup] Successfully installed 'cmm'")

	// Verify ~/.local/bin/cmm exists
	installedPath := filepath.Join(ctx.TempDir, ".local", "bin", "cmm")
	if _, err := os.Stat(installedPath); err != nil {
		t.Fatalf("expected auto-installed binary at %s: %v", installedPath, err)
	}

	// Verify ~/.bashrc was updated with PATH export
	data, err := os.ReadFile(bashrcPath)
	if err != nil || !strings.Contains(string(data), ".local/bin") {
		t.Fatalf("expected PATH export to be added to ~/.bashrc, got: %s", string(data))
	}
}
