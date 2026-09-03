package mod

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cmm/internal/config"
	"cmm/internal/modrinth"
)

func hash512(data []byte) string {
	sum := sha512.Sum512(data)
	return hex.EncodeToString(sum[:])
}

// setupAdversarialMockServer creates an HTTP mock server for complex dependency graphs
func setupAdversarialMockServer(t *testing.T) (*httptest.Server, *modrinth.Client) {
	mux := http.NewServeMux()

	dummyBytes := []byte("PK\x03\x04dummy-jar-bytes")
	dummyHash := hash512(dummyBytes)
	badBytes := []byte("PK\x03\x04corrupted-jar-bytes")

	makeVer := func(host, projID, verID, verNum, filename string, deps []modrinth.Dependency, sha string) modrinth.Version {
		if sha == "" {
			sha = dummyHash
		}
		return modrinth.Version{
			ID:            verID,
			ProjectID:     projID,
			VersionNumber: verNum,
			Dependencies:  deps,
			Files: []modrinth.VersionFile{
				{
					Filename: filename,
					Primary:  true,
					Hashes: map[string]string{
						"sha512": sha,
					},
					URL: "http://" + host + "/download/" + filename,
				},
			},
		}
	}

	mux.HandleFunc("/v2/project/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/v2/project/")

		if strings.HasSuffix(path, "/version") {
			slug := strings.TrimSuffix(path, "/version")
			switch strings.ToLower(slug) {
			case "circ-a", "id-circ-a":
				v := []modrinth.Version{
					makeVer(r.Host, "id-circ-a", "v-circ-a-1.0", "1.0", "circ-a-1.0.jar", []modrinth.Dependency{
						{ProjectID: strPtr("id-circ-b"), DependencyType: "required"},
					}, ""),
				}
				json.NewEncoder(w).Encode(v)
			case "circ-b", "id-circ-b":
				v := []modrinth.Version{
					makeVer(r.Host, "id-circ-b", "v-circ-b-1.0", "1.0", "circ-b-1.0.jar", []modrinth.Dependency{
						{ProjectID: strPtr("id-circ-a"), DependencyType: "required"},
					}, ""),
				}
				json.NewEncoder(w).Encode(v)
			case "circ-3a", "id-circ-3a":
				v := []modrinth.Version{
					makeVer(r.Host, "id-circ-3a", "v-circ-3a-1.0", "1.0", "circ-3a-1.0.jar", []modrinth.Dependency{
						{ProjectID: strPtr("id-circ-3b"), DependencyType: "required"},
					}, ""),
				}
				json.NewEncoder(w).Encode(v)
			case "circ-3b", "id-circ-3b":
				v := []modrinth.Version{
					makeVer(r.Host, "id-circ-3b", "v-circ-3b-1.0", "1.0", "circ-3b-1.0.jar", []modrinth.Dependency{
						{ProjectID: strPtr("id-circ-3c"), DependencyType: "required"},
					}, ""),
				}
				json.NewEncoder(w).Encode(v)
			case "circ-3c", "id-circ-3c":
				v := []modrinth.Version{
					makeVer(r.Host, "id-circ-3c", "v-circ-3c-1.0", "1.0", "circ-3c-1.0.jar", []modrinth.Dependency{
						{ProjectID: strPtr("id-circ-3a"), DependencyType: "required"},
					}, ""),
				}
				json.NewEncoder(w).Encode(v)
			case "incomp-target", "id-incomp-target":
				v := []modrinth.Version{
					makeVer(r.Host, "id-incomp-target", "v-incomp-target-1.0", "1.0", "incomp-target-1.0.jar", []modrinth.Dependency{
						{ProjectID: strPtr("id-incomp-bad"), DependencyType: "incompatible"},
					}, ""),
				}
				json.NewEncoder(w).Encode(v)
			case "incomp-bad", "id-incomp-bad":
				v := []modrinth.Version{
					makeVer(r.Host, "id-incomp-bad", "v-incomp-bad-1.0", "1.0", "incomp-bad-1.0.jar", nil, ""),
				}
				json.NewEncoder(w).Encode(v)
			case "opt-target", "id-opt-target":
				v := []modrinth.Version{
					makeVer(r.Host, "id-opt-target", "v-opt-target-1.0", "1.0", "opt-target-1.0.jar", []modrinth.Dependency{
						{ProjectID: strPtr("id-opt-req"), DependencyType: "required"},
						{ProjectID: strPtr("id-opt-optional"), DependencyType: "optional"},
					}, ""),
				}
				json.NewEncoder(w).Encode(v)
			case "opt-req", "id-opt-req":
				v := []modrinth.Version{
					makeVer(r.Host, "id-opt-req", "v-opt-req-1.0", "1.0", "opt-req-1.0.jar", nil, ""),
				}
				json.NewEncoder(w).Encode(v)
			case "opt-optional", "id-opt-optional":
				v := []modrinth.Version{
					makeVer(r.Host, "id-opt-optional", "v-opt-optional-1.0", "1.0", "opt-optional-1.0.jar", nil, ""),
				}
				json.NewEncoder(w).Encode(v)
			case "corrupt-hash-mod":
				v := []modrinth.Version{
					makeVer(r.Host, "corrupt-hash-mod", "v-corrupt-1.0", "1.0", "corrupt.jar", nil, "invalid-sha-512-hash-that-wont-match"),
				}
				json.NewEncoder(w).Encode(v)
			case "shared-a", "id-shared-a":
				v := []modrinth.Version{
					makeVer(r.Host, "id-shared-a", "v-shared-a-1.0", "1.0", "shared-a-1.0.jar", []modrinth.Dependency{
						{ProjectID: strPtr("id-shared-dep"), DependencyType: "required"},
					}, ""),
				}
				json.NewEncoder(w).Encode(v)
			case "shared-b", "id-shared-b":
				v := []modrinth.Version{
					makeVer(r.Host, "id-shared-b", "v-shared-b-1.0", "1.0", "shared-b-1.0.jar", []modrinth.Dependency{
						{ProjectID: strPtr("id-shared-dep"), DependencyType: "required"},
					}, ""),
				}
				json.NewEncoder(w).Encode(v)
			case "shared-dep", "id-shared-dep":
				v := []modrinth.Version{
					makeVer(r.Host, "id-shared-dep", "v-shared-dep-1.0", "1.0", "shared-dep-1.0.jar", nil, ""),
				}
				json.NewEncoder(w).Encode(v)
			default:
				http.NotFound(w, r)
			}
			return
		}

		// Project metadata
		slug := path
		switch strings.ToLower(slug) {
		case "circ-a", "id-circ-a":
			json.NewEncoder(w).Encode(modrinth.Project{ID: "id-circ-a", Slug: "circ-a", Title: "Circular A", ServerSide: "required", ClientSide: "required"})
		case "circ-b", "id-circ-b":
			json.NewEncoder(w).Encode(modrinth.Project{ID: "id-circ-b", Slug: "circ-b", Title: "Circular B", ServerSide: "required", ClientSide: "required"})
		case "circ-3a", "id-circ-3a":
			json.NewEncoder(w).Encode(modrinth.Project{ID: "id-circ-3a", Slug: "circ-3a", Title: "Circular 3A", ServerSide: "required", ClientSide: "required"})
		case "circ-3b", "id-circ-3b":
			json.NewEncoder(w).Encode(modrinth.Project{ID: "id-circ-3b", Slug: "circ-3b", Title: "Circular 3B", ServerSide: "required", ClientSide: "required"})
		case "circ-3c", "id-circ-3c":
			json.NewEncoder(w).Encode(modrinth.Project{ID: "id-circ-3c", Slug: "circ-3c", Title: "Circular 3C", ServerSide: "required", ClientSide: "required"})
		case "incomp-target", "id-incomp-target":
			json.NewEncoder(w).Encode(modrinth.Project{ID: "id-incomp-target", Slug: "incomp-target", Title: "Incomp Target", ServerSide: "required", ClientSide: "required"})
		case "incomp-bad", "id-incomp-bad":
			json.NewEncoder(w).Encode(modrinth.Project{ID: "id-incomp-bad", Slug: "incomp-bad", Title: "Incomp Bad", ServerSide: "required", ClientSide: "required"})
		case "opt-target", "id-opt-target":
			json.NewEncoder(w).Encode(modrinth.Project{ID: "id-opt-target", Slug: "opt-target", Title: "Opt Target", ServerSide: "required", ClientSide: "required"})
		case "opt-req", "id-opt-req":
			json.NewEncoder(w).Encode(modrinth.Project{ID: "id-opt-req", Slug: "opt-req", Title: "Opt Req", ServerSide: "required", ClientSide: "required"})
		case "opt-optional", "id-opt-optional":
			json.NewEncoder(w).Encode(modrinth.Project{ID: "id-opt-optional", Slug: "opt-optional", Title: "Opt Optional", ServerSide: "required", ClientSide: "required"})
		case "corrupt-hash-mod":
			json.NewEncoder(w).Encode(modrinth.Project{ID: "corrupt-hash-mod", Slug: "corrupt-hash-mod", Title: "Corrupt Hash Mod", ServerSide: "required", ClientSide: "required"})
		case "shared-a", "id-shared-a":
			json.NewEncoder(w).Encode(modrinth.Project{ID: "id-shared-a", Slug: "shared-a", Title: "Shared A", ServerSide: "required", ClientSide: "required"})
		case "shared-b", "id-shared-b":
			json.NewEncoder(w).Encode(modrinth.Project{ID: "id-shared-b", Slug: "shared-b", Title: "Shared B", ServerSide: "required", ClientSide: "required"})
		case "shared-dep", "id-shared-dep":
			json.NewEncoder(w).Encode(modrinth.Project{ID: "id-shared-dep", Slug: "shared-dep", Title: "Shared Dep", ServerSide: "required", ClientSide: "required"})
		default:
			http.NotFound(w, r)
		}
	})

	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		filename := strings.TrimPrefix(r.URL.Path, "/download/")
		if filename == "corrupt.jar" {
			w.Header().Set("Content-Type", "application/java-archive")
			w.Write(badBytes)
			return
		}
		w.Header().Set("Content-Type", "application/java-archive")
		w.Write(dummyBytes)
	})

	server := httptest.NewServer(mux)
	client, err := modrinth.NewClientWithToken(server.URL+"/v2", "CMM-Adversarial/1.0", "")
	if err != nil {
		t.Fatal(err)
	}

	return server, client
}

// Test 1: 2-way and 3-way circular dependencies
func TestAdversarial_CircularDependencies(t *testing.T) {
	server, client := setupAdversarialMockServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")

	cfg := &config.Config{
		Profile: config.Profile{
			MinecraftVersion: "1.21.1",
			Loader:           "fabric",
			Side:             "both",
		},
		Paths: config.Paths{
			ModsDir: modsDir,
		},
	}
	config.SaveConfig(configPath, cfg)

	mgr := NewManager(client, configPath, lockPath)

	// 2-way cycle: circ-a -> circ-b -> circ-a
	res, err := mgr.Add("circ-a", "")
	if err != nil {
		t.Fatalf("2-way circular dependency failed: %v", err)
	}
	if res.InstalledMod.Slug != "circ-a" {
		t.Errorf("expected circ-a installed, got %+v", res.InstalledMod)
	}
	if len(res.InstalledDeps) != 1 || res.InstalledDeps[0].Slug != "circ-b" {
		t.Errorf("expected circ-b installed as dep, got %+v", res.InstalledDeps)
	}

	// Verify lockfile has both
	lock, err := config.LoadLockfile(lockPath)
	if err != nil || len(lock.Mods) != 2 {
		t.Fatalf("expected 2 mods in lockfile, got %d", len(lock.Mods))
	}

	// Verify files on disk
	if _, err := os.Stat(filepath.Join(modsDir, "circ-a-1.0.jar")); err != nil {
		t.Errorf("circ-a jar missing on disk")
	}
	if _, err := os.Stat(filepath.Join(modsDir, "circ-b-1.0.jar")); err != nil {
		t.Errorf("circ-b jar missing on disk")
	}

	// 3-way cycle: circ-3a -> circ-3b -> circ-3c -> circ-3a
	res3, err := mgr.Add("circ-3a", "")
	if err != nil {
		t.Fatalf("3-way circular dependency failed: %v", err)
	}
	if res3.InstalledMod.Slug != "circ-3a" {
		t.Errorf("expected circ-3a installed, got %+v", res3.InstalledMod)
	}
	if len(res3.InstalledDeps) != 2 {
		t.Errorf("expected 2 deps (circ-3b and circ-3c), got %d", len(res3.InstalledDeps))
	}

	lock, _ = config.LoadLockfile(lockPath)
	if len(lock.Mods) != 5 { // 2 from previous + 3 new
		t.Fatalf("expected 5 mods in lockfile, got %d", len(lock.Mods))
	}
}

// Test 2: Incompatible dependency detection
func TestAdversarial_IncompatibleDependency(t *testing.T) {
	server, client := setupAdversarialMockServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")

	cfg := &config.Config{
		Profile: config.Profile{
			MinecraftVersion: "1.21.1",
			Loader:           "fabric",
			Side:             "both",
		},
		Paths: config.Paths{
			ModsDir: modsDir,
		},
	}
	config.SaveConfig(configPath, cfg)

	mgr := NewManager(client, configPath, lockPath)

	// Step 1: Install incomp-bad first
	_, err := mgr.Add("incomp-bad", "")
	if err != nil {
		t.Fatalf("failed to install incomp-bad: %v", err)
	}

	// Step 2: Try installing incomp-target which declares incomp-bad as incompatible
	_, err = mgr.Add("incomp-target", "")
	if err == nil {
		t.Fatalf("expected error when installing incompatible mod, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "incompatible") {
		t.Errorf("expected error message to contain 'incompatible', got: %v", err)
	}

	// Verify incomp-target was not added to lockfile
	lock, _ := config.LoadLockfile(lockPath)
	if len(lock.Mods) != 1 || lock.Mods[0].Slug != "incomp-bad" {
		t.Errorf("expected only incomp-bad in lockfile, got %+v", lock.Mods)
	}
}

// Test 3: Optional dependencies not installed automatically, but reported
func TestAdversarial_OptionalDependency(t *testing.T) {
	server, client := setupAdversarialMockServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")

	cfg := &config.Config{
		Profile: config.Profile{
			MinecraftVersion: "1.21.1",
			Loader:           "fabric",
			Side:             "both",
		},
		Paths: config.Paths{
			ModsDir: modsDir,
		},
	}
	config.SaveConfig(configPath, cfg)

	mgr := NewManager(client, configPath, lockPath)

	res, err := mgr.Add("opt-target", "")
	if err != nil {
		t.Fatalf("Add opt-target failed: %v", err)
	}

	if len(res.InstalledDeps) != 1 || res.InstalledDeps[0].Slug != "opt-req" {
		t.Errorf("expected only opt-req installed as dep, got %+v", res.InstalledDeps)
	}

	if len(res.OptionalDeps) != 1 || (!strings.Contains(res.OptionalDeps[0], "opt-optional") && res.OptionalDeps[0] != "id-opt-optional") {
		t.Errorf("expected opt-optional reported in OptionalDeps, got %+v", res.OptionalDeps)
	}

	// Lockfile should have opt-target and opt-req, NOT opt-optional
	lock, _ := config.LoadLockfile(lockPath)
	if len(lock.Mods) != 2 {
		t.Errorf("expected 2 mods in lockfile, got %d", len(lock.Mods))
	}
	if lock.GetMod("opt-optional") != nil {
		t.Errorf("opt-optional should NOT be installed in lockfile")
	}
}

// Test 4: Download checksum verification failure
func TestAdversarial_DownloadChecksumMismatch(t *testing.T) {
	server, client := setupAdversarialMockServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")

	cfg := &config.Config{
		Profile: config.Profile{
			MinecraftVersion: "1.21.1",
			Loader:           "fabric",
			Side:             "both",
		},
		Paths: config.Paths{
			ModsDir: modsDir,
		},
	}
	config.SaveConfig(configPath, cfg)

	mgr := NewManager(client, configPath, lockPath)

	_, err := mgr.Add("corrupt-hash-mod", "")
	if err == nil {
		t.Fatalf("expected error on checksum mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") && !strings.Contains(err.Error(), "SHA-512") {
		t.Errorf("expected checksum mismatch error, got: %v", err)
	}

	// Ensure corrupt file was deleted from disk
	if _, err := os.Stat(filepath.Join(modsDir, "corrupt.jar")); !os.IsNotExist(err) {
		t.Errorf("corrupt jar should have been deleted from disk on hash mismatch")
	}

	// Ensure lockfile not created or empty
	lock, err := config.LoadLockfile(lockPath)
	if err == nil && lock != nil && len(lock.Mods) != 0 {
		t.Errorf("lockfile should have 0 mods, got %d", len(lock.Mods))
	}
}

// Test 5: Shared dependency orphan retention across multiple removals
func TestAdversarial_SharedDependencyRetentionAndOrphan(t *testing.T) {
	server, client := setupAdversarialMockServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")

	cfg := &config.Config{
		Profile: config.Profile{
			MinecraftVersion: "1.21.1",
			Loader:           "fabric",
			Side:             "both",
		},
		Paths: config.Paths{
			ModsDir: modsDir,
		},
	}
	config.SaveConfig(configPath, cfg)

	mgr := NewManager(client, configPath, lockPath)

	// Install shared-a (pulls shared-dep)
	_, err := mgr.Add("shared-a", "")
	if err != nil {
		t.Fatalf("failed to add shared-a: %v", err)
	}

	// Install shared-b (shared-dep already installed, skips)
	_, err = mgr.Add("shared-b", "")
	if err != nil {
		t.Fatalf("failed to add shared-b: %v", err)
	}

	lock, _ := config.LoadLockfile(lockPath)
	if len(lock.Mods) != 3 {
		t.Fatalf("expected 3 mods (shared-a, shared-b, shared-dep), got %d", len(lock.Mods))
	}

	// Remove shared-a: shared-dep is still needed by shared-b -> OrphanedDeps must be empty
	resRemA, err := mgr.Remove("shared-a", false)
	if err != nil {
		t.Fatalf("failed to remove shared-a: %v", err)
	}
	if len(resRemA.OrphanedDeps) != 0 {
		t.Errorf("expected 0 orphaned deps when shared-b still depends on it, got %+v", resRemA.OrphanedDeps)
	}

	// Remove shared-b: now shared-dep is NOT needed by anything -> OrphanedDeps must contain shared-dep
	resRemB, err := mgr.Remove("shared-b", false)
	if err != nil {
		t.Fatalf("failed to remove shared-b: %v", err)
	}
	if len(resRemB.OrphanedDeps) != 1 || resRemB.OrphanedDeps[0] != "shared-dep" {
		t.Errorf("expected shared-dep in OrphanedDeps, got %+v", resRemB.OrphanedDeps)
	}

	// Clean orphan
	err = mgr.RemoveOrphan("shared-dep")
	if err != nil {
		t.Fatalf("RemoveOrphan failed: %v", err)
	}

	lock, _ = config.LoadLockfile(lockPath)
	if len(lock.Mods) != 0 {
		t.Errorf("expected empty lockfile, got %d", len(lock.Mods))
	}
}

// Test 6: Dry Run Guarantees
func TestAdversarial_DryRunGuarantees(t *testing.T) {
	server, client := setupAdversarialMockServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")

	cfg := &config.Config{
		Profile: config.Profile{
			MinecraftVersion: "1.21.1",
			Loader:           "fabric",
			Side:             "both",
		},
		Paths: config.Paths{
			ModsDir: modsDir,
		},
	}
	config.SaveConfig(configPath, cfg)

	mgr := NewManager(client, configPath, lockPath)

	_, err := mgr.Add("circ-a", "")
	if err != nil {
		t.Fatalf("failed to add circ-a: %v", err)
	}

	// Dry run removal
	res, err := mgr.Remove("circ-a", true)
	if err != nil {
		t.Fatalf("Dry run remove failed: %v", err)
	}
	if !res.DryRun {
		t.Errorf("expected DryRun = true")
	}

	// Check that files are NOT removed
	if _, err := os.Stat(filepath.Join(modsDir, "circ-a-1.0.jar")); os.IsNotExist(err) {
		t.Errorf("circ-a-1.0.jar was deleted during dry run!")
	}
	if _, err := os.Stat(filepath.Join(modsDir, "circ-b-1.0.jar")); os.IsNotExist(err) {
		t.Errorf("circ-b-1.0.jar was deleted during dry run!")
	}

	// Check lockfile is NOT modified
	lock, _ := config.LoadLockfile(lockPath)
	if len(lock.Mods) != 2 {
		t.Errorf("lockfile was modified during dry run! Expected 2 mods, got %d", len(lock.Mods))
	}
}
