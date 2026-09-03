package tier4

import (
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
	t.Fatalf("oracle cloud daemon on port %d did not start within %v", port, maxWait)
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

func TestWorkload_M5_EndToEndAdminWorkflow(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()

	// 1. Setup Remote Oracle Cloud Server Daemon
	oracleServerDir := t.TempDir()
	oraclePort := getFreePort(t)
	oracleAuthToken := "oracle-cloud-arm64-token-99"

	oracleToml := `[profile]
name = "oracle-production-server"
minecraft_version = "1.21.1"
loader = "fabric"
loader_version = "0.16.5"
side = "server"
`
	_ = os.WriteFile(filepath.Join(oracleServerDir, "cmm.toml"), []byte(oracleToml), 0644)
	_ = os.WriteFile(filepath.Join(oracleServerDir, "cmm.lock"), []byte("[mods]\n"), 0644)

	_, cleanupOracle := startDaemon(t, oracleServerDir, oraclePort, oracleAuthToken, ms.URL())
	defer cleanupOracle()

	oracleURL := fmt.Sprintf("http://127.0.0.1:%d", oraclePort)

	// 2. Setup Admin Local Workstation Context
	adminCtx := harness.NewTestContext(t, ms.URL())

	// Step A: Initialize local modpack
	resInit := adminCtx.Run("init",
		"--name", "oracle-cloud-modpack",
		"--mc-version", "1.21.1",
		"--loader", "fabric",
		"--side", "both",
	)
	resInit.AssertSuccess()

	// Step B: Add mods (Lithium optimization, Sodium renderer)
	resAddLithium := adminCtx.Run("add", "lithium")
	resAddLithium.AssertSuccess()

	resAddSodium := adminCtx.Run("add", "sodium")
	resAddSodium.AssertSuccess()

	// Add custom configurations
	adminCtx.WriteFile("config/sodium-options.json", `{"graphicsQuality": "high", "fpsLimit": 144}`)
	adminCtx.WriteFile("config/lithium.properties", "chunk.serialization=true\nentity.collisions=true\n")

	// Step C: Auto-detect local Prism instance and sync directly
	prismDir := filepath.Join(adminCtx.TempDir, ".local", "share", "PrismLauncher", "instances", "Prism-Admin-Test", ".minecraft")
	_ = os.MkdirAll(prismDir, 0755)
	_ = os.WriteFile(filepath.Join(adminCtx.TempDir, ".local", "share", "PrismLauncher", "instances", "Prism-Admin-Test", "instance.cfg"),
		[]byte("name=Prism-Admin-Test\nIntendedVersion=1.21.1\n"), 0644)
	packJSON := `{"components":[{"uid":"net.minecraft","version":"1.21.1"},{"uid":"net.fabricmc.fabric-loader","version":"0.16.5"}]}`
	_ = os.WriteFile(filepath.Join(adminCtx.TempDir, ".local", "share", "PrismLauncher", "instances", "Prism-Admin-Test", "mmc-pack.json"),
		[]byte(packJSON), 0644)

	// List launchers to verify auto-detection
	resList := adminCtx.Run("launcher", "list")
	resList.AssertSuccess()
	resList.AssertStdoutContains("Prism-Admin-Test", "Prism Launcher")

	// Direct sync into detected launcher instance
	resSyncLauncher := adminCtx.Run("launcher", "sync", "Prism-Admin-Test")
	resSyncLauncher.AssertSuccess()
	resSyncLauncher.AssertStdoutContains("Successfully synchronized modpack into Prism Launcher instance 'Prism-Admin-Test'")

	// Verify local Prism mods directory has jars
	localPrismModsDir := filepath.Join(prismDir, "mods")
	prismEntries, err := os.ReadDir(localPrismModsDir)
	if err != nil || len(prismEntries) == 0 {
		t.Fatalf("expected mods in local Prism instance: %v", err)
	}

	// Step D: Compatibility Audit against Remote Oracle Cloud Server
	resDiffPre := adminCtx.Run("diff", "--url", oracleURL, "--token", oracleAuthToken)
	resDiffPre.AssertSuccess()
	resDiffPre.AssertStdoutContains("[MISSING]", "lithium", "sodium")

	// Step E: Deploy Remotely with cmm push
	resPush := adminCtx.Run("push",
		"--url", oracleURL,
		"--token", oracleAuthToken,
		"--include-config",
	)
	resPush.AssertSuccess()
	resPush.AssertStdoutContains("Successfully pushed modpack to remote server.")

	// Step F: Post-Deployment Audit with cmm diff
	resDiffPost := adminCtx.Run("diff", "--url", oracleURL, "--token", oracleAuthToken, "--json")
	resDiffPost.AssertSuccess()

	var diffResult sync.DiffResult
	if err := json.Unmarshal([]byte(resDiffPost.Stdout), &diffResult); err != nil {
		t.Fatalf("failed to unmarshal post-push diff JSON: %v", err)
	}

	if diffResult.Mismatches != 0 {
		t.Errorf("expected 0 mismatches after deployment, got %d", diffResult.Mismatches)
	}
	if diffResult.Missing != 0 {
		t.Errorf("expected 0 missing after deployment, got %d", diffResult.Missing)
	}
	if diffResult.Synchronized < 2 {
		t.Errorf("expected at least 2 synchronized mods, got %d", diffResult.Synchronized)
	}

	// Step G: Verify Oracle Cloud Server Daemon State
	// 1. Configs were extracted
	serverConfig := filepath.Join(oracleServerDir, "config", "lithium.properties")
	if data, err := os.ReadFile(serverConfig); err != nil || !strings.Contains(string(data), "chunk.serialization=true") {
		t.Fatalf("server did not receive and extract config files: %v", err)
	}

	// 2. Server mods were downloaded
	serverModsDir := filepath.Join(oracleServerDir, "mods")
	serverEntries, err := os.ReadDir(serverModsDir)
	if err != nil || len(serverEntries) == 0 {
		t.Fatalf("server daemon did not download synchronized mods into server mods directory: %v", err)
	}
}
