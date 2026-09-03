package tier1

import (
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"testing"
)

func TestUpdate_AllModsInteractiveConfirm(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")
	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"sodium\"\nversion = \"0.4.0\"\n")

	res := ctx.RunWithStdin("y\n", "update")
	res.AssertSuccess()
	// Should update to latest from mockserver, maybe 0.5.0
}

func TestUpdate_SkipsPinnedMods(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")
	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"sodium\"\nversion = \"0.4.0\"\npinned = true\n[[mods]]\nname = \"lithium\"\nversion = \"0.11.0\"\n")

	res := ctx.Run("update")
	res.AssertSuccess()
	// Should show that sodium is skipped
	res.AssertStdoutContains("skipped")
}

func TestUpdate_ForcePinnedMod(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")
	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"sodium\"\nversion = \"0.4.0\"\npinned = true\n")

	res := ctx.Run("update", "sodium", "--force")
	res.AssertSuccess()
}

func TestUpdate_SingleSpecificMod(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")
	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"sodium\"\nversion = \"0.4.0\"\n[[mods]]\nname = \"lithium\"\nversion = \"0.11.0\"\n")

	res := ctx.Run("update", "lithium")
	res.AssertSuccess()
}

func TestUpdate_NoUpdatesAvailable(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")
	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"sodium\"\nversion = \"0.5.8\"\n")

	res := ctx.Run("update")
	res.AssertSuccess()
	res.AssertStdoutContains("up to date")
}

func TestUpdate_SlugDisplay_ChannelFilter_MultiSlug(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", `[profile]
name = "Test"
minecraft_version = "1.21.1"
loader = "fabric"
loader_version = "0.19.1"
`)
	ctx.WriteFile("cmm.lock", `[[mods]]
name = "Sodium"
slug = "sodium"
version = "0.4.0"

[[mods]]
name = "Lithium"
slug = "lithium"
version = "0.11.0"
`)

	res := ctx.RunWithStdin("y\n", "update", "sodium", "lithium", "--channel", "release")
	res.AssertSuccess()
	// Channel header
	res.AssertStdoutContains("[Channel: release]")
	// Slug in parenthesis
	res.AssertStdoutContains("Sodium (sodium)")
	res.AssertStdoutContains("Lithium (lithium)")
	// Notice of new loader
	res.AssertStdoutContains("Notice: A new Fabric Loader version is available (v0.19.3)")
}
