package sync

import (
	"archive/zip"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"cmm/internal/config"
	"cmm/internal/modrinth"
)

func hash512Str(data []byte) string {
	sum := sha512.Sum512(data)
	return hex.EncodeToString(sum[:])
}

// createZipFile is a helper to build arbitrary ZIP archives for testing
func createZipFile(t *testing.T, destPath string, files map[string][]byte) {
	t.Helper()
	_ = os.MkdirAll(filepath.Dir(destPath), 0755)
	f, err := os.Create(destPath)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create entry %s in zip: %v", name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("failed to write content for %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
}

// ============================================================================
// 1. R1: LOCAL SYNC ADVERSARIAL TESTS
// ============================================================================

func TestSyncLocal_Adversarial_NonExistentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods") // does not exist yet

	cfg := &config.Config{
		Profile: config.Profile{MinecraftVersion: "1.21.1", Loader: "fabric", Side: "server"},
		Paths:   config.Paths{ModsDir: modsDir},
	}
	config.SaveConfig(cfgPath, cfg)

	syncer := NewLocalSynchronizer(nil, cfgPath, lockPath)

	// Explicit non-existent path override -> MUST return error
	_, err := syncer.Sync(LocalSyncOptions{Path: filepath.Join(tmpDir, "non_existent_folder")})
	if err == nil {
		t.Errorf("expected error for non-existent explicit path, got nil")
	} else if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' error, got: %v", err)
	}

	// Default non-existent directory -> should report "No mods found" gracefully
	res, err := syncer.Sync(LocalSyncOptions{})
	if err != nil {
		t.Errorf("expected nil error for default missing mods dir, got: %v", err)
	}
	if res.Message != "No mods found" {
		t.Errorf("expected 'No mods found', got: %q", res.Message)
	}
}

func TestSyncLocal_Adversarial_FileInsteadOfDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	regularFile := filepath.Join(tmpDir, "regular_file.txt")
	os.WriteFile(regularFile, []byte("i am a file not a directory"), 0644)

	syncer := NewLocalSynchronizer(nil, cfgPath, lockPath)
	_, err := syncer.Sync(LocalSyncOptions{Path: regularFile})
	if err == nil {
		t.Fatalf("expected error when path is a file, got nil")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("expected error containing 'not a directory', got: %v", err)
	}
}

func TestSyncLocal_Adversarial_EmptyAndNonJarFiles(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")
	os.MkdirAll(modsDir, 0755)

	// Create subdirectories, non-jar files, hidden files
	os.MkdirAll(filepath.Join(modsDir, "subdir.jar"), 0755) // is a dir despite .jar suffix!
	os.WriteFile(filepath.Join(modsDir, "readme.txt"), []byte("readme"), 0644)
	os.WriteFile(filepath.Join(modsDir, ".DS_Store"), []byte("meta"), 0644)
	os.WriteFile(filepath.Join(modsDir, "archive.zip"), []byte("zipdata"), 0644)

	syncer := NewLocalSynchronizer(nil, cfgPath, lockPath)
	res, err := syncer.Sync(LocalSyncOptions{Path: modsDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Message != "No mods found" {
		t.Errorf("expected 'No mods found' when no real jar files exist, got %q", res.Message)
	}
}

func TestSyncLocal_Adversarial_UnrecognizedJarsAndPinPreservation(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")
	os.MkdirAll(modsDir, 0755)

	mod1Data := []byte("mod1-fabric-api-data")
	mod1Hash := hash512Str(mod1Data)
	os.WriteFile(filepath.Join(modsDir, "fabric-api-0.92.0.jar"), mod1Data, 0644)

	unknownData := []byte("unknown-custom-mod-data")
	os.WriteFile(filepath.Join(modsDir, "custom-unknown-mod.jar"), unknownData, 0644)

	// Seed existing lockfile with pinned status for fabric-api
	initialLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "fabric-api",
				Name:          "Fabric API",
				ProjectID:     "P-FABRIC",
				VersionID:     "V-OLD",
				VersionNumber: "0.90.0",
				FileName:      "fabric-api-0.90.0.jar",
				Pinned:        true,
			},
		},
	}
	config.SaveLockfile(lockPath, initialLock)

	// Setup mock Modrinth server recognizing fabric-api
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/version_files", func(w http.ResponseWriter, r *http.Request) {
		var reqBody struct {
			Hashes    []string `json:"hashes"`
			Algorithm string   `json:"algorithm"`
		}
		_ = json.NewDecoder(r.Body).Decode(&reqBody)

		res := make(map[string]modrinth.Version)
		for _, h := range reqBody.Hashes {
			if strings.EqualFold(h, mod1Hash) {
				res[h] = modrinth.Version{
					ID:            "V-NEW",
					ProjectID:     "P-FABRIC",
					Name:          "Fabric API",
					VersionNumber: "0.92.0",
					Files: []modrinth.VersionFile{
						{
							Filename: "fabric-api-0.92.0.jar",
							Primary:  true,
							Hashes:   map[string]string{"sha512": mod1Hash},
							URL:      "http://" + r.Host + "/files/fabric-api-0.92.0.jar",
						},
					},
				}
			}
		}
		json.NewEncoder(w).Encode(res)
	})
	mux.HandleFunc("/v2/project/P-FABRIC", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(modrinth.Project{
			ID:    "P-FABRIC",
			Slug:  "fabric-api",
			Title: "Fabric API",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := modrinth.NewClientWithToken(server.URL+"/v2", "CMM-Test/1.0", "")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	syncer := NewLocalSynchronizer(client, cfgPath, lockPath)
	res, err := syncer.Sync(LocalSyncOptions{Path: modsDir})
	if err != nil {
		t.Fatalf("local sync failed: %v", err)
	}

	// Verify unknown jar detected
	if len(res.UnknownJars) != 1 || res.UnknownJars[0] != "custom-unknown-mod.jar" {
		t.Errorf("expected custom-unknown-mod.jar in UnknownJars, got %v", res.UnknownJars)
	}

	// Verify lockfile saved with preserved pin
	savedLock, err := config.LoadLockfile(lockPath)
	if err != nil {
		t.Fatalf("failed to load saved lockfile: %v", err)
	}

	modEntry := savedLock.GetMod("fabric-api")
	if modEntry == nil {
		modEntry = savedLock.GetMod("Fabric API")
	}
	if modEntry == nil {
		t.Fatalf("fabric-api not found in lockfile after sync: %+v", savedLock.Mods)
	}
	if !modEntry.Pinned {
		t.Errorf("CRITICAL: pinned status was NOT preserved! Got pinned=false, expected true")
	}
	if modEntry.VersionNumber != "0.92.0" {
		t.Errorf("expected version 0.92.0, got %s", modEntry.VersionNumber)
	}
}

// ============================================================================
// 2. R2: MODRINTH / .MRPACK SYNC ADVERSARIAL TESTS
// ============================================================================

func TestSyncModrinth_Adversarial_CorruptedAndMalformedMrpack(t *testing.T) {
	tmpDir := t.TempDir()

	// Case 1: Corrupted non-zip file
	corruptZipPath := filepath.Join(tmpDir, "corrupted.mrpack")
	os.WriteFile(corruptZipPath, []byte("NOT A ZIP FILE HEADER AT ALL GARBAGE DATA"), 0644)

	_, err := ParseMrpack(corruptZipPath)
	if err == nil {
		t.Errorf("expected error for corrupted non-zip archive, got nil")
	} else if !strings.Contains(err.Error(), "invalid mrpack zip archive") {
		t.Errorf("expected 'invalid mrpack zip archive' error, got: %v", err)
	}

	// Case 2: Valid ZIP without modrinth.index.json
	missingIndexPath := filepath.Join(tmpDir, "missing_index.mrpack")
	createZipFile(t, missingIndexPath, map[string][]byte{
		"overrides/config.txt": []byte("some config"),
		"README.md":            []byte("hello"),
	})

	_, err = ParseMrpack(missingIndexPath)
	if err == nil {
		t.Errorf("expected error when modrinth.index.json is missing, got nil")
	} else if !strings.Contains(err.Error(), "modrinth.index.json not found") {
		t.Errorf("expected 'modrinth.index.json not found' error, got: %v", err)
	}

	// Case 3: Valid ZIP with malformed modrinth.index.json (invalid JSON syntax)
	malformedIndexPath := filepath.Join(tmpDir, "malformed_index.mrpack")
	createZipFile(t, malformedIndexPath, map[string][]byte{
		"modrinth.index.json": []byte("{ formatVersion: 1, broken json syntax missing quotes"),
	})

	_, err = ParseMrpack(malformedIndexPath)
	if err == nil {
		t.Errorf("expected error for malformed json, got nil")
	} else if !strings.Contains(err.Error(), "failed to decode modrinth.index.json") {
		t.Errorf("expected 'failed to decode modrinth.index.json' error, got: %v", err)
	}
}

func TestSyncModrinth_Adversarial_SideFiltering_Strict(t *testing.T) {
	// Tests server-only, client-only, both, optional filtering
	jarData1 := []byte("mod-both-content")
	jarData2 := []byte("mod-server-only-content")
	jarData3 := []byte("mod-client-only-content")
	jarData4 := []byte("mod-optional-content")

	hash1 := hash512Str(jarData1)
	hash2 := hash512Str(jarData2)
	hash3 := hash512Str(jarData3)
	hash4 := hash512Str(jarData4)

	mux := http.NewServeMux()
	mux.HandleFunc("/files/mod-both.jar", func(w http.ResponseWriter, r *http.Request) { w.Write(jarData1) })
	mux.HandleFunc("/files/mod-server.jar", func(w http.ResponseWriter, r *http.Request) { w.Write(jarData2) })
	mux.HandleFunc("/files/mod-client.jar", func(w http.ResponseWriter, r *http.Request) { w.Write(jarData3) })
	mux.HandleFunc("/files/mod-opt.jar", func(w http.ResponseWriter, r *http.Request) { w.Write(jarData4) })

	server := httptest.NewServer(mux)
	defer server.Close()

	index := ModpackIndex{
		FormatVersion: 1,
		Game:          "minecraft",
		VersionID:     "1.0.0",
		Name:          "Test Pack",
		Files: []ModpackFile{
			{
				Path:      "mods/mod-both.jar",
				Hashes:    map[string]string{"sha512": hash1},
				Env:       &ModpackFileEnv{Client: "required", Server: "required"},
				Downloads: []string{server.URL + "/files/mod-both.jar"},
			},
			{
				Path:      "mods/mod-server.jar",
				Hashes:    map[string]string{"sha512": hash2},
				Env:       &ModpackFileEnv{Client: "unsupported", Server: "required"},
				Downloads: []string{server.URL + "/files/mod-server.jar"},
			},
			{
				Path:      "mods/mod-client.jar",
				Hashes:    map[string]string{"sha512": hash3},
				Env:       &ModpackFileEnv{Client: "required", Server: "unsupported"},
				Downloads: []string{server.URL + "/files/mod-client.jar"},
			},
			{
				Path:      "mods/mod-opt.jar",
				Hashes:    map[string]string{"sha512": hash4},
				Env:       &ModpackFileEnv{Client: "optional", Server: "optional"},
				Downloads: []string{server.URL + "/files/mod-opt.jar"},
			},
		},
	}

	indexBytes, _ := json.Marshal(index)
	tmpDir := t.TempDir()
	mrpackPath := filepath.Join(tmpDir, "pack.mrpack")
	createZipFile(t, mrpackPath, map[string][]byte{"modrinth.index.json": indexBytes})

	client, _ := modrinth.NewClientWithToken(server.URL+"/v2", "CMM-Test/1.0", "")

	// 1. Test side = "server"
	{
		serverDir := filepath.Join(tmpDir, "server_env")
		cfgPath := filepath.Join(serverDir, "cmm.toml")
		lockPath := filepath.Join(serverDir, "cmm.lock")
		modsDir := filepath.Join(serverDir, "mods")

		cfg := &config.Config{
			Profile: config.Profile{MinecraftVersion: "1.21.1", Loader: "fabric", Side: "server"},
			Paths:   config.Paths{ModsDir: modsDir},
		}
		config.SaveConfig(cfgPath, cfg)

		syncer := NewModrinthSynchronizer(client, cfgPath, lockPath)
		res, err := syncer.Sync(ModrinthSyncOptions{FilePath: mrpackPath})
		if err != nil {
			t.Fatalf("server side sync failed: %v", err)
		}

		// Should install: mod-both, mod-server, mod-opt (3 mods). mod-client must be omitted!
		if len(res.AddedMods) != 3 {
			t.Errorf("expected 3 mods on server, got %d: %v", len(res.AddedMods), res.AddedMods)
		}

		if _, err := os.Stat(filepath.Join(modsDir, "mod-client.jar")); !os.IsNotExist(err) {
			t.Errorf("CRITICAL: mod-client.jar was installed in server environment!")
		}
		if _, err := os.Stat(filepath.Join(modsDir, "mod-server.jar")); err != nil {
			t.Errorf("mod-server.jar should be installed in server environment")
		}
		if _, err := os.Stat(filepath.Join(modsDir, "mod-both.jar")); err != nil {
			t.Errorf("mod-both.jar should be installed in server environment")
		}
	}

	// 2. Test side = "client"
	{
		clientDir := filepath.Join(tmpDir, "client_env")
		cfgPath := filepath.Join(clientDir, "cmm.toml")
		lockPath := filepath.Join(clientDir, "cmm.lock")
		modsDir := filepath.Join(clientDir, "mods")

		cfg := &config.Config{
			Profile: config.Profile{MinecraftVersion: "1.21.1", Loader: "fabric", Side: "client"},
			Paths:   config.Paths{ModsDir: modsDir},
		}
		config.SaveConfig(cfgPath, cfg)

		syncer := NewModrinthSynchronizer(client, cfgPath, lockPath)
		res, err := syncer.Sync(ModrinthSyncOptions{FilePath: mrpackPath})
		if err != nil {
			t.Fatalf("client side sync failed: %v", err)
		}

		// Should install: mod-both, mod-client, mod-opt (3 mods). mod-server must be omitted!
		if len(res.AddedMods) != 3 {
			t.Errorf("expected 3 mods on client, got %d: %v", len(res.AddedMods), res.AddedMods)
		}

		if _, err := os.Stat(filepath.Join(modsDir, "mod-server.jar")); !os.IsNotExist(err) {
			t.Errorf("CRITICAL: mod-server.jar was installed in client environment!")
		}
		if _, err := os.Stat(filepath.Join(modsDir, "mod-client.jar")); err != nil {
			t.Errorf("mod-client.jar should be installed in client environment")
		}
	}
}

func TestSyncModrinth_Adversarial_DeltaPruningAndPinPreservation(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")
	os.MkdirAll(modsDir, 0755)

	// Seed extraneous jar
	os.WriteFile(filepath.Join(modsDir, "unmanaged-old.jar"), []byte("garbage"), 0644)

	// Seed existing pinned mod where slug matches baseName
	sodiumOldData := []byte("sodium-0.4.0-old")
	os.WriteFile(filepath.Join(modsDir, "sodium.jar"), sodiumOldData, 0644)

	initialLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:     "sodium",
				Name:     "Sodium",
				FileName: "sodium.jar",
				Pinned:   true,
			},
			{
				Slug:     "unmanaged-old",
				Name:     "Unmanaged",
				FileName: "unmanaged-old.jar",
				Pinned:   false,
			},
		},
	}
	config.SaveLockfile(lockPath, initialLock)

	// Modpack has sodium.jar (new version) and iris.jar
	sodiumNewData := []byte("sodium-0.5.8-new")
	irisData := []byte("iris-1.7.0")
	sodiumNewHash := hash512Str(sodiumNewData)
	irisHash := hash512Str(irisData)

	mux := http.NewServeMux()
	mux.HandleFunc("/files/sodium.jar", func(w http.ResponseWriter, r *http.Request) { w.Write(sodiumNewData) })
	mux.HandleFunc("/files/iris.jar", func(w http.ResponseWriter, r *http.Request) { w.Write(irisData) })
	server := httptest.NewServer(mux)
	defer server.Close()

	index := ModpackIndex{
		FormatVersion: 1,
		Game:          "minecraft",
		VersionID:     "1.0.0",
		Name:          "Updated Pack",
		Files: []ModpackFile{
			{
				Path:      "mods/sodium.jar",
				Hashes:    map[string]string{"sha512": sodiumNewHash},
				Downloads: []string{server.URL + "/files/sodium.jar"},
			},
			{
				Path:      "mods/iris.jar",
				Hashes:    map[string]string{"sha512": irisHash},
				Downloads: []string{server.URL + "/files/iris.jar"},
			},
		},
	}
	indexBytes, _ := json.Marshal(index)
	mrpackPath := filepath.Join(tmpDir, "pack.mrpack")
	createZipFile(t, mrpackPath, map[string][]byte{"modrinth.index.json": indexBytes})

	cfg := &config.Config{
		Profile: config.Profile{MinecraftVersion: "1.21.1", Loader: "fabric", Side: "both"},
		Paths:   config.Paths{ModsDir: modsDir},
	}
	config.SaveConfig(cfgPath, cfg)

	client, _ := modrinth.NewClientWithToken(server.URL+"/v2", "CMM-Test/1.0", "")
	syncer := NewModrinthSynchronizer(client, cfgPath, lockPath)
	res, err := syncer.Sync(ModrinthSyncOptions{FilePath: mrpackPath})
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// 1. Verify unmanaged-old.jar was pruned
	if _, err := os.Stat(filepath.Join(modsDir, "unmanaged-old.jar")); !os.IsNotExist(err) {
		t.Errorf("expected unmanaged-old.jar to be deleted from disk")
	}

	// 2. Verify new mods downloaded
	if _, err := os.Stat(filepath.Join(modsDir, "sodium.jar")); err != nil {
		t.Errorf("expected new sodium.jar on disk")
	}
	if _, err := os.Stat(filepath.Join(modsDir, "iris.jar")); err != nil {
		t.Errorf("expected iris.jar on disk")
	}

	// 3. Verify Pinned status preserved for sodium!
	savedLock, err := config.LoadLockfile(lockPath)
	if err != nil {
		t.Fatalf("failed to load lockfile: %v", err)
	}

	sodiumMod := savedLock.GetMod("sodium")
	if sodiumMod == nil {
		t.Fatalf("sodium not found in lockfile: %+v", savedLock.Mods)
	}
	if !sodiumMod.Pinned {
		t.Errorf("CRITICAL: Sodium pin status was lost during mrpack sync!")
	}

	// Iris was NOT pinned originally, so must NOT be pinned now
	irisMod := savedLock.GetMod("iris")
	if irisMod != nil && irisMod.Pinned {
		t.Errorf("Iris was erroneously marked as pinned")
	}

	_ = res
}

// ============================================================================
// 3. R3: GITHUB SYNC ADVERSARIAL TESTS
// ============================================================================

func TestSyncGitHub_Adversarial_Auth401_404_AndMalformed(t *testing.T) {
	mux := http.NewServeMux()

	sodiumData := []byte("PK\x03\x04mock-sodium-jar-data-for-adversarial-test")
	sodiumHash := hash512Str(sodiumData)

	mux.HandleFunc("/files/sodium-fabric-0.5.8.jar", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/java-archive")
		w.Write(sodiumData)
	})

	// 404 handler
	mux.HandleFunc("/repos/org/nonexistent/contents/cmm.lock", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	})

	// 401 handler for private without correct token
	var serverURL string
	mux.HandleFunc("/repos/org/private/contents/cmm.lock", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer valid-secret-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		lockStr := fmt.Sprintf(`[[mods]]
name = "Sodium"
slug = "sodium"
version = "0.5.8"
file_name = "sodium-fabric-0.5.8.jar"
sha512 = "%s"
download_url = "%s/files/sodium-fabric-0.5.8.jar"
`, sodiumHash, serverURL)
		w.Write([]byte(lockStr))
	})

	// Malformed lockfile handler
	mux.HandleFunc("/repos/org/malformed/contents/cmm.lock", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("this is completely invalid TOML [ broken [["))
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	serverURL = server.URL

	os.Setenv("GITHUB_API_URL", server.URL)
	defer os.Unsetenv("GITHUB_API_URL")

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")

	cfg := &config.Config{
		Profile: config.Profile{MinecraftVersion: "1.21.1", Loader: "fabric", Side: "server"},
		Paths:   config.Paths{ModsDir: modsDir},
	}
	config.SaveConfig(cfgPath, cfg)

	client, err := modrinth.NewClientWithToken(server.URL+"/v2", "CMM-Test/1.0", "")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	syncer := NewGitHubSynchronizer(client, cfgPath, lockPath)

	// 1. Missing repo
	_, err = syncer.Sync(GitHubSyncOptions{})
	if err == nil || !strings.Contains(err.Error(), "--repo") {
		t.Errorf("expected error requiring --repo, got: %v", err)
	}

	// 2. 404 Not Found
	_, err = syncer.Sync(GitHubSyncOptions{Repo: "org/nonexistent"})
	if err == nil {
		t.Errorf("expected 404 error, got nil")
	} else if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected error containing '404', got: %v", err)
	}

	// 3. Private repo without token -> 401
	_, err = syncer.Sync(GitHubSyncOptions{Repo: "org/private"})
	if err == nil {
		t.Errorf("expected 401 error without token, got nil")
	} else if !strings.Contains(err.Error(), "401") && !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected authentication failed error, got: %v", err)
	}

	// 4. Private repo with invalid token -> 401
	_, err = syncer.Sync(GitHubSyncOptions{Repo: "org/private", Token: "wrong-token"})
	if err == nil {
		t.Errorf("expected 401 error with wrong token, got nil")
	}

	// 5. Malformed remote lockfile -> parse error
	_, err = syncer.Sync(GitHubSyncOptions{Repo: "org/malformed"})
	if err == nil {
		t.Errorf("expected parse error for malformed lockfile, got nil")
	} else if !strings.Contains(err.Error(), "failed to parse remote lockfile") {
		t.Errorf("expected 'failed to parse remote lockfile', got: %v", err)
	}

	// 6. Private repo with valid token -> SUCCESS
	res, err := syncer.Sync(GitHubSyncOptions{Repo: "org/private", Token: "valid-secret-token"})
	if err != nil {
		t.Fatalf("expected success with valid token, got: %v", err)
	}
	if len(res.AddedMods) != 1 {
		t.Errorf("expected 1 added mod, got %d", len(res.AddedMods))
	}
}

// ============================================================================
// 4. R5: REMOTE SERVER SYNC ADVERSARIAL TESTS
// ============================================================================

func TestSyncRemote_Adversarial_NetworkFailuresAndAuth(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")

	cfg := &config.Config{
		Profile: config.Profile{MinecraftVersion: "1.21.1", Loader: "fabric", Side: "server"},
		Paths:   config.Paths{ModsDir: modsDir},
	}
	config.SaveConfig(cfgPath, cfg)

	syncer := NewRemoteSynchronizer(nil, cfgPath, lockPath)

	// 1. Missing URL
	_, err := syncer.Sync(RemoteSyncOptions{})
	if err == nil || !strings.Contains(err.Error(), "--url") {
		t.Errorf("expected error requiring --url, got: %v", err)
	}

	// 2. Unreachable server (closed port / invalid host)
	_, err = syncer.Sync(RemoteSyncOptions{URL: "http://127.0.0.1:59123"})
	if err == nil {
		t.Errorf("expected error for unreachable server, got nil")
	} else if !strings.Contains(err.Error(), "remote server unreachable") {
		t.Errorf("expected 'remote server unreachable' error, got: %v", err)
	}

	// 3. Server returning HTTP 500
	mux500 := http.NewServeMux()
	mux500.HandleFunc("/lock", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	})
	srv500 := httptest.NewServer(mux500)
	defer srv500.Close()

	_, err = syncer.Sync(RemoteSyncOptions{URL: srv500.URL})
	if err == nil {
		t.Errorf("expected error for HTTP 500, got nil")
	} else if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error mentioning 500, got: %v", err)
	}

	// 4. Server requiring Bearer token
	lithiumData := []byte("PK\x03\x04mock-lithium-jar-data-for-remote-test")
	lithiumHash := hash512Str(lithiumData)

	muxAuth := http.NewServeMux()
	muxAuth.HandleFunc("/files/lithium-fabric-0.11.2.jar", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/java-archive")
		w.Write(lithiumData)
	})
	var srvAuthURL string
	muxAuth.HandleFunc("/lock", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer remote-secret" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		lockStr := fmt.Sprintf(`[[mods]]
name = "Lithium"
slug = "lithium"
version = "0.11.2"
file_name = "lithium-fabric-0.11.2.jar"
sha512 = "%s"
download_url = "%s/files/lithium-fabric-0.11.2.jar"
`, lithiumHash, srvAuthURL)
		w.Write([]byte(lockStr))
	})
	srvAuth := httptest.NewServer(muxAuth)
	defer srvAuth.Close()
	srvAuthURL = srvAuth.URL

	client, err := modrinth.NewClientWithToken(srvAuth.URL+"/v2", "CMM-Test/1.0", "")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	syncerWithClient := NewRemoteSynchronizer(client, cfgPath, lockPath)

	// Missing token -> 401
	_, err = syncerWithClient.Sync(RemoteSyncOptions{URL: srvAuth.URL})
	if err == nil {
		t.Errorf("expected 401 unauthorized, got nil")
	} else if !strings.Contains(err.Error(), "401") && !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("expected unauthorized error, got: %v", err)
	}

	// Wrong token -> 401
	_, err = syncerWithClient.Sync(RemoteSyncOptions{URL: srvAuth.URL, Token: "wrong"})
	if err == nil {
		t.Errorf("expected 401 with wrong token, got nil")
	}

	// Correct token -> Success
	res, err := syncerWithClient.Sync(RemoteSyncOptions{URL: srvAuth.URL, Token: "remote-secret"})
	if err != nil {
		t.Fatalf("expected success with valid token, got: %v", err)
	}
	if len(res.AddedMods) != 1 {
		t.Errorf("expected 1 added mod, got %d", len(res.AddedMods))
	}
}

func TestSyncRemote_Adversarial_TimeoutHandling(t *testing.T) {
	// Server hangs / delays beyond client timeout
	muxHang := http.NewServeMux()
	muxHang.HandleFunc("/lock", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(6 * time.Second)
		w.Write([]byte("[mods]\n"))
	})
	srvHang := httptest.NewServer(muxHang)
	defer srvHang.Close()

	tmpDir := t.TempDir()
	syncer := NewRemoteSynchronizer(nil, filepath.Join(tmpDir, "cmm.toml"), filepath.Join(tmpDir, "cmm.lock"))

	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := syncer.Sync(RemoteSyncOptions{URL: srvHang.URL})
		done <- err
	}()

	select {
	case <-ctx.Done():
		t.Fatal("test hung; client timeout did not trigger properly")
	case err := <-done:
		if err == nil {
			t.Fatalf("expected timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "unreachable") && !strings.Contains(err.Error(), "Client.Timeout") && !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Errorf("expected timeout/unreachable error, got: %v", err)
		}
	}
}

func TestServer_Adversarial_ConcurrentRequestsAndAuth(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	lockContent := "[[mods]]\nname = \"sodium\"\nversion = \"0.5.8\"\n"
	os.WriteFile(lockPath, []byte(lockContent), 0644)

	srv := NewServer(0, "concurrency-token", lockPath)
	mux := http.NewServeMux()
	mux.HandleFunc("/lock", srv.HandleLock)
	mux.HandleFunc("/health", srv.HandleHealth)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 50 concurrent requests
	var wg sync.WaitGroup
	errCount := 0
	var mu sync.Mutex

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req, _ := http.NewRequest("GET", ts.URL+"/lock", nil)
			if idx%2 == 0 {
				req.Header.Set("Authorization", "Bearer concurrency-token")
			} else {
				req.Header.Set("Authorization", "Bearer wrong-token")
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				mu.Lock()
				errCount++
				mu.Unlock()
				return
			}
			defer resp.Body.Close()

			if idx%2 == 0 {
				if resp.StatusCode != http.StatusOK {
					mu.Lock()
					errCount++
					mu.Unlock()
				}
			} else {
				if resp.StatusCode != http.StatusUnauthorized {
					mu.Lock()
					errCount++
					mu.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()
	if errCount > 0 {
		t.Errorf("encountered %d errors during concurrent server stress test", errCount)
	}
}

func TestSyncLocal_Adversarial_ToggleDiskExtensionStress(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	modData := []byte("mod-toggle-stress-data")
	modHash := hash512Str(modData)

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/version_files", func(w http.ResponseWriter, r *http.Request) {
		res := map[string]modrinth.Version{
			modHash: {
				ID:            "V-TOGGLE",
				ProjectID:     "P-TOGGLE",
				Name:          "Toggle Mod",
				VersionNumber: "1.0.0",
				Files: []modrinth.VersionFile{
					{
						Filename: "toggle.jar",
						Primary:  true,
						Hashes:   map[string]string{"sha512": modHash},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(res)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, err := modrinth.NewClientWithToken(srv.URL+"/v2", "CMM-Test/1.0", "")
	if err != nil {
		t.Fatal(err)
	}

	syncer := NewLocalSynchronizer(client, cfgPath, lockPath)

	for cycle := 0; cycle < 10; cycle++ {
		// 1. Set as active (.jar)
		activePath := filepath.Join(modsDir, "toggle.jar")
		disabledPath := filepath.Join(modsDir, "toggle.jar.disabled")
		_ = os.Remove(disabledPath)
		if err := os.WriteFile(activePath, modData, 0644); err != nil {
			t.Fatal(err)
		}

		_, err := syncer.Sync(LocalSyncOptions{Path: modsDir})
		if err != nil {
			t.Fatalf("cycle %d active sync failed: %v", cycle, err)
		}

		lock1, err := config.LoadLockfile(lockPath)
		if err != nil {
			t.Fatal(err)
		}
		m1 := lock1.GetMod("P-TOGGLE")
		if m1 == nil || m1.Disabled {
			t.Fatalf("cycle %d: expected Disabled=false for toggle.jar, got %+v", cycle, m1)
		}

		// 2. Set as disabled (.jar.disabled)
		_ = os.Remove(activePath)
		if err := os.WriteFile(disabledPath, modData, 0644); err != nil {
			t.Fatal(err)
		}

		_, err = syncer.Sync(LocalSyncOptions{Path: modsDir})
		if err != nil {
			t.Fatalf("cycle %d disabled sync failed: %v", cycle, err)
		}

		lock2, err := config.LoadLockfile(lockPath)
		if err != nil {
			t.Fatal(err)
		}
		m2 := lock2.GetMod("P-TOGGLE")
		if m2 == nil || !m2.Disabled {
			t.Fatalf("cycle %d: expected Disabled=true for toggle.jar.disabled, got %+v", cycle, m2)
		}
	}
}

func TestDeltaEngine_Adversarial_CorruptDownloadAndZeroOrphanFiles(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	_ = os.MkdirAll(modsDir, 0755)

	initialModData := []byte("mod-to-retain")
	_ = os.WriteFile(filepath.Join(modsDir, "retain.jar"), initialModData, 0644)

	initialLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{Slug: "retain", Name: "Retain", Version: "1.0.0", FileName: "retain.jar", SHA512: hash512Str(initialModData)},
		},
	}
	config.SaveLockfile(lockPath, initialLock)

	// Mock server that truncates response or drops connection
	mux := http.NewServeMux()
	mux.HandleFunc("/files/mod1.jar", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("valid-mod-1"))
	})
	mux.HandleFunc("/files/mod2.jar", func(w http.ResponseWriter, r *http.Request) {
		// Send incorrect checksum or short payload
		w.Write([]byte("corrupt-bytes"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, _ := modrinth.NewClientWithToken(srv.URL+"/v2", "CMM-Test/1.0", "")
	engine := NewDeltaEngine(client, nil, initialLock, modsDir, lockPath)

	targets := []TargetFile{
		{
			FileName:    "mod1.jar",
			SHA512:      hash512Str([]byte("valid-mod-1")),
			DownloadURL: srv.URL + "/files/mod1.jar",
		},
		{
			FileName:    "mod2.jar",
			SHA512:      hash512Str([]byte("expected-different-full-content")),
			DownloadURL: srv.URL + "/files/mod2.jar",
		},
	}

	_, err := engine.ApplyTargetFiles(targets)
	if err == nil {
		t.Fatalf("expected error on corrupt/mismatched download, got nil")
	}

	// Verify zero .tmp files exist in modsDir
	entries, _ := os.ReadDir(modsDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") || strings.Contains(e.Name(), ".tmp.") {
			t.Errorf("found leaked temporary file: %s", e.Name())
		}
	}

	// Verify retain.jar still exists intact
	content, err := os.ReadFile(filepath.Join(modsDir, "retain.jar"))
	if err != nil || string(content) != string(initialModData) {
		t.Fatalf("retain.jar was damaged or removed: %v", err)
	}
}

func TestServer_Adversarial_PushAuthVariations(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	_ = os.WriteFile(lockPath, []byte("[mods]\n"), 0644)

	validToken := "top-secret-daemon-token"
	srv := NewServer(0, validToken, lockPath)
	mux := http.NewServeMux()
	mux.HandleFunc("/push", srv.HandlePush)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	unauthorizedHeaders := []string{
		"",
		"Bearer ",
		"Bearer      ",
		"Bearer wrong",
		"Bearer top-secret-daemon-token-extra",
		"Bearer top-secret-daemon-toke",
		"Basic dXNlcjpwYXNz",
		"Bearer " + strings.ToUpper(validToken),
	}

	for _, hdr := range unauthorizedHeaders {
		req, _ := http.NewRequest("POST", ts.URL+"/push", strings.NewReader("dummy"))
		if hdr != "" {
			req.Header.Set("Authorization", hdr)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for header %q, got %d", hdr, resp.StatusCode)
		}
		if !strings.Contains(resp.Header.Get("WWW-Authenticate"), `Bearer realm="cmm"`) {
			t.Errorf("expected WWW-Authenticate header for %q, got: %s", hdr, resp.Header.Get("WWW-Authenticate"))
		}
	}
}
