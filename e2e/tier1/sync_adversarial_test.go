package tier1

import (
	"archive/zip"
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createTestZip(t *testing.T, destPath string, files map[string][]byte) {
	t.Helper()
	_ = os.MkdirAll(filepath.Dir(destPath), 0755)
	f, err := os.Create(destPath)
	if err != nil {
		t.Fatalf("failed to create zip %s: %v", destPath, err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create file %s in zip: %v", name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("failed to write %s in zip: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}
}

// 1. R1 Local Sync Adversarial E2E Tests
func TestCLI_SyncLocal_Adversarial(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	// Subtest 1: Empty mods directory -> "No mods found"
	res := ctx.Run("sync", "local")
	res.AssertSuccess()
	res.AssertStdoutContains("No mods found")

	// Subtest 2: Non-existent explicit path -> exit code 1
	res = ctx.Run("sync", "local", "--path", "./non_existent_dir_12345")
	res.AssertFailure(1)
	res.AssertStderrContains("does not exist")

	// Subtest 3: Path is a regular file -> exit code 1
	ctx.WriteFile("a_file.txt", "not a directory")
	res = ctx.Run("sync", "local", "--path", "a_file.txt")
	res.AssertFailure(1)
	res.AssertStderrContains("not a directory")

	// Subtest 4: Unknown / unrecognized JAR files
	ctx.WriteFile("mods/unrecognized_mod_99.jar", "unknown")
	res = ctx.Run("sync", "local")
	res.AssertSuccess()
	res.AssertStdoutContains("Warning: Unrecognized JAR file: unrecognized_mod_99.jar (unknown)")
}

// 2. R2 Modrinth / .mrpack Sync Adversarial E2E Tests
func TestCLI_SyncModrinth_Adversarial(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"\nside = \"server\"")

	// Subtest 1: Corrupted .mrpack (not a zip file)
	ctx.WriteFile("corrupt.mrpack", "PLAIN_TEXT_NOT_A_ZIP_ARCHIVE")
	res := ctx.Run("sync", "--source", "modrinth", "--file", "corrupt.mrpack")
	res.AssertFailure(1)
	res.AssertStderrContains("invalid mrpack zip archive")

	// Subtest 2: Valid ZIP missing modrinth.index.json
	missingIndexZip := filepath.Join(ctx.TempDir, "missing_index.mrpack")
	createTestZip(t, missingIndexZip, map[string][]byte{
		"README.txt": []byte("where is index?"),
	})
	res = ctx.Run("sync", "--source", "modrinth", "--file", "missing_index.mrpack")
	res.AssertFailure(1)
	res.AssertStderrContains("modrinth.index.json not found")

	// Subtest 3: Valid ZIP with malformed JSON
	malformedIndexZip := filepath.Join(ctx.TempDir, "malformed_index.mrpack")
	createTestZip(t, malformedIndexZip, map[string][]byte{
		"modrinth.index.json": []byte("{ invalid json syntax !!!"),
	})
	res = ctx.Run("sync", "--source", "modrinth", "--file", "malformed_index.mrpack")
	res.AssertFailure(1)
	res.AssertStderrContains("failed to decode modrinth.index.json")

	// Subtest 4: Non-existent remote modpack slug (404)
	res = ctx.Run("sync", "--source", "modrinth", "--slug", "non_existent_slug_404")
	res.AssertFailure(1)
	res.AssertStderrContains("404")
}

// 3. R3 GitHub Sync Adversarial E2E Tests
func TestCLI_SyncGitHub_Adversarial(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")

	// Subtest 1: Missing --repo flag
	res := ctx.Run("sync", "--source", "github")
	res.AssertFailure(1)
	res.AssertStderrContains("--repo")

	// Subtest 2: Nonexistent repo (404)
	res = ctx.Run("sync", "--source", "github", "--repo", "org/nonexistent")
	res.AssertFailure(1)
	res.AssertStderrContains("404")

	// Subtest 3: Private repo without token (401)
	res = ctx.Run("sync", "--source", "github", "--repo", "org/private")
	res.AssertFailure(1)
	res.AssertStderrContains("401")

	// Subtest 4: Private repo with invalid token (401)
	res = ctx.Run("sync", "--source", "github", "--repo", "org/private", "--token", "invalid_token_123")
	res.AssertFailure(1)
	res.AssertStderrContains("401")

	// Subtest 5: Private repo with valid token (Success)
	res = ctx.Run("sync", "--source", "github", "--repo", "org/private", "--token", "ghp_mock")
	res.AssertSuccess()

	// Subtest 6: Up-to-date detection
	res = ctx.Run("sync", "--source", "github", "--repo", "org/private", "--token", "ghp_mock")
	res.AssertSuccess()
	res.AssertStdoutContains("Already up to date")
}

// 4. R5 Remote Sync Adversarial E2E Tests
func TestCLI_SyncRemote_Adversarial(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")

	// Subtest 1: Unreachable URL
	res := ctx.Run("sync", "--url", "http://127.0.0.1:59999")
	res.AssertFailure(1)
	res.AssertStderrContains("unreachable")

	// Subtest 2: Missing auth token when server requires auth
	// Mock server /lock requires token if tested against auth server
	// Subtest 3: Valid remote sync and Up-to-date check
	res = ctx.Run("sync", "--url", srv.URL(), "--token", "tok")
	res.AssertSuccess()

	// Second run should be up to date
	res = ctx.Run("sync", "--url", srv.URL(), "--token", "tok")
	res.AssertSuccess()
	res.AssertStdoutContains("Already up to date")
}

// 5. Pin Preservation Across Sync Operations
func TestCLI_Sync_PinPreservation(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())
	ctx.WriteFile("cmm.toml", "name = \"Test\"\n[server]\nmc_version = \"1.21.1\"\nloader = \"fabric\"")

	// Seed lockfile with pinned mod
	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"sodium\"\nslug = \"sodium\"\nversion = \"0.4.0\"\npinned = true\n")

	// GitHub sync brings in sodium 0.4.0
	res := ctx.Run("sync", "--source", "github", "--repo", "org/pack")
	res.AssertSuccess()

	lockContent := ctx.ReadFile("cmm.lock")
	if !strings.Contains(lockContent, "pinned = true") {
		t.Fatalf("expected pinned = true in lockfile, got:\n%s", lockContent)
	}
}
