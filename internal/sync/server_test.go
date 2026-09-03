package sync

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cmm/internal/config"
)

func TestServer_AuthSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	os.WriteFile(lockPath, []byte("[mods]\n"), 0644)

	srv := NewServer(0, "test-token", lockPath)
	mux := http.NewServeMux()
	mux.HandleFunc("/lock", srv.HandleLock)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/lock", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "[mods]\n" {
		t.Fatalf("expected '[mods]\\n', got %q", string(body))
	}
}

func TestServer_AuthMissingOrInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	os.WriteFile(lockPath, []byte("[mods]\n"), 0644)

	srv := NewServer(0, "test-token", lockPath)
	mux := http.NewServeMux()
	mux.HandleFunc("/lock", srv.HandleLock)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Missing header
	req1, _ := http.NewRequest("GET", ts.URL+"/lock", nil)
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", resp1.StatusCode)
	}

	// Wrong token
	req2, _ := http.NewRequest("GET", ts.URL+"/lock", nil)
	req2.Header.Set("Authorization", "Bearer wrong-token")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", resp2.StatusCode)
	}
}

func TestServer_AnonymousWhenNoToken(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	os.WriteFile(lockPath, []byte("[mods]\n"), 0644)

	srv := NewServer(0, "", lockPath)
	mux := http.NewServeMux()
	mux.HandleFunc("/lock", srv.HandleLock)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/lock", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
}

func TestServer_HealthEndpoint(t *testing.T) {
	srv := NewServer(0, "", "")
	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.HandleHealth)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
}

func TestServer_GracefulShutdownHelper(t *testing.T) {
	srv := NewServer(0, "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func TestServer_HandlePush_Auth(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")
	configDir := filepath.Join(tmpDir, "config")

	srv := NewServer(0, "secret-token", lockPath)
	srv.ModsDir = modsDir
	srv.ConfigDir = configDir

	mux := http.NewServeMux()
	mux.HandleFunc("/push", srv.HandlePush)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	lockContent := "[[mods]]\nslug = \"fabric-api\"\nname = \"Fabric API\"\nversion = \"0.92.0\"\nside = \"both\"\n"

	// 1. Unauthorized - no token
	var buf1 bytes.Buffer
	w1 := multipart.NewWriter(&buf1)
	p1, _ := w1.CreateFormFile("lockfile", "cmm.lock")
	p1.Write([]byte(lockContent))
	w1.Close()

	req1, _ := http.NewRequest("POST", ts.URL+"/push", &buf1)
	req1.Header.Set("Content-Type", w1.FormDataContentType())
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", resp1.StatusCode)
	}
	if !strings.Contains(resp1.Header.Get("WWW-Authenticate"), `Bearer realm="cmm"`) {
		t.Fatalf("expected WWW-Authenticate header, got %q", resp1.Header.Get("WWW-Authenticate"))
	}

	// 2. Unauthorized - wrong token
	var buf2 bytes.Buffer
	w2 := multipart.NewWriter(&buf2)
	p2, _ := w2.CreateFormFile("lockfile", "cmm.lock")
	p2.Write([]byte(lockContent))
	w2.Close()

	req2, _ := http.NewRequest("POST", ts.URL+"/push", &buf2)
	req2.Header.Set("Content-Type", w2.FormDataContentType())
	req2.Header.Set("Authorization", "Bearer bad-token")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", resp2.StatusCode)
	}

	// 3. Authorized - correct token
	var buf3 bytes.Buffer
	w3 := multipart.NewWriter(&buf3)
	p3, _ := w3.CreateFormFile("lockfile", "cmm.lock")
	p3.Write([]byte(lockContent))
	w3.Close()

	req3, _ := http.NewRequest("POST", ts.URL+"/push", &buf3)
	req3.Header.Set("Content-Type", w3.FormDataContentType())
	req3.Header.Set("Authorization", "Bearer secret-token")
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp3.Body)
		t.Fatalf("expected 200 OK, got %d: %s", resp3.StatusCode, string(body))
	}
}

func TestServer_HandlePush_LockfileIngestion(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")
	configDir := filepath.Join(tmpDir, "config")

	srv := NewServer(0, "", lockPath)
	srv.ModsDir = modsDir
	srv.ConfigDir = configDir

	mux := http.NewServeMux()
	mux.HandleFunc("/push", srv.HandlePush)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	lockContent := "[[mods]]\nslug = \"sodium\"\nname = \"Sodium\"\nversion = \"0.5.8\"\nside = \"both\"\n"

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("lockfile", "cmm.lock")
	part.Write([]byte(lockContent))
	w.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/push", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(body))
	}

	var pResp PushResponse
	if err := json.NewDecoder(resp.Body).Decode(&pResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !pResp.Success {
		t.Fatalf("expected Success=true, got false")
	}

	// Verify lockfile on disk
	savedBytes, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("failed to read persisted lockfile: %v", err)
	}
	parsed, err := config.ParseLockfile(savedBytes)
	if err != nil {
		t.Fatalf("failed to parse persisted lockfile: %v", err)
	}
	if len(parsed.Mods) != 1 || parsed.Mods[0].Slug != "sodium" {
		t.Fatalf("unexpected persisted mods: %+v", parsed.Mods)
	}
}

func TestServer_HandlePush_ZipSlipRejection(t *testing.T) {
	maliciousPaths := []string{
		"../evil.txt",
		"..\\evil.txt",
		"config/../../evil.txt",
		"overrides/config/../../../evil.txt",
		"nested/../../evil.txt",
	}

	for _, malPath := range maliciousPaths {
		t.Run(malPath, func(t *testing.T) {
			tmpDir := t.TempDir()
			lockPath := filepath.Join(tmpDir, "cmm.lock")
			modsDir := filepath.Join(tmpDir, "mods")
			configDir := filepath.Join(tmpDir, "config")
			evilTarget := filepath.Join(tmpDir, "evil.txt")

			srv := NewServer(0, "", lockPath)
			srv.ModsDir = modsDir
			srv.ConfigDir = configDir

			mux := http.NewServeMux()
			mux.HandleFunc("/push", srv.HandlePush)
			ts := httptest.NewServer(mux)
			defer ts.Close()

			// Create malicious zip
			var zipBuf bytes.Buffer
			zw := zip.NewWriter(&zipBuf)
			f, err := zw.Create(malPath)
			if err != nil {
				t.Fatalf("failed to create zip entry: %v", err)
			}
			f.Write([]byte("malicious content"))
			zw.Close()

			// Multipart request
			var buf bytes.Buffer
			w := multipart.NewWriter(&buf)
			lp, _ := w.CreateFormFile("lockfile", "cmm.lock")
			lp.Write([]byte("[mods]\n"))
			cp, _ := w.CreateFormFile("config", "config.zip")
			cp.Write(zipBuf.Bytes())
			w.Close()

			req, _ := http.NewRequest("POST", ts.URL+"/push", &buf)
			req.Header.Set("Content-Type", w.FormDataContentType())
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400 Bad Request for zip-slip entry '%s', got %d", malPath, resp.StatusCode)
			}

			// Verify malicious file was not written outside target
			if _, err := os.Stat(evilTarget); !os.IsNotExist(err) {
				t.Fatalf("SECURITY FAILED: evil.txt was created on disk!")
			}
		})
	}
}

func TestServer_HandlePush_ConfigExtraction(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")
	configDir := filepath.Join(tmpDir, "config")

	srv := NewServer(0, "", lockPath)
	srv.ModsDir = modsDir
	srv.ConfigDir = configDir

	mux := http.NewServeMux()
	mux.HandleFunc("/push", srv.HandlePush)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Create valid zip with config files
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	f1, _ := zw.Create("server.properties")
	f1.Write([]byte("motd=CMM Server\n"))
	f2, _ := zw.Create("config/nested/settings.json")
	f2.Write([]byte("{\"enabled\": true}\n"))
	zw.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	lp, _ := w.CreateFormFile("lockfile", "cmm.lock")
	lp.Write([]byte("[mods]\n"))
	cp, _ := w.CreateFormFile("config", "config.zip")
	cp.Write(zipBuf.Bytes())
	w.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/push", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(body))
	}

	var pResp PushResponse
	if err := json.NewDecoder(resp.Body).Decode(&pResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if pResp.ConfigsUpdated != 2 {
		t.Fatalf("expected ConfigsUpdated=2, got %d", pResp.ConfigsUpdated)
	}

	// Verify extracted files
	file1Bytes, err := os.ReadFile(filepath.Join(configDir, "server.properties"))
	if err != nil || string(file1Bytes) != "motd=CMM Server\n" {
		t.Fatalf("extracted file1 invalid: %v, content: %s", err, string(file1Bytes))
	}
	file2Bytes, err := os.ReadFile(filepath.Join(configDir, "nested", "settings.json"))
	if err != nil || string(file2Bytes) != "{\"enabled\": true}\n" {
		t.Fatalf("extracted file2 invalid: %v, content: %s", err, string(file2Bytes))
	}
}

func TestServer_HandlePush_SideFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")
	configDir := filepath.Join(tmpDir, "config")

	srv := NewServer(0, "", lockPath)
	srv.ModsDir = modsDir
	srv.ConfigDir = configDir

	mux := http.NewServeMux()
	mux.HandleFunc("/push", srv.HandlePush)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	lockContent := `
[[mods]]
slug = "sodium"
name = "Sodium"
file_name = "sodium-0.5.8.jar"
version = "0.5.8"
side = "client"

[[mods]]
slug = "fabric-api"
name = "Fabric API"
file_name = "fabric-api-0.92.0.jar"
version = "0.92.0"
side = "both"

[[mods]]
slug = "luckperms"
name = "LuckPerms"
file_name = "luckperms-5.4.102.jar"
version = "5.4.102"
side = "server"
`

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	lp, _ := w.CreateFormFile("lockfile", "cmm.lock")
	lp.Write([]byte(lockContent))
	w.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/push", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(body))
	}

	// Verify mods dir has server/both mods but NOT client-only mod
	if _, err := os.Stat(filepath.Join(modsDir, "sodium-0.5.8.jar")); !os.IsNotExist(err) {
		t.Fatalf("client-only mod sodium-0.5.8.jar should NOT be installed on server daemon")
	}
	if _, err := os.Stat(filepath.Join(modsDir, "fabric-api-0.92.0.jar")); err != nil {
		t.Fatalf("both mod fabric-api-0.92.0.jar should be installed on server daemon: %v", err)
	}
	if _, err := os.Stat(filepath.Join(modsDir, "luckperms-5.4.102.jar")); err != nil {
		t.Fatalf("server mod luckperms-5.4.102.jar should be installed on server daemon: %v", err)
	}

	// Verify cmm.lock on disk contains ALL mods
	savedLock, err := config.LoadLockfile(lockPath)
	if err != nil {
		t.Fatalf("failed to load saved lockfile: %v", err)
	}
	if len(savedLock.Mods) != 3 {
		t.Fatalf("expected 3 mods in persisted cmm.lock, got %d", len(savedLock.Mods))
	}
}

func TestServer_HandlePush_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")
	configDir := filepath.Join(tmpDir, "config")

	srv := NewServer(0, "", lockPath)
	srv.ModsDir = modsDir
	srv.ConfigDir = configDir

	mux := http.NewServeMux()
	mux.HandleFunc("/push", srv.HandlePush)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	lockContent := `
[[mods]]
slug = "fabric-api"
name = "Fabric API"
file_name = "fabric-api-0.92.0.jar"
version = "0.92.0"
side = "both"
`

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	lp, _ := w.CreateFormFile("lockfile", "cmm.lock")
	lp.Write([]byte(lockContent))
	w.WriteField("dry_run", "true")
	w.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/push", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(body))
	}

	var pResp PushResponse
	if err := json.NewDecoder(resp.Body).Decode(&pResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !pResp.Success || !strings.Contains(pResp.Message, "Dry-run") {
		t.Fatalf("expected dry run response, got: %+v", pResp)
	}

	// Verify nothing was written to disk
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("dry run must NOT create lockfile on disk")
	}
	if _, err := os.Stat(filepath.Join(modsDir, "fabric-api-0.92.0.jar")); !os.IsNotExist(err) {
		t.Fatalf("dry run must NOT create mod files on disk")
	}
}

func TestServer_HandlePush_InvalidLockfile(t *testing.T) {
	tmpDir := t.TempDir()
	srv := NewServer(0, "", filepath.Join(tmpDir, "cmm.lock"))

	mux := http.NewServeMux()
	mux.HandleFunc("/push", srv.HandlePush)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	lp, _ := w.CreateFormFile("lockfile", "cmm.lock")
	lp.Write([]byte("corrupted = = = toml [["))
	w.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/push", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for corrupted lockfile, got %d", resp.StatusCode)
	}
}
