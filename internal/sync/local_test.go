package sync

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"cmm/internal/config"
	"cmm/internal/modrinth"
)

func sha512Hex(data []byte) string {
	sum := sha512.Sum512(data)
	return hex.EncodeToString(sum[:])
}

func TestSyncLocal_ExtensionFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Valid jars
	os.WriteFile(filepath.Join(modsDir, "active.jar"), []byte("active-data"), 0644)
	os.WriteFile(filepath.Join(modsDir, "inactive.jar.disabled"), []byte("inactive-data"), 0644)

	// Non-jar files that should be strictly ignored even if ending in .disabled
	os.WriteFile(filepath.Join(modsDir, "notes.disabled"), []byte("text data"), 0644)
	os.WriteFile(filepath.Join(modsDir, "options.txt.disabled"), []byte("options"), 0644)
	os.WriteFile(filepath.Join(modsDir, "readme.txt"), []byte("readme"), 0644)
	os.WriteFile(filepath.Join(modsDir, "config.json.disabled"), []byte("{}"), 0644)

	syncer := NewLocalSynchronizer(nil, cfgPath, lockPath)
	res, err := syncer.Sync(LocalSyncOptions{Path: modsDir})
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// Since client is nil, valid jars will end up in UnknownJars
	if len(res.UnknownJars) != 2 {
		t.Fatalf("expected exactly 2 jars recognized (active.jar and inactive.jar.disabled), got %d: %v", len(res.UnknownJars), res.UnknownJars)
	}

	for _, j := range res.UnknownJars {
		if j != "active.jar" && j != "inactive.jar.disabled" {
			t.Errorf("unexpected file in UnknownJars: %s", j)
		}
	}
}

func TestSyncLocal_AuthoritativeDiskState_ReEnable(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	modData := []byte("sodium-fabric-0.5.8-content")
	modHash := sha512Hex(modData)

	// User has renamed sodium.jar.disabled -> sodium.jar on disk
	activePath := filepath.Join(modsDir, "sodium.jar")
	if err := os.WriteFile(activePath, modData, 0644); err != nil {
		t.Fatal(err)
	}

	// Lockfile still has previous disabled state
	initialLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "sodium",
				Name:          "Sodium",
				ProjectID:     "P-SODIUM",
				VersionID:     "V-SODIUM-1",
				VersionNumber: "0.5.8",
				FileName:      "sodium.jar.disabled",
				Disabled:      true,
			},
		},
	}
	if err := config.SaveLockfile(lockPath, initialLock); err != nil {
		t.Fatal(err)
	}

	// Mock Modrinth
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/version_files", func(w http.ResponseWriter, r *http.Request) {
		res := map[string]modrinth.Version{
			modHash: {
				ID:            "V-SODIUM-1",
				ProjectID:     "P-SODIUM",
				Name:          "Sodium",
				VersionNumber: "0.5.8",
				Files: []modrinth.VersionFile{
					{
						Filename: "sodium.jar",
						Primary:  true,
						Hashes:   map[string]string{"sha512": modHash},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(res)
	})
	mux.HandleFunc("/v2/project/P-SODIUM", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(modrinth.Project{
			ID:    "P-SODIUM",
			Slug:  "sodium",
			Title: "Sodium",
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, err := modrinth.NewClientWithToken(srv.URL+"/v2", "CMM-Test/1.0", "")
	if err != nil {
		t.Fatal(err)
	}

	syncer := NewLocalSynchronizer(client, cfgPath, lockPath)
	res, err := syncer.Sync(LocalSyncOptions{Path: modsDir})
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if len(res.AddedMods) != 1 {
		t.Fatalf("expected 1 recognized mod, got: %v", res.AddedMods)
	}

	updatedLock, err := config.LoadLockfile(lockPath)
	if err != nil {
		t.Fatal(err)
	}

	mod := updatedLock.GetMod("sodium")
	if mod == nil {
		t.Fatalf("sodium mod missing in lockfile")
	}

	if mod.Disabled {
		t.Errorf("CRITICAL: mod.Disabled should be false because active sodium.jar is on disk!")
	}
	if mod.FileName != "sodium.jar" {
		t.Errorf("expected FileName 'sodium.jar', got %q", mod.FileName)
	}
}

func TestSyncLocal_AuthoritativeDiskState_Disable(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	modData := []byte("lithium-fabric-0.11.2-content")
	modHash := sha512Hex(modData)

	// User disabled mod on disk by renaming to .jar.disabled
	disabledPath := filepath.Join(modsDir, "lithium.jar.disabled")
	if err := os.WriteFile(disabledPath, modData, 0644); err != nil {
		t.Fatal(err)
	}

	// Lockfile previously had it enabled
	initialLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "lithium",
				Name:          "Lithium",
				ProjectID:     "P-LITHIUM",
				VersionID:     "V-LITHIUM-1",
				VersionNumber: "0.11.2",
				FileName:      "lithium.jar",
				Disabled:      false,
			},
		},
	}
	if err := config.SaveLockfile(lockPath, initialLock); err != nil {
		t.Fatal(err)
	}

	// Mock Modrinth
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/version_files", func(w http.ResponseWriter, r *http.Request) {
		res := map[string]modrinth.Version{
			modHash: {
				ID:            "V-LITHIUM-1",
				ProjectID:     "P-LITHIUM",
				Name:          "Lithium",
				VersionNumber: "0.11.2",
				Files: []modrinth.VersionFile{
					{
						Filename: "lithium.jar",
						Primary:  true,
						Hashes:   map[string]string{"sha512": modHash},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(res)
	})
	mux.HandleFunc("/v2/project/P-LITHIUM", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(modrinth.Project{
			ID:    "P-LITHIUM",
			Slug:  "lithium",
			Title: "Lithium",
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, err := modrinth.NewClientWithToken(srv.URL+"/v2", "CMM-Test/1.0", "")
	if err != nil {
		t.Fatal(err)
	}

	syncer := NewLocalSynchronizer(client, cfgPath, lockPath)
	res, err := syncer.Sync(LocalSyncOptions{Path: modsDir})
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if len(res.AddedMods) != 1 {
		t.Fatalf("expected 1 recognized mod, got: %v", res.AddedMods)
	}

	updatedLock, err := config.LoadLockfile(lockPath)
	if err != nil {
		t.Fatal(err)
	}

	mod := updatedLock.GetMod("lithium")
	if mod == nil {
		t.Fatalf("lithium mod missing in lockfile")
	}

	if !mod.Disabled {
		t.Errorf("CRITICAL: mod.Disabled should be true because lithium.jar.disabled is on disk!")
	}
	if mod.FileName != "lithium.jar.disabled" {
		t.Errorf("expected FileName 'lithium.jar.disabled', got %q", mod.FileName)
	}
}

func TestSyncLocal_ZeroByteJars(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create 0-byte jars
	os.WriteFile(filepath.Join(modsDir, "empty1.jar"), []byte{}, 0644)
	os.WriteFile(filepath.Join(modsDir, "empty2.jar"), []byte{}, 0644)

	var modrinthQueriedEmptyHash atomic.Bool
	emptySha512 := sha512Hex([]byte{})

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/version_files", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Hashes []string `json:"hashes"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		for _, h := range req.Hashes {
			if strings.EqualFold(h, emptySha512) {
				modrinthQueriedEmptyHash.Store(true)
			}
		}
		json.NewEncoder(w).Encode(map[string]modrinth.Version{})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, err := modrinth.NewClientWithToken(srv.URL+"/v2", "CMM-Test/1.0", "")
	if err != nil {
		t.Fatal(err)
	}

	syncer := NewLocalSynchronizer(client, cfgPath, lockPath)
	res, err := syncer.Sync(LocalSyncOptions{Path: modsDir})
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if modrinthQueriedEmptyHash.Load() {
		t.Errorf("CRITICAL: Modrinth was queried with the empty-file SHA-512 hash!")
	}

	if len(res.EmptyJars) != 2 {
		t.Errorf("expected 2 empty jars recorded, got %d: %v", len(res.EmptyJars), res.EmptyJars)
	}

	// 0-byte jars should not be considered normal unknown jars
	if len(res.UnknownJars) != 0 {
		t.Errorf("expected 0 UnknownJars for 0-byte files, got %d: %v", len(res.UnknownJars), res.UnknownJars)
	}
}

func TestSyncLocal_UnreadableFiles(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("skipping unreadable file test when running as root")
	}

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	unreadableFile := filepath.Join(modsDir, "locked.jar")
	if err := os.WriteFile(unreadableFile, []byte("forbidden data"), 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(unreadableFile, 0644) // ensure cleanup succeeds

	syncer := NewLocalSynchronizer(nil, cfgPath, lockPath)
	res, err := syncer.Sync(LocalSyncOptions{Path: modsDir})
	if err != nil {
		t.Fatalf("sync should not crash on unreadable file: %v", err)
	}

	if len(res.UnreadableFiles) != 1 || res.UnreadableFiles[0] != "locked.jar" {
		t.Errorf("expected locked.jar in UnreadableFiles, got: %v", res.UnreadableFiles)
	}

	if len(res.UnknownJars) != 0 {
		t.Errorf("unreadable file should not be reported as UnknownJars, got: %v", res.UnknownJars)
	}
}
