package tier1

import (
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"testing"
)

func TestInit_DefaultInteractive(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.RunWithStdin("srv\n1.21.1\nfabric\nserver\n", "init")
	res.AssertSuccess()
	ctx.ReadFile("cmm.toml")
}

func TestInit_NonInteractiveFlags(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("init", "--name", "Srv", "--mc-version", "1.21.1", "--loader", "fabric", "--side", "server")
	res.AssertSuccess()
	ctx.ReadFile("cmm.toml")
}

func TestInit_AlreadyInitialized(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", "name = \"Srv\"")
	res := ctx.Run("init")
	res.AssertFailure()
}

func TestInit_InvalidMinecraftVersion(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("init", "--mc-version", "invalid-version")
	res.AssertFailure()
	res.AssertStderrContains("Validation error") // or similar
}

func TestInit_CustomPaths(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("init", "--mods-dir", "my_mods", "--config-dir", "my_cfg")
	res.AssertSuccess()
	ctx.ReadFile("cmm.toml")
}
