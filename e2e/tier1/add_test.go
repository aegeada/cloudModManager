package tier1

import (
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"testing"
)

func TestAdd_SingleModNoDeps(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("add", "lithium")
	res.AssertSuccess()
}

func TestAdd_WithRequiredDependencies(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("add", "sodium")
	res.AssertSuccess()
}

func TestAdd_ServerSideFiltersClientOnly(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", "side = \"server\"")
	res := ctx.Run("add", "iris")
	res.AssertSuccess()
}

func TestAdd_AlreadyInstalled(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"lithium\"")
	res := ctx.Run("add", "lithium")
	res.AssertSuccess()
}

func TestAdd_NonExistentMod(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("add", "non_existent_slug_404")
	res.AssertFailure()
}

func TestInstall_AliasAndPaging(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.RunWithStdin("1\ny\n", "install", "sodium")
	res.AssertSuccess()
	res.AssertStdoutContains("Successfully installed Sodium")
}

func TestInstall_SpecificVersionTargeting(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.RunWithStdin("y\n", "install", "sodium", "-v", "0.5.8")
	res.AssertSuccess()
	res.AssertStdoutContains("Successfully installed Sodium (0.5.8)")
}

func TestInstall_MultiModBatch(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.RunWithStdin("y\n", "install", "sodium", "lithium")
	res.AssertSuccess()
	res.AssertStdoutContains("Preparing to install 2 mod(s)")
	res.AssertStdoutContains("Successfully installed")
}

func TestInstall_AlreadyInstalledReplacement(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.lock", `[[mods]]
name = "Sodium"
slug = "sodium"
version = "0.4.0"
file_name = "sodium-0.4.0.jar"
`)
	ctx.WriteFile("mods/sodium-0.4.0.jar", "old-jar-binary")

	res := ctx.RunWithStdin("y\n", "install", "sodium", "-v", "0.5.8")
	res.AssertSuccess()
	res.AssertStdoutContains("Successfully replaced Sodium with version 0.5.8")
}
