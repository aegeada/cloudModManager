package tier1

import (
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"testing"
)

func TestRemove_SingleMod(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"lithium\"")
	res := ctx.Run("remove", "lithium")
	res.AssertSuccess()
}

func TestRemove_NotInstalledMod(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("remove", "unknown_mod")
	res.AssertFailure()
}

func TestRemove_OrphanedDependencyPrompt(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"sodium\"\n[[mods]]\nname = \"fabric-api\"")
	res := ctx.RunWithStdin("y\n", "remove", "sodium")
	res.AssertSuccess()
}

func TestRemove_SharedDependencyRetained(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"a\"\n[[mods]]\nname = \"b\"\n[[mods]]\nname = \"fabric-api\"")
	res := ctx.Run("remove", "a")
	res.AssertSuccess()
}

func TestRemove_DryRun(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"lithium\"")
	res := ctx.Run("remove", "lithium", "--dry-run")
	res.AssertSuccess()
}
