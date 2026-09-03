package tier1

import (
	"archive/zip"
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// 1. HTTP Server Daemon Authentication Matrix
func TestAdversarial_Serve_AuthMatrix(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.lock", "[mods.sodium]\nversion = \"0.5.8\"\n")

	port := "8091"
	expectedToken := "secret-auth-token-xyz"

	cmd := exec.Command(harness.BinaryPath, "serve", "--port", port, "--token", expectedToken)
	cmd.Dir = ctx.TempDir
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start serve: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()

	// Allow server to spin up
	time.Sleep(250 * time.Millisecond)

	baseURL := fmt.Sprintf("http://127.0.0.1:%s/lock", port)

	testCases := []struct {
		name       string
		authHeader string
		wantCode   int
		wantBody   bool
	}{
		{
			name:       "Missing auth header",
			authHeader: "",
			wantCode:   http.StatusUnauthorized,
			wantBody:   false,
		},
		{
			name:       "Valid Bearer token",
			authHeader: "Bearer " + expectedToken,
			wantCode:   http.StatusOK,
			wantBody:   true,
		},
		{
			name:       "Valid raw token (no prefix)",
			authHeader: expectedToken,
			wantCode:   http.StatusOK,
			wantBody:   true,
		},
		{
			name:       "Invalid Bearer token",
			authHeader: "Bearer wrong-token-123",
			wantCode:   http.StatusUnauthorized,
			wantBody:   false,
		},
		{
			name:       "Invalid raw token",
			authHeader: "wrong-token-123",
			wantCode:   http.StatusUnauthorized,
			wantBody:   false,
		},
		{
			name:       "Empty Bearer prefix",
			authHeader: "Bearer ",
			wantCode:   http.StatusUnauthorized,
			wantBody:   false,
		},
		{
			name:       "Basic auth scheme",
			authHeader: "Basic dXNlcjpwYXNz",
			wantCode:   http.StatusUnauthorized,
			wantBody:   false,
		},
		{
			name:       "Token with extra word/injection",
			authHeader: "Bearer " + expectedToken + " extra-injected",
			wantCode:   http.StatusUnauthorized,
			wantBody:   false,
		},
		{
			name:       "Token with leading whitespace",
			authHeader: "Bearer  " + expectedToken,
			wantCode:   http.StatusUnauthorized,
			wantBody:   false,
		},
	}

	client := &http.Client{Timeout: 2 * time.Second}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, baseURL, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantCode {
				t.Errorf("got status %d, want %d", resp.StatusCode, tc.wantCode)
			}

			if tc.wantCode == http.StatusUnauthorized {
				wwwAuth := resp.Header.Get("WWW-Authenticate")
				if !strings.Contains(wwwAuth, "Bearer") {
					t.Errorf("expected WWW-Authenticate header with Bearer, got %q", wwwAuth)
				}
			}

			if tc.wantBody {
				body, _ := io.ReadAll(resp.Body)
				if !strings.Contains(string(body), "sodium") {
					t.Errorf("expected body to contain 'sodium', got %q", string(body))
				}
			}
		})
	}
}

// 2. HTTP Server Daemon Config Fallback Token
func TestAdversarial_Serve_TomlFallbackToken(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.toml", `
sync_token = "toml-secret-token-abc"

[profile]
name = "server-pack"
minecraft_version = "1.21.1"
`)
	ctx.WriteFile("cmm.lock", "[mods.iris]\nversion = \"1.7.0\"\n")

	port := "8092"
	// Do not pass --token flag to verify fallback to cmm.toml sync_token
	cmd := exec.Command(harness.BinaryPath, "serve", "--port", port)
	cmd.Dir = ctx.TempDir
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start serve: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()

	time.Sleep(250 * time.Millisecond)

	baseURL := fmt.Sprintf("http://127.0.0.1:%s/lock", port)
	client := &http.Client{Timeout: 2 * time.Second}

	// 1. Missing auth -> 401
	req1, _ := http.NewRequest(http.MethodGet, baseURL, nil)
	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", resp1.StatusCode)
	}

	// 2. Correct token from cmm.toml -> 200
	req2, _ := http.NewRequest(http.MethodGet, baseURL, nil)
	req2.Header.Set("Authorization", "Bearer toml-secret-token-abc")
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK with toml sync_token, got %d", resp2.StatusCode)
	}
	body, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(body), "iris") {
		t.Errorf("expected body to contain 'iris', got %q", string(body))
	}
}

// 3. HTTP Server Daemon Methods, Routing and Lockfile Edge Cases
func TestAdversarial_Serve_HttpMethodsAndEdgeCases(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	port := "8093"
	// Anonymous mode without token and without existing cmm.lock
	cmd := exec.Command(harness.BinaryPath, "serve", "--port", port)
	cmd.Dir = ctx.TempDir
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start serve: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()

	time.Sleep(250 * time.Millisecond)

	client := &http.Client{Timeout: 2 * time.Second}

	// Health endpoint
	healthResp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/health", port))
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Errorf("health returned status %d, want 200", healthResp.StatusCode)
	}

	// Lockfile non-existent returns default [mods] with 200
	lockResp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/lock", port))
	if err != nil {
		t.Fatalf("lock check failed: %v", err)
	}
	defer lockResp.Body.Close()
	if lockResp.StatusCode != http.StatusOK {
		t.Errorf("lock returned status %d, want 200", lockResp.StatusCode)
	}
	body, _ := io.ReadAll(lockResp.Body)
	if !strings.Contains(string(body), "[mods]") {
		t.Errorf("expected default lockfile content, got %q", string(body))
	}

	// Method Not Allowed checks (POST, PUT, DELETE)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, m := range methods {
		req, _ := http.NewRequest(m, fmt.Sprintf("http://127.0.0.1:%s/lock", port), strings.NewReader("payload"))
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s request failed: %v", m, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s /lock returned %d, want 405 Method Not Allowed", m, resp.StatusCode)
		}
	}

	// 404 on unknown routes
	unknownResp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/unknown-route", port))
	if err != nil {
		t.Fatalf("unknown request failed: %v", err)
	}
	unknownResp.Body.Close()
	if unknownResp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown route returned %d, want 404", unknownResp.StatusCode)
	}
}

// 4. HTTP Server Graceful Shutdown Latency Under Load (SIGINT and SIGTERM < 2s)
func TestAdversarial_Serve_GracefulShutdownUnderLoad(t *testing.T) {
	signals := []os.Signal{os.Interrupt, syscall.SIGTERM}

	for idx, sig := range signals {
		t.Run(fmt.Sprintf("Signal_%s", sig.String()), func(t *testing.T) {
			srv := mockserver.New()
			defer srv.Close()
			ctx := harness.NewTestContext(t, srv.URL())

			ctx.WriteFile("cmm.lock", "[mods.sodium]\nversion=\"0.5.8\"\n")

			portInt := 8094 + idx
			port := fmt.Sprintf("%d", portInt)

			cmd := exec.Command(harness.BinaryPath, "serve", "--port", port)
			cmd.Dir = ctx.TempDir
			if err := cmd.Start(); err != nil {
				t.Fatalf("failed to start serve: %v", err)
			}

			time.Sleep(250 * time.Millisecond)

			// Spawn concurrent workers hitting the server
			stopLoad := make(chan struct{})
			var wg sync.WaitGroup
			workerCount := 10
			client := &http.Client{Timeout: 500 * time.Millisecond}

			for i := 0; i < workerCount; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for {
						select {
						case <-stopLoad:
							return
						default:
							req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%s/lock", port), nil)
							resp, err := client.Do(req)
							if err == nil {
								resp.Body.Close()
							}
							time.Sleep(10 * time.Millisecond)
						}
					}
				}()
			}

			// Send signal
			startShutdown := time.Now()
			if err := cmd.Process.Signal(sig); err != nil {
				close(stopLoad)
				cmd.Process.Kill()
				t.Fatalf("failed to send signal %v: %v", sig, err)
			}

			done := make(chan error, 1)
			go func() {
				done <- cmd.Wait()
			}()

			select {
			case <-time.After(2 * time.Second):
				close(stopLoad)
				cmd.Process.Kill()
				t.Fatalf("server failed to shut down within 2 seconds (exceeded SLA)")
			case err := <-done:
				shutdownDuration := time.Since(startShutdown)
				close(stopLoad)
				wg.Wait()

				if shutdownDuration > 2*time.Second {
					t.Fatalf("shutdown duration %v exceeded 2.0s requirement", shutdownDuration)
				}
				t.Logf("Graceful shutdown under load succeeded in %v (signal: %s, exit err: %v)", shutdownDuration, sig, err)
			}

			// Verify the port is immediately free and can be re-bound
			l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", portInt))
			if err != nil {
				t.Fatalf("port %d remained bound after shutdown: %v", portInt, err)
			}
			_ = l.Close()
		})
	}
}

// 5. Mrpack Export - Deep Validation of Zip, Metadata, Env Mapping, and Overrides
func TestAdversarial_ExportMrpack_DeepValidation(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.toml", `
[profile]
name = "EpicServerPack"
minecraft_version = "1.21.1"
loader = "fabric"
loader_version = "0.19.3"
`)

	ctx.WriteFile("cmm.lock", `
[mods.sodium]
name = "Sodium"
filename = "sodium-fabric-0.5.8.jar"
side = "client"
sha512 = "a1b2c3d4e5f6"
download_url = "https://cdn.modrinth.com/data/sodium.jar"

[mods.worldedit]
name = "WorldEdit"
filename = "worldedit-mod-7.2.15.jar"
side = "server"
sha512 = "112233445566"
download_url = "https://cdn.modrinth.com/data/worldedit.jar"

[mods.fabric_api]
name = "Fabric API"
filename = "fabric-api-0.92.0.jar"
side = "both"
sha512 = "998877665544"
download_url = "https://cdn.modrinth.com/data/fabric-api.jar"

[mods.replaymod]
name = "Replay Mod"
filename = "replaymod-1.21.1.jar"
side = "optional"
sha512 = "ffeeddccbbaa"
download_url = "https://cdn.modrinth.com/data/replaymod.jar"
`)

	// Create config overrides and general overrides
	ctx.WriteFile("config/sodium-options.json", `{"graphics": "fast"}`)
	ctx.WriteFile("config/nested/advanced.toml", `debug = true`)
	ctx.WriteFile("overrides/resourcepacks/custom_pack.zip", "dummy-zip-data")

	outZipPath := "dist/my_custom_modpack.mrpack"
	res := ctx.Run("export", "--format", "mrpack", "--output", outZipPath, "--name", "AdversarialPack", "--version-id", "2.5.0-beta")
	res.AssertSuccess()

	fullPath := filepath.Join(ctx.TempDir, outZipPath)
	zr, err := zip.OpenReader(fullPath)
	if err != nil {
		t.Fatalf("failed to open generated mrpack zip: %v", err)
	}
	defer zr.Close()

	zipEntries := make(map[string][]byte)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open zip entry %s: %v", f.Name, err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		zipEntries[f.Name] = data
	}

	// 1. Verify modrinth.index.json exists and is valid JSON
	indexData, ok := zipEntries["modrinth.index.json"]
	if !ok {
		t.Fatalf("missing modrinth.index.json in mrpack")
	}

	type mrpackIndex struct {
		FormatVersion int               `json:"formatVersion"`
		Game          string            `json:"game"`
		VersionID     string            `json:"versionId"`
		Name          string            `json:"name"`
		Files         []harness.MrpackFileEntry `json:"files"`
		Dependencies  map[string]string `json:"dependencies"`
	}

	var idx mrpackIndex
	if err := json.Unmarshal(indexData, &idx); err != nil {
		t.Fatalf("invalid JSON in modrinth.index.json: %v", err)
	}

	if idx.FormatVersion != 1 {
		t.Errorf("expected formatVersion 1, got %d", idx.FormatVersion)
	}
	if idx.Game != "minecraft" {
		t.Errorf("expected game 'minecraft', got %q", idx.Game)
	}
	if idx.Name != "AdversarialPack" {
		t.Errorf("expected name override 'AdversarialPack', got %q", idx.Name)
	}
	if idx.VersionID != "2.5.0-beta" {
		t.Errorf("expected versionId override '2.5.0-beta', got %q", idx.VersionID)
	}
	if idx.Dependencies["minecraft"] != "1.21.1" {
		t.Errorf("expected dependency minecraft=1.21.1, got %q", idx.Dependencies["minecraft"])
	}
	if idx.Dependencies["fabric-loader"] != "0.19.3" {
		t.Errorf("expected dependency fabric-loader=0.19.3, got %q", idx.Dependencies["fabric-loader"])
	}

	if len(idx.Files) != 4 {
		t.Fatalf("expected 4 files in index, got %d", len(idx.Files))
	}

	// Verify env mappings
	for _, f := range idx.Files {
		switch f.Path {
		case "mods/sodium-fabric-0.5.8.jar":
			if f.Env.Client != "required" || f.Env.Server != "unsupported" {
				t.Errorf("sodium env mismatch: client=%s server=%s", f.Env.Client, f.Env.Server)
			}
			if f.Hashes["sha512"] != "a1b2c3d4e5f6" {
				t.Errorf("sodium hash mismatch: %v", f.Hashes)
			}
		case "mods/worldedit-mod-7.2.15.jar":
			if f.Env.Client != "unsupported" || f.Env.Server != "required" {
				t.Errorf("worldedit env mismatch: client=%s server=%s", f.Env.Client, f.Env.Server)
			}
		case "mods/fabric-api-0.92.0.jar":
			if f.Env.Client != "required" || f.Env.Server != "required" {
				t.Errorf("fabric-api env mismatch: client=%s server=%s", f.Env.Client, f.Env.Server)
			}
		case "mods/replaymod-1.21.1.jar":
			if f.Env.Client != "optional" || f.Env.Server != "optional" {
				t.Errorf("replaymod env mismatch: client=%s server=%s", f.Env.Client, f.Env.Server)
			}
		}
	}

	// 2. Verify overrides packaged correctly
	if _, ok := zipEntries["overrides/config/sodium-options.json"]; !ok {
		t.Errorf("expected overrides/config/sodium-options.json in zip")
	}
	if _, ok := zipEntries["overrides/config/nested/advanced.toml"]; !ok {
		t.Errorf("expected overrides/config/nested/advanced.toml in zip")
	}
	if _, ok := zipEntries["overrides/resourcepacks/custom_pack.zip"]; !ok {
		t.Errorf("expected overrides/resourcepacks/custom_pack.zip in zip")
	}
}

// 6. GitHub Export - Strict Token Stripping and Confidentiality Audit
func TestAdversarial_ExportGitHub_SecretSanitizationAudit(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	syncSecret := "LEAK_SYNC_TOKEN_SECRET_999"
	modrinthSecret := "LEAK_MODRINTH_TOKEN_SECRET_888"
	ghSourceSecret := "LEAK_GITHUB_TOKEN_777"
	remoteSourceSecret := "LEAK_REMOTE_TOKEN_666"

	ctx.WriteFile("cmm.toml", fmt.Sprintf(`
[profile]
name = "ConfidentialPack"
minecraft_version = "1.21.1"

sync_token = "%s"

[modrinth]
token = "%s"

[[sync_sources]]
type = "github"
repo = "org/private-repo"
token = "%s"

[[sync_sources]]
type = "remote"
url = "https://example.com"
token = "%s"
`, syncSecret, modrinthSecret, ghSourceSecret, remoteSourceSecret))

	ctx.WriteFile("cmm.lock", `
[mods.sodium]
name = "Sodium"
version = "0.5.8"
pinned = true
`)

	exportOut := "gh_export_out"
	res := ctx.Run("export", "--format", "github", "--output", exportOut)
	res.AssertSuccess()

	exportedToml := ctx.ReadFile(filepath.Join(exportOut, "cmm.toml"))
	exportedLock := ctx.ReadFile(filepath.Join(exportOut, "cmm.lock"))

	// Rigorous substring check: NONE of the sensitive tokens should appear anywhere
	secrets := []string{syncSecret, modrinthSecret, ghSourceSecret, remoteSourceSecret}
	for _, sec := range secrets {
		if strings.Contains(exportedToml, sec) {
			t.Fatalf("SECURITY VIOLATION: Exported cmm.toml contains secret %q", sec)
		}
		if strings.Contains(exportedLock, sec) {
			t.Fatalf("SECURITY VIOLATION: Exported cmm.lock contains secret %q", sec)
		}
	}

	// Verify cmm.lock retained mods and pin status
	if !strings.Contains(exportedLock, "sodium") || !strings.Contains(exportedLock, "pinned = true") {
		t.Errorf("exported lockfile lost mod or pin data: %s", exportedLock)
	}
}

// 7. Error Handling on Unwritable and Invalid Export Paths
func TestAdversarial_Export_UnwritableAndInvalidPaths(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.toml", `
[profile]
name = "Test"
minecraft_version = "1.21.1"
`)
	ctx.WriteFile("cmm.lock", "[mods]\n")

	// 1. Invalid format flag
	resInvalidFormat := ctx.Run("export", "--format", "invalid_format_xyz")
	resInvalidFormat.AssertFailure()
	if !strings.Contains(strings.ToLower(resInvalidFormat.Stderr), "invalid export format") {
		t.Errorf("expected stderr to mention 'Invalid export format', got %q", resInvalidFormat.Stderr)
	}

	// 2. Unwritable read-only directory for mrpack export
	roDir := filepath.Join(ctx.TempDir, "readonly_dir")
	if err := os.MkdirAll(roDir, 0555); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}
	defer os.Chmod(roDir, 0755)

	resMrpackRO := ctx.Run("export", "--format", "mrpack", "--output", filepath.Join(roDir, "nested", "pack.mrpack"))
	resMrpackRO.AssertFailure()

	// 3. Unwritable read-only directory for github export
	resGitHubRO := ctx.Run("export", "--format", "github", "--output", filepath.Join(roDir, "nested_gh"))
	resGitHubRO.AssertFailure()
}

// 8. Cross-Compilation Binaries Verification
func TestAdversarial_CrossCompilationBinariesVerification(t *testing.T) {
	binDir := filepath.Join("..", "..", "bin")
	expectedBinaries := []string{
		"cmm-linux-amd64",
		"cmm-linux-arm64",
		"cmm-darwin-amd64",
		"cmm-darwin-arm64",
		"cmm-windows-amd64.exe",
	}

	for _, name := range expectedBinaries {
		path := filepath.Join(binDir, name)
		fi, err := os.Stat(path)
		if err != nil {
			t.Errorf("binary %s is missing: %v", name, err)
			continue
		}
		if fi.Size() < 5*1024*1024 { // at least 5 MB
			t.Errorf("binary %s size (%d bytes) is unexpectedly small", name, fi.Size())
		}
	}

	// Test executing the native linux-amd64 binary directly
	linuxAmd64Bin := filepath.Join(binDir, "cmm-linux-amd64")
	absBin, err := filepath.Abs(linuxAmd64Bin)
	if err != nil {
		t.Fatalf("failed to get abs path: %v", err)
	}

	ctx := harness.NewTestContext(t, "http://127.0.0.1:9999")
	ctx.WriteFile("cmm.toml", `
[profile]
name = "BinTest"
minecraft_version = "1.21.1"
`)
	ctx.WriteFile("cmm.lock", "[mods.sodium]\nversion=\"0.5.8\"\n")

	// Run export using cross-compiled binary
	cmd := exec.Command(absBin, "export", "--format", "github", "--output", "bin_gh_out")
	cmd.Dir = ctx.TempDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("executing cmm-linux-amd64 failed: %v, output: %s", err, string(out))
	}

	if ctx.ReadFile("bin_gh_out/cmm.lock") == "" {
		t.Errorf("cmm-linux-amd64 failed to generate exported cmm.lock")
	}
}

// 9. HTTP Server Port Conflict Handling
func TestAdversarial_Serve_PortConflict(t *testing.T) {
	port := 8098
	// Occupy the port first
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("failed to bind port %d: %v", port, err)
	}
	defer l.Close()

	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	res := ctx.Run("serve", "--port", fmt.Sprintf("%d", port))
	res.AssertFailure(1)
	if !strings.Contains(strings.ToLower(res.Stderr), "bind") && !strings.Contains(strings.ToLower(res.Stderr), "address already in use") && !strings.Contains(strings.ToLower(res.Stderr), "server error") {
		t.Errorf("expected error message regarding port conflict, got: %q", res.Stderr)
	}
}

// 10. HTTP Server with Directory instead of Lockfile (500 Error Handling)
func TestAdversarial_Serve_DirectoryLockfileError(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	// Create cmm.lock as a directory rather than a file
	if err := os.Mkdir(filepath.Join(ctx.TempDir, "cmm.lock"), 0755); err != nil {
		t.Fatalf("failed to create directory cmm.lock: %v", err)
	}

	port := "8099"
	cmd := exec.Command(harness.BinaryPath, "serve", "--port", port)
	cmd.Dir = ctx.TempDir
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start serve: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()

	time.Sleep(250 * time.Millisecond)

	client := &http.Client{Timeout: 2 * time.Second}

	// Health still works -> 200
	healthResp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/health", port))
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Errorf("health returned %d, want 200", healthResp.StatusCode)
	}

	// Lock returns 500 Internal Server Error because it's a directory
	lockResp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/lock", port))
	if err != nil {
		t.Fatalf("lock request failed: %v", err)
	}
	lockResp.Body.Close()
	if lockResp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 Internal Server Error for directory lockfile, got %d", lockResp.StatusCode)
	}
}

// 11. Mrpack Export - Local Jar Disk Hashing (SHA-512 and SHA-1 computation)
func TestAdversarial_ExportMrpack_LocalJarHashing(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.toml", `
[profile]
name = "LocalHashPack"
minecraft_version = "1.21.1"
`)

	// Create a real jar file with specific bytes
	jarContent := "PK\x03\x04custom-local-mod-content-for-sha-computation"
	ctx.WriteFile("mods/localmod.jar", jarContent)

	ctx.WriteFile("cmm.lock", `
[mods.localmod]
name = "Local Mod"
filename = "localmod.jar"
side = "client"
`)

	res := ctx.Run("export", "--format", "mrpack", "--output", "hash_test.mrpack")
	res.AssertSuccess()

	fullPath := filepath.Join(ctx.TempDir, "hash_test.mrpack")
	zr, err := zip.OpenReader(fullPath)
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name == "modrinth.index.json" {
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()

			var idx harness.MrpackIndex
			if err := json.Unmarshal(data, &idx); err != nil {
				t.Fatalf("failed to parse index json: %v", err)
			}
			if len(idx.Files) != 1 {
				t.Fatalf("expected 1 file in index, got %d", len(idx.Files))
			}
			fileEntry := idx.Files[0]
			if fileEntry.FileSize != int64(len(jarContent)) {
				t.Errorf("expected fileSize %d, got %d", len(jarContent), fileEntry.FileSize)
			}
			if fileEntry.Hashes["sha512"] == "" {
				t.Errorf("expected sha512 hash to be computed from disk jar")
			}
			if fileEntry.Hashes["sha1"] == "" {
				t.Errorf("expected sha1 hash to be computed from disk jar")
			}
			t.Logf("Computed hashes for local jar: sha512=%s, sha1=%s, size=%d",
				fileEntry.Hashes["sha512"], fileEntry.Hashes["sha1"], fileEntry.FileSize)
		}
	}
}

// 12. GitHub Export - Full Non-Sensitive Config Fidelity Preservation
func TestAdversarial_ExportGitHub_PreservesAllNonSensitiveFields(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.toml", `
sync_token = "secret-to-strip"

[profile]
name = "FidelityPack"
minecraft_version = "1.21.1"
loader = "fabric"
loader_version = "0.19.3"
side = "server"

[paths]
mods_dir = "custom_mods"
config_dir = "custom_config"

[modrinth]
token = "modrinth-to-strip"

[[sync_sources]]
type = "github"
repo = "owner/repo"
token = "gh-to-strip"
`)

	ctx.WriteFile("cmm.lock", `
[mods.sodium]
name = "Sodium"
version = "0.5.8"
pinned = true
`)

	res := ctx.Run("export", "--format", "github", "--output", "fidelity_out")
	res.AssertSuccess()

	exportedToml := ctx.ReadFile("fidelity_out/cmm.toml")

	// Secrets stripped
	if strings.Contains(exportedToml, "secret-to-strip") ||
		strings.Contains(exportedToml, "modrinth-to-strip") ||
		strings.Contains(exportedToml, "gh-to-strip") {
		t.Fatalf("secrets leaked in exported TOML:\n%s", exportedToml)
	}

	// Non-sensitive settings preserved
	expectedSubstrings := []string{
		"FidelityPack",
		"1.21.1",
		"fabric",
		"0.19.3",
		"custom_mods",
		"custom_config",
		"owner/repo",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(exportedToml, sub) {
			t.Errorf("expected exported TOML to preserve %q, but missing:\n%s", sub, exportedToml)
		}
	}
}

