package tier1

import (
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"strings"
	"testing"
)

func TestPin_CurrentVersion(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"")
	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"sodium\"\nversion = \"0.5.0\"\npinned = false\n")

	res := ctx.Run("pin", "sodium")
	res.AssertSuccess()
	
	lockfile := ctx.ReadFile("cmm.lock")
	if !strings.Contains(lockfile, "pinned = true") {
		t.Fatalf("expected pinned = true in lockfile")
	}
}

func TestPin_SpecificVersion(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"")
	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"sodium\"\nversion = \"0.5.0\"\npinned = false\n")

	res := ctx.Run("pin", "sodium", "--version", "0.5.3")
	res.AssertSuccess()
	
	lockfile := ctx.ReadFile("cmm.lock")
	if !strings.Contains(lockfile, "0.5.3") || !strings.Contains(lockfile, "pinned = true") {
		t.Fatalf("expected version 0.5.3 and pinned = true in lockfile")
	}
}

func TestUnpin_PinnedMod(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"")
	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"sodium\"\nversion = \"0.5.0\"\npinned = true\n")

	res := ctx.Run("unpin", "sodium")
	res.AssertSuccess()
	
	lockfile := ctx.ReadFile("cmm.lock")
	if !strings.Contains(lockfile, "pinned = false") {
		t.Fatalf("expected pinned = false in lockfile")
	}
}

func TestPin_UninstalledMod(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"")
	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"sodium\"\nversion = \"0.5.0\"\n")

	res := ctx.Run("pin", "nonexistent")
	res.AssertFailure()
	res.AssertStderrContains("Mod not found")
}

func TestUnpin_AlreadyUnpinnedMod(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())
	
	ctx.WriteFile("cmm.toml", "name = \"Test\"")
	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"sodium\"\nversion = \"0.5.0\"\npinned = false\n")

	res := ctx.Run("unpin", "sodium")
	res.AssertSuccess()
	res.AssertStdoutContains("Already unpinned")
}
