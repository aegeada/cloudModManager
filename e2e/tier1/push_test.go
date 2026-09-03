package tier1

import (
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
		time.Sleep(40 * time.Millisecond)
	}
	t.Fatalf("daemon on port %d did not start within %v", port, maxWait)
}

func startServeDaemon(t *testing.T, dir string, port int, token string, mockServerURL string) (*exec.Cmd, func()) {
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

func TestPush_SuccessWithToken(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()

	// 1. Setup Server Daemon
	serverDir := t.TempDir()
	serverPort := getFreePort(t)
	token := "auth-token-xyz-123"

	serverCfg := `[profile]
name = "server-pack"
minecraft_version = "1.21.1"
loader = "fabric"
loader_version = "0.16.5"
side = "server"
`
	_ = os.WriteFile(filepath.Join(serverDir, "cmm.toml"), []byte(serverCfg), 0644)
	_ = os.WriteFile(filepath.Join(serverDir, "cmm.lock"), []byte("[mods]\n"), 0644)

	_, cleanup := startServeDaemon(t, serverDir, serverPort, token, ms.URL())
	defer cleanup()

	// 2. Setup Client Project
	ctx := harness.NewTestContext(t, ms.URL())
	ctx.WriteFile("cmm.toml", `[profile]
name = "client-pack"
minecraft_version = "1.21.1"
loader = "fabric"
loader_version = "0.16.5"
side = "both"
`)
	clientLock := fmt.Sprintf(`[[mods]]
name = "Lithium"
slug = "lithium"
file_name = "lithium-fabric-0.12.0.jar"
side = "both"
version = "0.12.0"
download_url = "%s/download/lithium-fabric-0.12.0.jar"
`, ms.URL())
	ctx.WriteFile("cmm.lock", clientLock)

	// 3. Execute push command
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", serverPort)
	res := ctx.Run("push", "--url", serverURL, "--token", token)
	res.AssertSuccess()
	res.AssertStdoutContains("Successfully pushed modpack to remote server.")

	// 4. Verify server cmm.lock was updated
	serverLockBytes, err := os.ReadFile(filepath.Join(serverDir, "cmm.lock"))
	if err != nil {
		t.Fatalf("failed to read server lockfile: %v", err)
	}
	serverLockStr := string(serverLockBytes)
	if !strings.Contains(serverLockStr, "lithium") || !strings.Contains(serverLockStr, "0.12.0") {
		t.Fatalf("server lockfile was not updated with client mods: %s", serverLockStr)
	}

	// Verify server downloaded the mod into mods directory
	serverModFile := filepath.Join(serverDir, "mods", "lithium-fabric-0.12.0.jar")
	if _, err := os.Stat(serverModFile); err != nil {
		t.Fatalf("expected server to download mod file '%s': %v", serverModFile, err)
	}
}

func TestPush_UnauthorizedWithoutToken(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()

	serverDir := t.TempDir()
	serverPort := getFreePort(t)
	token := "secure-super-token-456"

	_ = os.WriteFile(filepath.Join(serverDir, "cmm.toml"), []byte("side = \"server\"\n"), 0644)
	_ = os.WriteFile(filepath.Join(serverDir, "cmm.lock"), []byte("[mods]\n"), 0644)

	_, cleanup := startServeDaemon(t, serverDir, serverPort, token, ms.URL())
	defer cleanup()

	ctx := harness.NewTestContext(t, ms.URL())
	ctx.WriteFile("cmm.toml", "side = \"both\"\n")
	ctx.WriteFile("cmm.lock", "[[mods]]\nname = \"mod-a\"\n")

	serverURL := fmt.Sprintf("http://127.0.0.1:%d", serverPort)

	// Case 1: Push without token
	resNoToken := ctx.Run("push", "--url", serverURL)
	resNoToken.AssertFailure()

	// Case 2: Push with invalid token
	resWrongToken := ctx.Run("push", "--url", serverURL, "--token", "invalid-token")
	resWrongToken.AssertFailure()
}

func TestPush_WithConfigArchive(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()

	serverDir := t.TempDir()
	serverPort := getFreePort(t)
	token := "config-push-token-789"

	_ = os.WriteFile(filepath.Join(serverDir, "cmm.toml"), []byte("side = \"server\"\n"), 0644)
	_ = os.WriteFile(filepath.Join(serverDir, "cmm.lock"), []byte("[mods]\n"), 0644)

	_, cleanup := startServeDaemon(t, serverDir, serverPort, token, ms.URL())
	defer cleanup()

	ctx := harness.NewTestContext(t, ms.URL())
	ctx.WriteFile("cmm.toml", "side = \"both\"\n")
	clientLock := fmt.Sprintf(`[[mods]]
name = "Sodium"
slug = "sodium"
file_name = "sodium-fabric-0.5.8.jar"
side = "client"
download_url = "%s/download/sodium-fabric-0.5.8.jar"
`, ms.URL())
	ctx.WriteFile("cmm.lock", clientLock)

	// Create client config directory with multiple files
	ctx.WriteFile("config/mymod.json", `{"key": "remote-value", "enabled": true}`)
	ctx.WriteFile("config/nested/settings.toml", "difficulty = \"hard\"\nrender_distance = 16\n")

	serverURL := fmt.Sprintf("http://127.0.0.1:%d", serverPort)
	res := ctx.Run("push", "--url", serverURL, "--token", token, "--include-config")
	res.AssertSuccess()
	res.AssertStdoutContains("Configs Updated")

	// Verify server config files were created and match contents
	serverConfigFile := filepath.Join(serverDir, "config", "mymod.json")
	data, err := os.ReadFile(serverConfigFile)
	if err != nil {
		t.Fatalf("expected server config file '%s' to exist: %v", serverConfigFile, err)
	}
	if !strings.Contains(string(data), "remote-value") {
		t.Fatalf("unexpected content in server config: %s", string(data))
	}

	serverNestedFile := filepath.Join(serverDir, "config", "nested", "settings.toml")
	dataNested, err := os.ReadFile(serverNestedFile)
	if err != nil {
		t.Fatalf("expected nested server config file '%s' to exist: %v", serverNestedFile, err)
	}
	if !strings.Contains(string(dataNested), "difficulty = \"hard\"") {
		t.Fatalf("unexpected content in nested server config: %s", string(dataNested))
	}
}

func TestPush_DryRun(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()

	serverDir := t.TempDir()
	serverPort := getFreePort(t)
	token := "dryrun-token-999"

	initialServerLock := `[[mods]]
name = "Initial Server Mod"
slug = "server-only-mod"
file_name = "server-mod-1.0.0.jar"
side = "server"
`
	_ = os.WriteFile(filepath.Join(serverDir, "cmm.toml"), []byte("side = \"server\"\n"), 0644)
	_ = os.WriteFile(filepath.Join(serverDir, "cmm.lock"), []byte(initialServerLock), 0644)

	_, cleanup := startServeDaemon(t, serverDir, serverPort, token, ms.URL())
	defer cleanup()

	ctx := harness.NewTestContext(t, ms.URL())
	ctx.WriteFile("cmm.toml", "side = \"both\"\n")
	clientLock := fmt.Sprintf(`[[mods]]
name = "New Client Mod"
slug = "sodium"
file_name = "sodium-fabric-0.5.8.jar"
side = "both"
download_url = "%s/download/sodium-fabric-0.5.8.jar"
`, ms.URL())
	ctx.WriteFile("cmm.lock", clientLock)
	ctx.WriteFile("config/dryrun.txt", "should not be written")

	serverURL := fmt.Sprintf("http://127.0.0.1:%d", serverPort)
	res := ctx.Run("push", "--url", serverURL, "--token", token, "--include-config", "--dry-run")
	res.AssertSuccess()
	res.AssertStdoutContains("[Dry-Run]")

	// Verify server lockfile was NOT changed
	serverLockBytes, err := os.ReadFile(filepath.Join(serverDir, "cmm.lock"))
	if err != nil {
		t.Fatalf("failed to read server lockfile: %v", err)
	}
	if string(serverLockBytes) != initialServerLock {
		t.Fatalf("server lockfile was modified during dry run: %s", string(serverLockBytes))
	}

	// Verify server config file was NOT created
	if _, err := os.Stat(filepath.Join(serverDir, "config", "dryrun.txt")); !os.IsNotExist(err) {
		t.Fatalf("dry run created config file on server")
	}
}
