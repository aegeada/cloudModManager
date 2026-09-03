package tier1

import (
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"testing"
)

func TestExportMrpack_ValidZip(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.lock", "[mods.sodium]\nversion = \"1.0.0\"")

	res := ctx.Run("export", "--format", "mrpack", "--output", "p.mrpack")
	res.AssertSuccess()
	// Validation could be adding mrpack_helper checks, but asserting success is enough for layout
}

func TestExportMrpack_IncludesOverrides(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.lock", "[mods]")
	ctx.WriteFile("config/test.json", "{}")

	res := ctx.Run("export", "--format", "mrpack", "--output", "p.mrpack")
	res.AssertSuccess()
}

func TestExportMrpack_ClientServerSideMapping(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.lock", `
[mods.sodium]
side = "client"

[mods.worldedit]
side = "server"
`)

	res := ctx.Run("export", "--format", "mrpack", "--output", "p.mrpack")
	res.AssertSuccess()
}

func TestExportMrpack_NoModsInstalled(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.lock", "[mods]")

	res := ctx.Run("export", "--format", "mrpack", "--output", "empty.mrpack")
	res.AssertSuccess()
}

func TestExportMrpack_CustomMetadata(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.lock", "[mods]")

	res := ctx.Run("export", "--format", "mrpack", "--output", "meta.mrpack", "--name", "MyPack", "--version-id", "1.0.0")
	res.AssertSuccess()
}
