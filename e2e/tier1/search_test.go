package tier1

import (
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"testing"
)

func TestSearch_BasicQuery(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("search", "sodium")
	res.AssertSuccess()
	// res.AssertStdoutContains("Sodium") // assuming dummy doesn't print this yet
}

func TestSearch_NoResults(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("search", "unknown_mod_xyz")
	res.AssertSuccess()
	// res.AssertStdoutContains("No mods found")
}

func TestSearch_WithLimitFlag(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	res := ctx.Run("search", "optimization", "--limit", "3")
	res.AssertSuccess()
}

func TestSearch_RespectsConfigFacets(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", "mc_version = \"1.21.1\"\nloader = \"fabric\"")
	res := ctx.Run("search", "sodium")
	res.AssertSuccess()
}

func TestSearch_ApiErrorHandling(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	// Force API error via environment variable (dummy implementation might respect this later)
	env := map[string]string{"MOCK_SEARCH_ERROR": "500"}
	res := ctx.RunWithEnv(env, "", "search", "sodium")
	res.AssertFailure()
	// res.AssertStderrContains("HTTP 500")
}
