package tier1

import (
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"testing"
)

func TestSyncModrinth_FromSlug(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")

	res := ctx.Run("sync", "--source", "modrinth", "--slug", "pack")
	res.AssertSuccess()
}

func TestSyncModrinth_FromFile(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")
	ctx.WriteBytes("pack.mrpack", mockserver.GetMockMrpackContent())

	res := ctx.Run("sync", "--source", "modrinth", "--file", "pack.mrpack")
	res.AssertSuccess()
}

func TestSyncModrinth_SideFiltering(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"\nside = \"server\"")

	res := ctx.Run("sync", "--source", "modrinth", "--slug", "pack")
	res.AssertSuccess()
}

func TestSyncModrinth_DeltaRemoval(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")
	ctx.WriteFile("mods/extraneous.jar", "dummy content")

	res := ctx.Run("sync", "--source", "modrinth", "--slug", "pack")
	res.AssertSuccess()
}

func TestSyncModrinth_InvalidSlugOrFile(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")

	res := ctx.Run("sync", "--source", "modrinth", "--slug", "bad404")
	res.AssertFailure()
}
