package tier1

import (
	"os"
	"path/filepath"
	"testing"

	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
)

func TestCLI_DisableAndEnable(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("mods/sodium-0.5.8.jar", "dummy-jar-binary")
	ctx.WriteFile("cmm.lock", `[[mods]]
name = "Sodium"
slug = "sodium"
version = "0.5.8"
file_name = "sodium-0.5.8.jar"
disabled = false
`)

	// 1. Disable Sodium
	res := ctx.Run("disable", "sodium")
	res.AssertSuccess()
	res.AssertStdoutContains("Successfully disabled Sodium")

	// Verify file on disk is renamed
	disabledPath := filepath.Join(ctx.TempDir, "mods", "sodium-0.5.8.jar.disabled")
	if _, err := os.Stat(disabledPath); err != nil {
		t.Fatalf("expected file %s to exist: %v", disabledPath, err)
	}

	// 2. Check cmm list output shows disabled
	resList := ctx.Run("list")
	resList.AssertSuccess()
	resList.AssertStdoutContains("disabled")

	// 3. Enable Sodium
	resEnable := ctx.Run("enable", "sodium")
	resEnable.AssertSuccess()
	resEnable.AssertStdoutContains("Successfully enabled Sodium")

	// Verify file on disk is restored
	restoredPath := filepath.Join(ctx.TempDir, "mods", "sodium-0.5.8.jar")
	if _, err := os.Stat(restoredPath); err != nil {
		t.Fatalf("expected file %s to exist: %v", restoredPath, err)
	}

	// 4. Check cmm list output shows enabled
	resList2 := ctx.Run("list")
	resList2.AssertSuccess()
	resList2.AssertStdoutContains("enabled")
}

func TestCLI_Disable_DryRun(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("mods/sodium-0.5.8.jar", "dummy-jar-binary")
	ctx.WriteFile("cmm.lock", `[[mods]]
name = "Sodium"
slug = "sodium"
version = "0.5.8"
file_name = "sodium-0.5.8.jar"
disabled = false
`)

	res := ctx.Run("disable", "sodium", "--dry-run")
	res.AssertSuccess()
	res.AssertStdoutContains("[Dry-Run] Would disable mod 'sodium'")

	// File should still be .jar
	jarPath := filepath.Join(ctx.TempDir, "mods", "sodium-0.5.8.jar")
	if _, err := os.Stat(jarPath); err != nil {
		t.Fatalf("expected file %s to still exist after dry-run: %v", jarPath, err)
	}
}
