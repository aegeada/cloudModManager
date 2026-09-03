package sync

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushSynchronizer_Push_Success(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	lockContent := "[[mods]]\nslug = \"sodium\"\nversion = \"0.5.8\"\n"
	if err := os.WriteFile(lockPath, []byte(lockContent), 0644); err != nil {
		t.Fatalf("failed to write test lockfile: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/push" {
			t.Errorf("expected path /push, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer valid-token" {
			t.Errorf("expected Bearer valid-token, got %s", r.Header.Get("Authorization"))
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("expected multipart/form-data content type, got %s", r.Header.Get("Content-Type"))
		}

		err := r.ParseMultipartForm(32 << 20)
		if err != nil {
			t.Errorf("failed to parse multipart form: %v", err)
		}
		f, _, err := r.FormFile("lockfile")
		if err != nil {
			t.Errorf("missing lockfile in form: %v", err)
		}
		defer f.Close()
		content, _ := io.ReadAll(f)
		if string(content) != lockContent {
			t.Errorf("expected lock content %q, got %q", lockContent, string(content))
		}

		resp := PushResult{
			Success:        true,
			Message:        "Push synchronized successfully",
			AddedMods:      []string{"sodium-0.5.8.jar"},
			UpdatedMods:    []string{},
			PrunedMods:     []string{},
			ConfigsUpdated: 0,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	syncer := NewPushSynchronizer("", lockPath, "")
	res, err := syncer.Push(PushOptions{
		URL:   ts.URL,
		Token: "valid-token",
	})
	if err != nil {
		t.Fatalf("push failed: %v", err)
	}
	if !res.Success || len(res.AddedMods) != 1 || res.AddedMods[0] != "sodium-0.5.8.jar" {
		t.Fatalf("unexpected push result: %+v", res)
	}
}

func TestPushSynchronizer_Push_WithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	os.WriteFile(lockPath, []byte("[mods]\n"), 0644)

	configDir := filepath.Join(tmpDir, "config")
	os.MkdirAll(filepath.Join(configDir, "sub"), 0755)
	os.WriteFile(filepath.Join(configDir, "server.properties"), []byte("motd=Test"), 0644)
	os.WriteFile(filepath.Join(configDir, "sub", "mod.json"), []byte("{}"), 0644)

	receivedConfig := false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(32 << 20)
		f, _, err := r.FormFile("config")
		if err == nil {
			defer f.Close()
			data, _ := io.ReadAll(f)
			zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err == nil && len(zr.File) >= 2 {
				receivedConfig = true
			}
		}

		resp := PushResult{
			Success:        true,
			ConfigsUpdated: 2,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	syncer := NewPushSynchronizer("", lockPath, configDir)
	res, err := syncer.Push(PushOptions{
		URL:           ts.URL,
		IncludeConfig: true,
		ConfigDir:     configDir,
	})
	if err != nil {
		t.Fatalf("push with config failed: %v", err)
	}
	if !receivedConfig {
		t.Fatalf("server did not receive expected config archive")
	}
	if res.ConfigsUpdated != 2 {
		t.Fatalf("expected ConfigsUpdated=2, got %d", res.ConfigsUpdated)
	}
}

func TestPushSynchronizer_Push_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	os.WriteFile(lockPath, []byte("[mods]\n"), 0644)

	dryRunReceived := false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(32 << 20)
		if r.FormValue("dry_run") == "true" {
			dryRunReceived = true
		}

		resp := PushResult{
			Success: true,
			Message: "Dry-run simulation completed successfully",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	syncer := NewPushSynchronizer("", lockPath, "")
	res, err := syncer.Push(PushOptions{
		URL:    ts.URL,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("push failed: %v", err)
	}
	if !dryRunReceived {
		t.Fatalf("server did not receive dry_run parameter")
	}
	if !strings.Contains(res.Message, "Dry-run") {
		t.Fatalf("expected dry-run message, got: %s", res.Message)
	}
}

func TestPushSynchronizer_Push_AuthError(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	os.WriteFile(lockPath, []byte("[mods]\n"), 0644)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="cmm"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}))
	defer ts.Close()

	syncer := NewPushSynchronizer("", lockPath, "")
	_, err := syncer.Push(PushOptions{
		URL:   ts.URL,
		Token: "wrong-token",
	})
	if err == nil {
		t.Fatalf("expected auth error, got nil")
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("expected unauthorized error message, got: %v", err)
	}
}

func TestPushSynchronizer_Push_ServerError(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	os.WriteFile(lockPath, []byte("[mods]\n"), 0644)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal delta sync failed", http.StatusInternalServerError)
	}))
	defer ts.Close()

	syncer := NewPushSynchronizer("", lockPath, "")
	_, err := syncer.Push(PushOptions{
		URL: ts.URL,
	})
	if err == nil {
		t.Fatalf("expected server error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 500") || !strings.Contains(err.Error(), "Internal delta sync failed") {
		t.Fatalf("expected HTTP 500 error, got: %v", err)
	}
}

func TestPushSynchronizer_Push_MissingURL(t *testing.T) {
	syncer := NewPushSynchronizer("", "", "")
	_, err := syncer.Push(PushOptions{
		URL: "",
	})
	if err == nil {
		t.Fatalf("expected error on missing URL, got nil")
	}
}

func TestPushSynchronizer_Push_MissingLockfile(t *testing.T) {
	syncer := NewPushSynchronizer("", "/non/existent/cmm.lock", "")
	_, err := syncer.Push(PushOptions{
		URL: "http://127.0.0.1:8080",
	})
	if err == nil {
		t.Fatalf("expected error on missing lockfile, got nil")
	}
}

func TestPackageConfigZip(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	os.MkdirAll(filepath.Join(configDir, "sub"), 0755)
	os.WriteFile(filepath.Join(configDir, "a.txt"), []byte("alpha"), 0644)
	os.WriteFile(filepath.Join(configDir, "sub", "b.txt"), []byte("beta"), 0644)

	zipBytes, err := PackageConfigZip(configDir)
	if err != nil {
		t.Fatalf("failed to package config zip: %v", err)
	}
	if len(zipBytes) == 0 {
		t.Fatalf("expected non-empty zip bytes")
	}

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("failed to read created zip: %v", err)
	}

	foundA := false
	foundB := false
	for _, f := range zr.File {
		if f.Name == "a.txt" {
			foundA = true
		}
		if f.Name == "sub/b.txt" {
			foundB = true
		}
	}
	if !foundA || !foundB {
		t.Fatalf("expected files a.txt and sub/b.txt in zip, got %+v", zr.File)
	}

	// Test non-existent dir returns nil, nil
	emptyBytes, err := PackageConfigZip(filepath.Join(tmpDir, "non_existent"))
	if err != nil || emptyBytes != nil {
		t.Fatalf("expected nil for non existent dir, got err=%v, bytes=%v", err, emptyBytes)
	}
}
