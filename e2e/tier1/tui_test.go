package tier1

import (
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"testing"
)

func TestTUI_LaunchAndQuit(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", "name = \"Test\"")

	res := ctx.RunWithStdin("q", "tui")
	res.AssertSuccess()
}

func TestTUI_HelpFlag(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("tui", "--help")
	res.AssertSuccess()
	res.AssertStdoutContains("keybindings")
}

func TestTUI_NoConfigError(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("tui")
	res.AssertFailure()
	res.AssertStderrContains("cmm init")
}

func TestTUI_ConfigEditorTab(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", "name = \"Test\"")

	// Simulate navigation to Config tab, e.g. pressing Tab, then q to quit
	res := ctx.RunWithStdin("\tq", "tui")
	res.AssertSuccess()
}

func TestTUI_NonInteractiveDetection(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", "name = \"Test\"")

	// Running with empty stdin triggers non-interactive check
	res := ctx.RunWithStdin("", "tui")
	res.AssertStdoutContains("non-interactive")
}

func TestTUI_WithInstalledMods(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", "name = \"Test\"\nminecraft_version = \"1.21.1\"\nloader = \"fabric\"")
	ctx.WriteFile("cmm.lock", "[[mods]]\nslug = \"sodium\"\nname = \"Sodium\"\nversion = \"0.5.8\"\npinned = true")

	// Navigate with 'j', view details with Enter, close with Esc, quit with 'q'
	res := ctx.RunWithStdin("j\r\x1bq", "tui")
	res.AssertSuccess()
}
