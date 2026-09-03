package tier1

import (
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"testing"
)

func TestSyncLocal_ScansAndIdentifiesJars(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("mods/sodium.jar", "mock jar data 1")
	ctx.WriteFile("mods/lithium.jar", "mock jar data 2")

	res := ctx.Run("sync", "local")
	res.AssertSuccess()
	// Lockfile should have been generated
	lock := ctx.ReadFile("cmm.lock")
	if lock == "" {
		t.Fatal("expected cmm.lock to be created")
	}
}

func TestSyncLocal_UnrecognizedJars(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("mods/known.jar", "known")
	ctx.WriteFile("mods/unknown.jar", "unknown")

	res := ctx.Run("sync", "local")
	res.AssertSuccess()
	res.AssertStdoutContains("unknown")
}

func TestSyncLocal_EmptyDirectory(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	res := ctx.Run("sync", "local")
	res.AssertSuccess()
	res.AssertStdoutContains("No mods found")
}

func TestSyncLocal_InvalidPath(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	res := ctx.Run("sync", "local", "--path", "./not_exist")
	res.AssertFailure()
}

func TestSyncLocal_PreservesExistingPins(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.lock", `[mods.sodium]
pinned = true`)
	ctx.WriteFile("mods/sodium.jar", "mock jar data")

	res := ctx.Run("sync", "local")
	res.AssertSuccess()
	res.AssertLockfileMod("sodium", "", true)
}
