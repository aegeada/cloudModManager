package mod

import (
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"cmm/internal/config"
	"cmm/internal/modrinth"
)

var testJarBytes = []byte("PK\x03\x04mock-test-jar-bytes")

func testSHA512() string {
	sum := sha512.Sum512(testJarBytes)
	return hex.EncodeToString(sum[:])
}

func testSHA1() string {
	sum := sha1.Sum(testJarBytes)
	return hex.EncodeToString(sum[:])
}

func strPtr(s string) *string {
	return &s
}

func setupMockModrinthServer(t *testing.T) (*httptest.Server, *modrinth.Client) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v2/project/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/v2/project/non_existent_slug_404" {
			http.NotFound(w, r)
			return
		}

		if path == "/v2/project/sodium/version" || path == "/v2/project/AANobbMI/version" {
			versions := []modrinth.Version{
				{
					ID:            "v-sodium-0.5.8",
					ProjectID:     "AANobbMI",
					VersionNumber: "0.5.8",
					Dependencies: []modrinth.Dependency{
						{ProjectID: strPtr("P7dR8mSH"), DependencyType: "required"},
					},
					Files: []modrinth.VersionFile{
						{
							Filename: "sodium-fabric-0.5.8.jar",
							Primary:  true,
							Hashes: map[string]string{
								"sha512": testSHA512(),
								"sha1":   testSHA1(),
							},
							URL: "http://" + r.Host + "/download/sodium-fabric-0.5.8.jar",
						},
					},
				},
				{
					ID:            "v-sodium-0.5.0",
					VersionNumber: "0.5.0",
					Dependencies: []modrinth.Dependency{
						{ProjectID: strPtr("P7dR8mSH"), DependencyType: "required"},
					},
					Files: []modrinth.VersionFile{
						{
							Filename: "sodium-fabric-0.5.0.jar",
							Primary:  true,
							Hashes: map[string]string{
								"sha512": testSHA512(),
								"sha1":   testSHA1(),
							},
							URL: "http://" + r.Host + "/download/sodium-fabric-0.5.0.jar",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(versions)
			return
		}

		if path == "/v2/project/fabric-api/version" || path == "/v2/project/P7dR8mSH/version" {
			versions := []modrinth.Version{
				{
					ID:            "v-fapi-0.100.0",
					ProjectID:     "P7dR8mSH",
					VersionNumber: "0.100.0",
					Files: []modrinth.VersionFile{
						{
							Filename: "fabric-api-0.100.0.jar",
							Primary:  true,
							Hashes: map[string]string{
								"sha512": testSHA512(),
								"sha1":   testSHA1(),
							},
							URL: "http://" + r.Host + "/download/fabric-api-0.100.0.jar",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(versions)
			return
		}

		if path == "/v2/project/lithium/version" || path == "/v2/project/gvQqBUqZ/version" {
			versions := []modrinth.Version{
				{
					ID:            "v-lithium-0.11.2",
					ProjectID:     "gvQqBUqZ",
					VersionNumber: "0.11.2",
					Changelog:     "Lithium update changelog",
					Files: []modrinth.VersionFile{
						{
							Filename: "lithium-fabric-0.11.2.jar",
							Primary:  true,
							Hashes: map[string]string{
								"sha512": testSHA512(),
								"sha1":   testSHA1(),
							},
							URL: "http://" + r.Host + "/download/lithium-fabric-0.11.2.jar",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(versions)
			return
		}

		// Single project requests
		if path == "/v2/project/sodium" || path == "/v2/project/AANobbMI" {
			json.NewEncoder(w).Encode(modrinth.Project{
				ID:         "AANobbMI",
				Slug:       "sodium",
				Title:      "Sodium",
				ServerSide: "unsupported",
				ClientSide: "required",
			})
			return
		}
		if path == "/v2/project/fabric-api" || path == "/v2/project/P7dR8mSH" {
			json.NewEncoder(w).Encode(modrinth.Project{
				ID:         "P7dR8mSH",
				Slug:       "fabric-api",
				Title:      "Fabric API",
				ServerSide: "required",
				ClientSide: "required",
			})
			return
		}
		if path == "/v2/project/lithium" || path == "/v2/project/gvQqBUqZ" {
			json.NewEncoder(w).Encode(modrinth.Project{
				ID:         "gvQqBUqZ",
				Slug:       "lithium",
				Title:      "Lithium",
				ServerSide: "required",
				ClientSide: "optional",
			})
			return
		}
		if path == "/v2/project/iris" {
			json.NewEncoder(w).Encode(modrinth.Project{
				ID:         "YL57xq9U",
				Slug:       "iris",
				Title:      "Iris Shaders",
				ServerSide: "unsupported",
				ClientSide: "required",
			})
			return
		}

		http.NotFound(w, r)
	})

	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/java-archive")
		w.Write(testJarBytes)
	})

	server := httptest.NewServer(mux)

	client, err := modrinth.NewClientWithToken(server.URL+"/v2", "CMM-Test/1.0", "")
	if err != nil {
		t.Fatal(err)
	}

	return server, client
}

func TestManager_AddAndRemove(t *testing.T) {
	server, client := setupMockModrinthServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")

	cfg := &config.Config{
		Profile: config.Profile{
			Name:             "test-server",
			MinecraftVersion: "1.21.1",
			Loader:           "fabric",
			Side:             "both",
		},
		Paths: config.Paths{
			ModsDir: modsDir,
		},
	}
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(client, configPath, lockPath)

	// Add sodium (should also install fabric-api dependency)
	addRes, err := mgr.Add("sodium", "")
	if err != nil {
		t.Fatalf("mgr.Add sodium failed: %v", err)
	}
	if addRes.InstalledMod == nil || addRes.InstalledMod.Slug != "sodium" {
		t.Errorf("expected sodium installed, got %+v", addRes.InstalledMod)
	}
	if len(addRes.InstalledDeps) != 1 || addRes.InstalledDeps[0].Slug != "fabric-api" {
		t.Errorf("expected fabric-api dependency installed, got %+v", addRes.InstalledDeps)
	}

	// Verify files created on disk
	if _, err := os.Stat(filepath.Join(modsDir, "sodium-fabric-0.5.8.jar")); err != nil {
		t.Errorf("sodium jar file missing on disk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(modsDir, "fabric-api-0.100.0.jar")); err != nil {
		t.Errorf("fabric-api jar file missing on disk: %v", err)
	}

	// Verify lockfile contents
	lock, err := config.LoadLockfile(lockPath)
	if err != nil || len(lock.Mods) != 2 {
		t.Fatalf("expected 2 mods in lockfile, got %v (err: %v)", len(lock.Mods), err)
	}

	// Remove sodium with dry run
	dryRes, err := mgr.Remove("sodium", true)
	if err != nil {
		t.Fatalf("mgr.Remove dryRun failed: %v", err)
	}
	if !dryRes.DryRun || len(dryRes.OrphanedDeps) == 0 {
		t.Errorf("expected dry run with orphaned deps, got %+v", dryRes)
	}

	// Remove sodium actually
	remRes, err := mgr.Remove("sodium", false)
	if err != nil {
		t.Fatalf("mgr.Remove sodium failed: %v", err)
	}
	if len(remRes.OrphanedDeps) != 1 || remRes.OrphanedDeps[0] != "fabric-api" {
		t.Errorf("expected fabric-api orphaned, got %+v", remRes.OrphanedDeps)
	}

	// Remove orphan
	err = mgr.RemoveOrphan("fabric-api")
	if err != nil {
		t.Fatalf("RemoveOrphan failed: %v", err)
	}

	lock, err = config.LoadLockfile(lockPath)
	if err != nil || len(lock.Mods) != 0 {
		t.Errorf("expected empty lockfile after removing orphan, got %d mods", len(lock.Mods))
	}
}

func TestManager_ServerSideFiltersClientOnly(t *testing.T) {
	server, client := setupMockModrinthServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")

	cfg := &config.Config{
		Profile: config.Profile{
			Side: "server",
		},
		Paths: config.Paths{
			ModsDir: filepath.Join(tmpDir, "mods"),
		},
	}
	config.SaveConfig(configPath, cfg)

	mgr := NewManager(client, configPath, lockPath)
	res, err := mgr.Add("iris", "")
	if err != nil {
		t.Fatalf("Add iris failed: %v", err)
	}
	if !res.SkippedClientOnly {
		t.Errorf("expected iris to be skipped on server side, got %+v", res)
	}
}

func TestManager_PinAndUnpin(t *testing.T) {
	server, client := setupMockModrinthServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "cmm.lock")

	lock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "sodium",
				Name:          "Sodium",
				VersionNumber: "0.5.0",
				Pinned:        false,
			},
		},
	}
	config.SaveLockfile(lockPath, lock)

	mgr := NewManager(client, "", lockPath)

	// Pin specific version
	err := mgr.Pin("sodium", "0.5.3")
	if err != nil {
		t.Fatalf("Pin failed: %v", err)
	}
	lock, _ = config.LoadLockfile(lockPath)
	if !lock.Mods[0].Pinned || lock.Mods[0].VersionNumber != "0.5.3" {
		t.Errorf("expected pinned true and version 0.5.3, got %+v", lock.Mods[0])
	}

	// Unpin
	alreadyUnpinned, err := mgr.Unpin("sodium")
	if err != nil || alreadyUnpinned {
		t.Errorf("Unpin failed: alreadyUnpinned=%v, err=%v", alreadyUnpinned, err)
	}
	lock, _ = config.LoadLockfile(lockPath)
	if lock.Mods[0].Pinned {
		t.Errorf("expected pinned false")
	}

	// Unpin already unpinned
	alreadyUnpinned, err = mgr.Unpin("sodium")
	if err != nil || !alreadyUnpinned {
		t.Errorf("expected alreadyUnpinned=true, got %v, err=%v", alreadyUnpinned, err)
	}
}

func TestManager_ListAndUpdates(t *testing.T) {
	server, client := setupMockModrinthServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")

	cfg := &config.Config{
		Profile: config.Profile{
			MinecraftVersion: "1.21.1",
			Loader:           "fabric",
		},
		Paths: config.Paths{
			ModsDir: filepath.Join(tmpDir, "mods"),
		},
	}
	config.SaveConfig(configPath, cfg)

	lock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "lithium",
				Name:          "Lithium",
				VersionNumber: "0.11.0",
				Version:       "0.11.0",
				FileName:      "lithium-fabric-0.11.0.jar",
				Pinned:        false,
			},
		},
	}
	config.SaveLockfile(lockPath, lock)

	mgr := NewManager(client, configPath, lockPath)

	// List should report update available for lithium (0.11.0 vs 0.11.2)
	list, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 || !list[0].UpdateAvailable || list[0].LatestVersion != "0.11.2" {
		t.Errorf("unexpected list output: %+v", list)
	}

	// Check updates
	candidates, skipped, err := mgr.CheckUpdates("", false)
	if err != nil || len(candidates) != 1 || len(skipped) != 0 {
		t.Fatalf("CheckUpdates failed: candidates=%d, skipped=%d, err=%v", len(candidates), len(skipped), err)
	}

	// Apply updates
	err = mgr.ApplyUpdates(candidates)
	if err != nil {
		t.Fatalf("ApplyUpdates failed: %v", err)
	}

	lock, _ = config.LoadLockfile(lockPath)
	if lock.Mods[0].VersionNumber != "0.11.2" {
		t.Errorf("expected updated version 0.11.2, got %s", lock.Mods[0].VersionNumber)
	}
}

func TestManager_Add_EdgeCases(t *testing.T) {
	server, client := setupMockModrinthServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")

	cfg := &config.Config{
		Profile: config.Profile{
			Side: "both",
		},
		Paths: config.Paths{
			ModsDir: filepath.Join(tmpDir, "mods"),
		},
	}
	config.SaveConfig(configPath, cfg)

	lock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "lithium",
				Name:          "Lithium",
				VersionNumber: "0.11.0",
			},
		},
	}
	config.SaveLockfile(lockPath, lock)

	mgr := NewManager(client, configPath, lockPath)

	// Test adding already installed mod
	res, err := mgr.Add("lithium", "")
	if err != nil {
		t.Fatalf("Add lithium failed: %v", err)
	}
	if !res.AlreadyInstalled {
		t.Errorf("expected already installed")
	}

	// Test adding non-existent mod
	_, err = mgr.Add("non_existent_slug_404", "")
	if err == nil {
		t.Errorf("expected error when adding non-existent mod")
	}
}

func TestManager_Remove_NotInstalled(t *testing.T) {
	server, client := setupMockModrinthServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	mgr := NewManager(client, filepath.Join(tmpDir, "cmm.toml"), filepath.Join(tmpDir, "cmm.lock"))

	_, err := mgr.Remove("not_installed_mod", false)
	if err == nil {
		t.Errorf("expected error removing non-installed mod")
	}
}

func TestManager_Updates_PinnedAndForce(t *testing.T) {
	server, client := setupMockModrinthServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")

	cfg := &config.Config{
		Profile: config.Profile{
			MinecraftVersion: "1.21.1",
			Loader:           "fabric",
		},
		Paths: config.Paths{
			ModsDir: filepath.Join(tmpDir, "mods"),
		},
	}
	config.SaveConfig(configPath, cfg)

	lock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "sodium",
				Name:          "Sodium",
				VersionNumber: "0.4.0",
				Version:       "0.4.0",
				Pinned:        true,
			},
		},
	}
	config.SaveLockfile(lockPath, lock)

	mgr := NewManager(client, configPath, lockPath)

	// Check updates without force -> should be skipped
	candidates, skipped, err := mgr.CheckUpdates("", false)
	if err != nil {
		t.Fatalf("CheckUpdates failed: %v", err)
	}
	if len(candidates) != 0 || len(skipped) != 1 {
		t.Errorf("expected 0 candidates, 1 skipped; got candidates=%d, skipped=%d", len(candidates), len(skipped))
	}

	// Check updates with force -> should find candidate
	candidates, skipped, err = mgr.CheckUpdates("", true)
	if err != nil {
		t.Fatalf("CheckUpdates with force failed: %v", err)
	}
	if len(candidates) != 1 || len(skipped) != 0 {
		t.Errorf("expected 1 candidate, 0 skipped with force; got candidates=%d, skipped=%d", len(candidates), len(skipped))
	}
}

func TestHashVerification(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jar")
	if err := os.WriteFile(testFile, testJarBytes, 0644); err != nil {
		t.Fatal(err)
	}

	sha512Hash, err := ComputeSHA512(testFile)
	if err != nil {
		t.Fatalf("ComputeSHA512 failed: %v", err)
	}
	if sha512Hash != testSHA512() {
		t.Errorf("expected %s, got %s", testSHA512(), sha512Hash)
	}

	sha1Hash, err := ComputeSHA1(testFile)
	if err != nil {
		t.Fatalf("ComputeSHA1 failed: %v", err)
	}
	if sha1Hash != testSHA1() {
		t.Errorf("expected %s, got %s", testSHA1(), sha1Hash)
	}

	valid, err := VerifyFileSHA512(testFile, testSHA512())
	if err != nil || !valid {
		t.Errorf("expected valid true, got %v, err=%v", valid, err)
	}

	valid, err = VerifyFileSHA512(testFile, "invalid-sha512")
	if valid || err == nil {
		t.Errorf("expected valid false on mismatch")
	}
}

