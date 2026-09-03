package tier2

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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

func sendPushMultipart(serverURL, token string, lockContent []byte, zipBuffer *bytes.Buffer, dryRun bool) (int, string, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	if lockContent != nil {
		part, err := writer.CreateFormFile("lockfile", "cmm.lock")
		if err != nil {
			return 0, "", err
		}
		_, _ = part.Write(lockContent)
	}

	if zipBuffer != nil {
		part, err := writer.CreateFormFile("config", "config.zip")
		if err != nil {
			return 0, "", err
		}
		_, _ = io.Copy(part, zipBuffer)
	}

	if dryRun {
		_ = writer.WriteField("dry_run", "true")
	}

	_ = writer.Close()

	req, err := http.NewRequest(http.MethodPost, serverURL+"/push", body)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}

func createZipWithEntries(entries map[string]string) *bytes.Buffer {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err == nil {
			_, _ = w.Write([]byte(content))
		}
	}
	_ = zw.Close()
	return buf
}

func TestAdversarial_Push_ZipSlipVectors(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()

	serverDir := t.TempDir()
	serverPort := getFreePort(t)
	token := "zipslip-token"

	_ = os.WriteFile(filepath.Join(serverDir, "cmm.toml"), []byte("side = \"server\"\n"), 0644)
	_ = os.WriteFile(filepath.Join(serverDir, "cmm.lock"), []byte("[mods]\n"), 0644)

	_, cleanup := startDaemon(t, serverDir, serverPort, token, ms.URL())
	defer cleanup()

	serverURL := fmt.Sprintf("http://127.0.0.1:%d", serverPort)
	validLock := []byte("[[mods]]\nname = \"Sodium\"\nslug = \"sodium\"\nside = \"client\"\n")

	testVectors := []struct {
		name       string
		entryPath  string
		probedPath string
	}{
		{
			name:       "POSIX directory traversal",
			entryPath:  "../../evil_posix.txt",
			probedPath: filepath.Join(serverDir, "..", "evil_posix.txt"),
		},
		{
			name:       "Windows backslash traversal",
			entryPath:  `..\..\evil_win.txt`,
			probedPath: filepath.Join(serverDir, "..", "evil_win.txt"),
		},
		{
			name:       "Nested config prefix traversal",
			entryPath:  "config/../../evil_nested.txt",
			probedPath: filepath.Join(serverDir, "..", "evil_nested.txt"),
		},
		{
			name:       "Overrides prefix traversal",
			entryPath:  "overrides/config/../../../evil_overrides.txt",
			probedPath: filepath.Join(serverDir, "..", "evil_overrides.txt"),
		},
		{
			name:       "Deep tmp directory traversal",
			entryPath:  "deep/../../../../tmp/cmm_exploit_test.txt",
			probedPath: "/tmp/cmm_exploit_test.txt",
		},
	}

	for _, tc := range testVectors {
		t.Run(tc.name, func(t *testing.T) {
			zipBuf := createZipWithEntries(map[string]string{
				tc.entryPath: "MALICIOUS_DATA_SHOULD_NOT_EXIST",
			})

			status, body, err := sendPushMultipart(serverURL, token, validLock, zipBuf, false)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}

			if status == http.StatusOK {
				t.Fatalf("expected zip-slip vector '%s' to be rejected, but got HTTP 200: %s", tc.entryPath, body)
			}

			if !strings.Contains(body, "security violation") && !strings.Contains(body, "Config extraction failed") && !strings.Contains(body, "traversal") {
				t.Logf("server returned error without traversal mention: %s", body)
			}

			// Ensure probed file was NOT created
			if _, err := os.Stat(tc.probedPath); !os.IsNotExist(err) {
				_ = os.Remove(tc.probedPath)
				t.Fatalf("CRITICAL SECURITY VULNERABILITY: file '%s' was created on filesystem via Zip-Slip!", tc.probedPath)
			}
		})
	}
}

func TestAdversarial_Push_MalformedZip(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()

	serverDir := t.TempDir()
	serverPort := getFreePort(t)
	token := "malformed-zip-token"

	_ = os.WriteFile(filepath.Join(serverDir, "cmm.toml"), []byte("side = \"server\"\n"), 0644)
	_ = os.WriteFile(filepath.Join(serverDir, "cmm.lock"), []byte("[mods]\n"), 0644)

	_, cleanup := startDaemon(t, serverDir, serverPort, token, ms.URL())
	defer cleanup()

	serverURL := fmt.Sprintf("http://127.0.0.1:%d", serverPort)
	validLock := []byte("[mods]\n")

	// Corrupted zip buffer (random garbage)
	corruptBuf := bytes.NewBuffer([]byte("PK\x03\x04corrupted-garbage-binary-stream-not-a-valid-zip"))

	status, body, err := sendPushMultipart(serverURL, token, validLock, corruptBuf, false)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if status == http.StatusOK {
		t.Fatalf("expected corrupted zip to fail, got HTTP 200: %s", body)
	}

	// Verify server remains responsive
	resp, err := http.Get(serverURL + "/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("server crashed after corrupted zip: %v", err)
	}
	_ = resp.Body.Close()
}

func TestAdversarial_Push_EmptyPayload(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()

	serverDir := t.TempDir()
	serverPort := getFreePort(t)
	token := "empty-payload-token"

	_ = os.WriteFile(filepath.Join(serverDir, "cmm.toml"), []byte("side = \"server\"\n"), 0644)
	_ = os.WriteFile(filepath.Join(serverDir, "cmm.lock"), []byte("[mods]\n"), 0644)

	_, cleanup := startDaemon(t, serverDir, serverPort, token, ms.URL())
	defer cleanup()

	serverURL := fmt.Sprintf("http://127.0.0.1:%d", serverPort)

	// 1. Missing lockfile
	status, _, err := sendPushMultipart(serverURL, token, nil, nil, false)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if status != http.StatusBadRequest {
		t.Errorf("expected HTTP 400 for missing lockfile, got %d", status)
	}

	// 2. Completely empty request body
	req, _ := http.NewRequest(http.MethodPost, serverURL+"/push", bytes.NewReader([]byte{}))
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("empty body request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected HTTP 400 for empty body, got %d", resp.StatusCode)
	}
}

func TestAdversarial_Push_ConcurrentRequests(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()

	serverDir := t.TempDir()
	serverPort := getFreePort(t)
	token := "concurrent-push-token"

	_ = os.WriteFile(filepath.Join(serverDir, "cmm.toml"), []byte("side = \"server\"\n"), 0644)
	_ = os.WriteFile(filepath.Join(serverDir, "cmm.lock"), []byte("[mods]\n"), 0644)

	_, cleanup := startDaemon(t, serverDir, serverPort, token, ms.URL())
	defer cleanup()

	serverURL := fmt.Sprintf("http://127.0.0.1:%d", serverPort)

	concurrency := 8
	var wg sync.WaitGroup
	errs := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			lockContent := []byte(fmt.Sprintf("[[mods]]\nname = \"Mod-%d\"\nslug = \"mod-%d\"\nside = \"client\"\n", id, id))
			zipBuf := createZipWithEntries(map[string]string{
				fmt.Sprintf("config/worker_%d.txt", id): fmt.Sprintf("worker %d payload", id),
			})

			status, body, err := sendPushMultipart(serverURL, token, lockContent, zipBuf, false)
			if err != nil {
				errs <- fmt.Errorf("worker %d failed: %w", id, err)
				return
			}
			if status != http.StatusOK {
				errs <- fmt.Errorf("worker %d got non-200 status %d: %s", id, status, body)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrency error: %v", err)
	}

	// Verify server remains healthy
	resp, err := http.Get(serverURL + "/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("server unhealthy after concurrent pushes: %v", err)
	}
	_ = resp.Body.Close()
}
