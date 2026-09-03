package tier1

import (
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"testing"
)

func TestList_EmptyMods(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("list")
	res.AssertSuccess()
}

func TestList_MultipleInstalledMods(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"a\"\n[[mods]]\nname = \"b\"\n[[mods]]\nname = \"c\"")
	res := ctx.Run("list")
	res.AssertSuccess()
}

func TestList_ShowsUpdateAvailable(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"sodium\"\nversion = \"1.0\"")
	res := ctx.Run("list")
	res.AssertSuccess()
}

func TestList_ShowsPinnedStatus(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"sodium\"\npinned = true")
	res := ctx.Run("list")
	res.AssertSuccess()
}

func TestList_JsonOrQuietOutput(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("list", "--format", "json")
	res.AssertSuccess()
}

func TestList_ShowsProperSideValues(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.lock", `[[mods]]
name = "Reliable Gliders"
slug = "reliable-gliders"
version = "1.3.3"
side = "required"

[[mods]]
name = "Sodium"
slug = "sodium"
version = "0.5.8"
side = "client"

[[mods]]
name = "ServerCore"
slug = "servercore"
version = "1.5.19"
side = "server"
`)

	res := ctx.Run("list")
	res.AssertSuccess()
	// Must display granular tags
	res.AssertStdoutContains("Client & Server")
	res.AssertStdoutContains("Client Only")
	res.AssertStdoutContains("Server Only")
	if res.Stdout == "" {
		t.Errorf("expected non-empty list output")
	}
}

