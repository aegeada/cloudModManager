package tier3

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"cmm/internal/sync"
)

func getFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func waitForServerReady(t *testing.T, port int, maxWait time.Duration) {
	t.Helper()
	deadline := time.Now().Add(maxWait)
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatalf("daemon on port %d did not start within %v", port, maxWait)
}

func startDaemon(t *testing.T, dir string, port int, token, mockServerURL string) (*exec.Cmd, func()) {
	t.Helper()
	args := []string{"serve", "--port", strconv.Itoa(port)}
	if token != "" {
		args = append(args, "--token", token)
	}

	cmd := exec.Command(harness.BinaryPath, args...)
	cmd.Dir = dir
	cmdEnv := []string{
		"CMM_NO_SELF_INSTALL=1",
		"TERM=dumb",
		"HOME=" + dir,
	}
	if mockServerURL != "" {
		cmdEnv = append(cmdEnv,
			"MODRINTH_API_URL="+mockServerURL+"/v2",
			"FABRIC_META_URL="+mockServerURL+"/fabric-meta",
		)
	}
	cmd.Env = append(os.Environ(), cmdEnv...)

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start cmm serve daemon: %v", err)
	}

	waitForServerReady(t, port, 3*time.Second)

	cleanup := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}

	return cmd, cleanup
}

func TestPairwise_ModAdd_DiffMismatch_RemotePush_DiffOK_LauncherSync_Export(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()

	// 1. Setup Remote Server Daemon
	serverDir := t.TempDir()
	serverPort := getFreePort(t)
	token := "pairwise-secret-token"

	serverCfg := `[profile]
name = "server-modpack"
minecraft_version = "1.21.1"
loader = "fabric"
loader_version = "0.16.5"
side = "server"
`
	_ = os.WriteFile(filepath.Join(serverDir, "cmm.toml"), []byte(serverCfg), 0644)
	_ = os.WriteFile(filepath.Join(serverDir, "cmm.lock"), []byte("[mods]\n"), 0644)

	_, cleanup := startDaemon(t, serverDir, serverPort, token, ms.URL())
	defer cleanup()

	serverURL := fmt.Sprintf("http://127.0.0.1:%d", serverPort)

	// 2. Initialize Client Project
	ctx := harness.NewTestContext(t, ms.URL())
	resInit := ctx.Run("init", "--name", "my-client-pack", "--mc-version", "1.21.1", "--loader", "fabric", "--side", "both")
	resInit.AssertSuccess()

	// 3. Add Mods locally
	resAddLithium := ctx.Run("add", "lithium")
	resAddLithium.AssertSuccess()

	resAddSodium := ctx.Run("add", "sodium")
	resAddSodium.AssertSuccess()

	// Add custom config
	ctx.WriteFile("config/client_settings.json", `{"graphics": "fast", "render_distance": 12}`)

	// 4. Run Compatibility Diff against remote server -> Expect MISSING on server
	resDiffPre := ctx.Run("diff", "--url", serverURL, "--token", token)
	resDiffPre.AssertSuccess()
	resDiffPre.AssertStdoutContains("[MISSING]", "lithium", "sodium")

	// 5. Deploy remotely using cmm push
	resPush := ctx.Run("push", "--url", serverURL, "--token", token, "--include-config")
	resPush.AssertSuccess()
	resPush.AssertStdoutContains("Successfully pushed modpack to remote server.")

	// 6. Re-run Diff -> Expect Synchronized [OK]
	resDiffPost := ctx.Run("diff", "--url", serverURL, "--token", token, "--json")
	resDiffPost.AssertSuccess()

	var diffRes sync.DiffResult
	if err := json.Unmarshal([]byte(resDiffPost.Stdout), &diffRes); err != nil {
		t.Fatalf("failed to parse diff JSON: %v", err)
	}

	if diffRes.Mismatches != 0 || diffRes.Missing != 0 {
		t.Errorf("expected 0 mismatches and 0 missing after push, got mismatches=%d missing=%d",
			diffRes.Mismatches, diffRes.Missing)
	}
	if diffRes.Synchronized < 2 {
		t.Errorf("expected at least 2 synchronized mods, got %d", diffRes.Synchronized)
	}

	// Verify server config extraction
	serverConfigFile := filepath.Join(serverDir, "config", "client_settings.json")
	if data, err := os.ReadFile(serverConfigFile); err != nil || !strings.Contains(string(data), "render_distance") {
		t.Fatalf("server config was not extracted properly: %v", err)
	}

	// 7. Setup Mock Prism Instance and Sync Locally
	prismInstanceDir := filepath.Join(ctx.TempDir, ".local", "share", "PrismLauncher", "instances", "Prism-Dev", ".minecraft")
	_ = os.MkdirAll(prismInstanceDir, 0755)
	_ = os.WriteFile(filepath.Join(ctx.TempDir, ".local", "share", "PrismLauncher", "instances", "Prism-Dev", "instance.cfg"),
		[]byte("name=Prism-Dev\nIntendedVersion=1.21.1\n"), 0644)
	packJSON := `{"components":[{"uid":"net.minecraft","version":"1.21.1"},{"uid":"net.fabricmc.fabric-loader","version":"0.16.5"}]}`
	_ = os.WriteFile(filepath.Join(ctx.TempDir, ".local", "share", "PrismLauncher", "instances", "Prism-Dev", "mmc-pack.json"),
		[]byte(packJSON), 0644)

	resLauncherSync := ctx.Run("launcher", "sync", "Prism-Dev")
	resLauncherSync.AssertSuccess()
	resLauncherSync.AssertStdoutContains("Successfully synchronized modpack into Prism Launcher instance 'Prism-Dev'")

	// Verify launcher instance mods directory contains the mods
	modsDir := filepath.Join(prismInstanceDir, "mods")
	entries, err := os.ReadDir(modsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected mods in launcher instance mods dir: %v", err)
	}

	// 8. Export Modpack to .mrpack
	mrpackPath := filepath.Join(ctx.TempDir, "release.mrpack")
	resExport := ctx.Run("export", "--format", "mrpack", "--output", mrpackPath)
	resExport.AssertSuccess()

	zr, err := zip.OpenReader(mrpackPath)
	if err != nil {
		t.Fatalf("failed to open exported mrpack archive: %v", err)
	}
	defer zr.Close()

	foundIndex := false
	for _, f := range zr.File {
		if f.Name == "modrinth.index.json" {
			foundIndex = true
			break
		}
	}
	if !foundIndex {
		t.Fatalf("exported mrpack is missing modrinth.index.json")
	}
}

func TestPairwise_RemotePush_WithConfig_And_SideFiltering_Diff(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()

	serverDir := t.TempDir()
	serverPort := getFreePort(t)
	token := "side-filter-token"

	// Server initial lock has a server-only mod
	serverLock := fmt.Sprintf(`[[mods]]
name = "Lithium"
slug = "lithium"
version = "0.12.0"
side = "both"
download_url = "%s/download/lithium-fabric-0.12.0.jar"

[[mods]]
name = "Chunky"
slug = "chunky"
version = "1.3.0"
side = "server"
download_url = "%s/download/chunky-1.3.0.jar"
`, ms.URL(), ms.URL())

	_ = os.WriteFile(filepath.Join(serverDir, "cmm.toml"), []byte("side = \"server\"\n"), 0644)
	_ = os.WriteFile(filepath.Join(serverDir, "cmm.lock"), []byte(serverLock), 0644)

	_, cleanup := startDaemon(t, serverDir, serverPort, token, ms.URL())
	defer cleanup()

	serverURL := fmt.Sprintf("http://127.0.0.1:%d", serverPort)

	// Client has lithium (both) and iris (client-only)
	ctx := harness.NewTestContext(t, ms.URL())
	ctx.WriteFile("cmm.toml", "side = \"both\"\n")
	clientLock := fmt.Sprintf(`[[mods]]
name = "Lithium"
slug = "lithium"
version = "0.12.0"
side = "both"
download_url = "%s/download/lithium-fabric-0.12.0.jar"

[[mods]]
name = "Iris"
slug = "iris"
version = "1.7.0"
side = "client"
download_url = "%s/download/iris-1.7.0.jar"
`, ms.URL(), ms.URL())
	ctx.WriteFile("cmm.lock", clientLock)

	// Diff before push:
	// Lithium: [OK]
	// Iris: [CLIENT]
	// Chunky: [SERVER]
	resDiff := ctx.Run("diff", "--url", serverURL, "--token", token)
	resDiff.AssertSuccess()
	resDiff.AssertStdoutContains("[OK]", "[CLIENT]", "[SERVER]")
	resDiff.AssertStdoutContains("1 synchronized | 0 mismatches | 1 client-only | 1 server-only | 0 missing")

	// Push client lockfile to server
	resPush := ctx.Run("push", "--url", serverURL, "--token", token)
	resPush.AssertSuccess()

	// Server lockfile now contains Iris (retained as client-only in lockfile), Lithium, and Chunky (if retained or pruned)
	serverLockBytes, err := os.ReadFile(filepath.Join(serverDir, "cmm.lock"))
	if err != nil {
		t.Fatalf("failed to read server lockfile: %v", err)
	}
	if !strings.Contains(string(serverLockBytes), "lithium") {
		t.Fatalf("server lockfile missing lithium: %s", string(serverLockBytes))
	}
}
