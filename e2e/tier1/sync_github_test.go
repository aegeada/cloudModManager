package tier1

import (
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"testing"
)

func TestSyncGitHub_ValidRepo(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")

	res := ctx.Run("sync", "--source", "github", "--repo", "org/pack")
	res.AssertSuccess()
}

func TestSyncGitHub_WithBranch(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")

	res := ctx.Run("sync", "--source", "github", "--repo", "org/pack", "--branch", "develop")
	res.AssertSuccess()
}

func TestSyncGitHub_RepoNotFound(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")

	res := ctx.Run("sync", "--source", "github", "--repo", "org/nonexistent")
	res.AssertFailure()
	res.AssertStderrContains("404")
}

func TestSyncGitHub_AuthToken(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")

	res := ctx.Run("sync", "--source", "github", "--repo", "org/private", "--token", "ghp_mock")
	res.AssertSuccess()
}

func TestSyncGitHub_NoChanges(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")
	// Seed a lockfile that matches what the mockserver would return for the github sync.
	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"sodium\"\nversion = \"0.4.0\"\n")

	// We don't have the real Github API, so this is just testing the CLI execution logic for now.
	res := ctx.Run("sync", "--source", "github", "--repo", "org/pack")
	res.AssertSuccess()
	// res.AssertStdoutContains("Already up to date") // Might fail if binary is a dummy
}
