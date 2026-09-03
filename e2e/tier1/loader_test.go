package tier1

import (
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"testing"
)

func TestLoaderList_Fabric(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	res := ctx.Run("loader", "list", "fabric")
	res.AssertSuccess()
}

func TestLoaderList_AllLoaders(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	res := ctx.Run("loader", "list")
	res.AssertSuccess()
}

func TestLoaderInstall_Fabric(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.toml", "name = \"Test\"")

	res := ctx.Run("loader", "install", "fabric", "--version", "0.19.3")
	res.AssertSuccess()
}

func TestLoaderInstall_UnsupportedVersion(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	res := ctx.Run("loader", "install", "fabric", "--version", "99.9")
	res.AssertFailure()
	res.AssertStderrContains("version")
}

func TestLoaderInstall_ForgeOrNeoForge(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.toml", "name = \"Test\"")

	res := ctx.Run("loader", "install", "neoforge")
	res.AssertSuccess()
}

func TestLoaderUpdate_Success(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.toml", `[profile]
name = "Test"
loader = "fabric"
loader_version = "0.19.1"
`)

	res := ctx.RunWithStdin("y\n", "loader", "update")
	res.AssertSuccess()
	res.AssertStdoutContains("Successfully updated Fabric Loader to v0.19.3")
}

func TestLoaderUpdate_ServerRunningSafetyCheck(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.toml", `[profile]
name = "Test"
loader = "fabric"
loader_version = "0.19.1"
`)

	// Simulate running server process
	ctx.SetEnv("CMM_MOCK_SERVER_RUNNING", "1")

	// Without --force, must abort with safety error
	res := ctx.Run("loader", "update")
	res.AssertFailure()
	res.AssertStderrContains("Server is currently running")

	// With --force, should proceed
	resForce := ctx.RunWithStdin("y\n", "loader", "update", "--force")
	resForce.AssertSuccess()
	resForce.AssertStdoutContains("Successfully updated")
}
