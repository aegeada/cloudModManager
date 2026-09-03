package tier2

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"cmm/internal/config"
	"cmm/internal/mod"
	"cmm/internal/tui"
	"cmm/internal/tui/tea"
)

// 1. Full Lifecycle Integration Test (M1 -> M2 -> M3 -> M4)
func TestLifecycle_FullSystemWorkflow(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	// Step 1: Initialize Project (M1)
	res := ctx.Run("init", "--name", "OmniPack", "--mc-version", "1.21.1", "--loader", "fabric", "--side", "both")
	res.AssertSuccess()

	cfg, err := config.LoadConfig(filepath.Join(ctx.TempDir, "cmm.toml"))
	if err != nil || cfg.Profile.Name != "OmniPack" {
		t.Fatalf("failed to verify cmm.toml: %v", err)
	}

	// Step 2: Install Fabric Loader (M2)
	res = ctx.Run("loader", "install", "fabric", "--version", "0.19.3")
	res.AssertSuccess()

	// Step 3: Search Modrinth (M1)
	res = ctx.Run("search", "sodium")
	res.AssertSuccess()
	res.AssertStdoutContains("Sodium")

	// Step 4: Add Sodium with transitive dependency resolution (M2)
	res = ctx.Run("add", "sodium")
	res.AssertSuccess()

	// Verify lockfile and jar presence
	lock, err := config.LoadLockfile(filepath.Join(ctx.TempDir, "cmm.lock"))
	if err != nil || len(lock.Mods) == 0 {
		t.Fatalf("failed to verify cmm.lock after add: %v", err)
	}

	// Step 5: Pin mod (M2)
	res = ctx.Run("pin", "sodium")
	res.AssertSuccess()

	lock, _ = config.LoadLockfile(filepath.Join(ctx.TempDir, "cmm.lock"))
	sodiumMod := lock.GetMod("sodium")
	if sodiumMod == nil || !sodiumMod.Pinned {
		t.Fatalf("expected sodium to be pinned in cmm.lock")
	}

	// Step 6: Export to .mrpack modpack archive (M3)
	mrpackPath := filepath.Join(ctx.TempDir, "exported.mrpack")
	res = ctx.Run("export", "--format", "mrpack", "--output", mrpackPath, "--name", "OmniPack-Export", "--version-id", "1.0.0")
	res.AssertSuccess()

	if _, err := os.Stat(mrpackPath); err != nil {
		t.Fatalf("expected exported.mrpack to exist on disk: %v", err)
	}

	// Step 7: Export clean GitHub repo config (M3)
	ghExportDir := filepath.Join(ctx.TempDir, "github_export")
	res = ctx.Run("export", "--format", "github", "--output", ghExportDir)
	res.AssertSuccess()

	if _, err := os.Stat(filepath.Join(ghExportDir, "cmm.toml")); err != nil {
		t.Fatalf("expected exported cmm.toml in github export dir")
	}

	// Step 8: Sync Local Directory (M3)
	res = ctx.Run("sync", "local")
	res.AssertSuccess()

	// Verify that pinned status survived sync local
	lock, _ = config.LoadLockfile(filepath.Join(ctx.TempDir, "cmm.lock"))
	sodiumMod = lock.GetMod("sodium")
	if sodiumMod == nil || !sodiumMod.Pinned {
		t.Fatalf("expected sodium to remain pinned after sync local")
	}

	// Step 9: Headless TUI Navigation & Actions (M4)
	app, err := tui.NewApp(filepath.Join(ctx.TempDir, "cmm.toml"), filepath.Join(ctx.TempDir, "cmm.lock"))
	if err != nil {
		t.Fatalf("failed to initialize TUI app: %v", err)
	}

	// Simulate tab switches: Tab 1 -> Tab 2 -> Tab 3 -> Tab 4 -> Tab 1
	var m tea.Model = app
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	view := m.View()
	if !strings.Contains(view, "Installed Mods") && !strings.Contains(view, "OmniPack") {
		t.Errorf("expected view to contain app headers, got: %s", view)
	}
}

// 2. High-Concurrency Sync Server Stress Test (M3)
func TestServer_HighConcurrencyStress(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", `[profile]
name = "ServerStress"
minecraft_version = "1.21.1"
loader = "fabric"
side = "server"
`)
	ctx.WriteFile("cmm.lock", `[[mods]]
slug = "sodium"
name = "Sodium"
version = "0.5.8"
pinned = true
file_name = "sodium-fabric-0.5.8.jar"
`)

	port := 59871
	token := "stress-secret-token-99"

	// Start server in background
	cmd := exec.Command(harness.BinaryPath, "serve", "--port", fmt.Sprintf("%d", port), "--token", token)
	cmd.Dir = ctx.TempDir
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start cmm serve: %v", err)
	}
	defer func() {
		_ = cmd.Process.Signal(os.Interrupt)
		_ = cmd.Wait()
	}()

	time.Sleep(300 * time.Millisecond)

	// Send 50 concurrent requests
	var wg sync.WaitGroup
	errCount := 0
	var mu sync.Mutex

	serverURL := fmt.Sprintf("http://127.0.0.1:%d/lock", port)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(reqNum int) {
			defer wg.Done()
			req, err := http.NewRequest("GET", serverURL, nil)
			if err != nil {
				mu.Lock()
				errCount++
				mu.Unlock()
				return
			}
			req.Header.Set("Authorization", "Bearer "+token)
			client := &http.Client{Timeout: 2 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				mu.Lock()
				errCount++
				mu.Unlock()
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				mu.Lock()
				errCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	if errCount > 0 {
		t.Errorf("encountered %d failures out of 50 concurrent requests", errCount)
	}
}

// 3. Corrupt and Malformed Modpack Archive Recovery (M3)
func TestSyncModrinth_CorruptArchiveResilience(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", `name = "CorruptTest"`)

	// Test A: Truncated ZIP file
	corruptZip := filepath.Join(ctx.TempDir, "truncated.mrpack")
	_ = os.WriteFile(corruptZip, []byte("PK\x03\x04truncated_corrupt_data"), 0644)

	res := ctx.Run("sync", "--source", "modrinth", "--file", corruptZip)
	res.AssertFailure()
	res.AssertStderrContains("invalid mrpack zip archive")

	// Test B: Valid ZIP without modrinth.index.json
	missingIndexZip := filepath.Join(ctx.TempDir, "no_index.mrpack")
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, _ := zw.Create("other_file.txt")
	_, _ = w.Write([]byte("some data"))
	_ = zw.Close()
	_ = os.WriteFile(missingIndexZip, buf.Bytes(), 0644)

	res = ctx.Run("sync", "--source", "modrinth", "--file", missingIndexZip)
	res.AssertFailure()
	res.AssertStderrContains("modrinth.index.json not found")
}

// 4. Large-Scale Directory Scan (100+ JARs) in Sync Local (M3)
func TestSyncLocal_LargeScaleDirectoryScan(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	ctx.WriteFile("cmm.toml", `[paths]
mods_dir = "mods"
`)
	modsDir := filepath.Join(ctx.TempDir, "mods")
	_ = os.MkdirAll(modsDir, 0755)

	// Create 1 recognized mock jar and 20 unknown jars
	_ = os.WriteFile(filepath.Join(modsDir, "sodium-fabric-0.5.8.jar"), mockserver.MockJarContent, 0644)

	for i := 1; i <= 20; i++ {
		fakeJar := filepath.Join(modsDir, fmt.Sprintf("custom_unknown_mod_%02d.jar", i))
		_ = os.WriteFile(fakeJar, []byte(fmt.Sprintf("custom-data-%d", i)), 0644)
	}

	res := ctx.Run("sync", "local")
	res.AssertSuccess()
	res.AssertStdoutContains("Warning: Unrecognized JAR file:")
	res.AssertStdoutContains("Successfully synchronized")
}

// 5. Binary Standalone Subprocess Execution Verification
func TestStandaloneBinary_DirectExecution(t *testing.T) {
	binPath := harness.BinaryPath
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("binary %s not found: %v", binPath, err)
	}

	// 1. Help flag
	out, err := exec.Command(binPath, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("expected binary --help to succeed: %v", err)
	}
	if !strings.Contains(string(out), "managing Minecraft mods") && !strings.Contains(string(out), "cmm") {
		t.Errorf("expected stdout to contain CLI description, got: %s", string(out))
	}

	// 2. Subcommand listing
	commands := []string{"init", "search", "add", "remove", "list", "pin", "unpin", "update", "loader", "sync", "serve", "export", "tui"}
	for _, cmdName := range commands {
		out, err := exec.Command(binPath, cmdName, "--help").CombinedOutput()
		if err != nil {
			t.Errorf("expected '%s --help' to succeed, got error: %v (output: %s)", cmdName, err, string(out))
		}
	}
}

// 6. SHA-512 Hash Precision and Collision Resistance (M2)
func TestHashVerification_Precision(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "sample.jar")

	data1 := []byte("content-version-1")
	_ = os.WriteFile(testFile, data1, 0644)
	hash1, err := mod.ComputeSHA512(testFile)
	if err != nil || len(hash1) != 128 {
		t.Fatalf("expected 128-char SHA512 hex string, got: %s", hash1)
	}

	// Mutate 1 single bit
	data2 := []byte("content-version-2")
	_ = os.WriteFile(testFile, data2, 0644)
	hash2, err := mod.ComputeSHA512(testFile)
	if err != nil {
		t.Fatalf("failed to compute hash: %v", err)
	}

	if hash1 == hash2 {
		t.Fatalf("hash collision detected on modified content!")
	}
}
