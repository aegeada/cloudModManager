package mod

import (
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

// TestChallenger_CleanVersion_Stress examines cleanVersion behavior on key inputs.
func TestChallenger_CleanVersion_Stress(t *testing.T) {
	tests := []struct {
		input        string
		expectedCore string
		expectedPre  string
	}{
		{
			input:        "1.21-0.5.8",
			expectedCore: "0.5.8",
			expectedPre:  "",
		},
		{
			input:        "mc1.20.1-1.0.0",
			expectedCore: "1.0.0",
			expectedPre:  "",
		},
		{
			input:        "1.21.2-rc1",
			expectedCore: "1.21.2",
			expectedPre:  "rc1",
		},
		{
			input:        "1.21.2-pre1",
			expectedCore: "1.21.2",
			expectedPre:  "pre1",
		},
	}

	for _, tt := range tests {
		core, pre := cleanVersion(tt.input)
		t.Logf("cleanVersion(%q) => core: %q, pre: %q (expected core: %q, pre: %q)",
			tt.input, core, pre, tt.expectedCore, tt.expectedPre)

		if core != tt.expectedCore || pre != tt.expectedPre {
			t.Errorf("cleanVersion(%q) = (%q, %q); want (%q, %q)",
				tt.input, core, pre, tt.expectedCore, tt.expectedPre)
		}
	}
}

// TestChallenger_IsNewerVersion_CrossChannel tests stability channel comparisons.
func TestChallenger_IsNewerVersion_CrossChannel(t *testing.T) {
	type testCase struct {
		candidate string
		current   string
		wantNewer bool
		desc      string
	}

	tests := []testCase{
		// Standard SemVer bumps within channel
		{"1.0.1", "1.0.0", true, "Patch upgrade within release"},
		{"1.1.0", "1.0.0", true, "Minor upgrade within release"},
		{"2.0.0", "1.0.0", true, "Major upgrade within release"},
		{"1.0.0", "1.0.1", false, "Patch downgrade must be rejected"},
		{"1.0.0", "1.1.0", false, "Minor downgrade must be rejected"},
		{"1.0.0", "2.0.0", false, "Major downgrade must be rejected"},
		{"1.0.0", "1.0.0", false, "Identical release must not be newer"},

		// Cross-channel: Release vs Beta
		{"1.2.0", "1.2.0-beta.2", true, "Official release is newer than its beta"},
		{"1.2.0-beta.2", "1.2.0", false, "Beta is older than its official release (downgrade prevention)"},
		{"1.1.0", "1.2.0-beta.2", false, "Older release is older than newer minor beta (downgrade prevention)"},
		{"1.2.0-beta.2", "1.1.0", true, "Newer minor beta is newer than older release"},

		// Cross-channel: Beta vs Alpha
		{"1.2.0-beta.1", "1.2.0-alpha.1", true, "Beta is newer than alpha of same version"},
		{"1.2.0-alpha.1", "1.2.0-beta.1", false, "Alpha is older than beta of same version (downgrade prevention)"},
		{"1.2.0-alpha.2", "1.2.0-alpha.1", true, "Alpha bump within alpha channel"},
		{"1.2.0-alpha.1", "1.2.0-alpha.2", false, "Alpha downgrade must be rejected"},

		// Cross-channel: Release vs Alpha
		{"1.2.0", "1.2.0-alpha.1", true, "Official release is newer than its alpha"},
		{"1.2.0-alpha.1", "1.2.0", false, "Alpha is older than its official release (downgrade prevention)"},
	}

	for _, tt := range tests {
		got := IsNewerVersion(tt.candidate, tt.current)
		if got != tt.wantNewer {
			t.Errorf("[%s] IsNewerVersion(%q, %q) = %v; want %v",
				tt.desc, tt.candidate, tt.current, got, tt.wantNewer)
		}
	}
}

// TestChallenger_ReleaseCandidate_DowngradeDefect empirically verifies whether release candidates
// cause downgrade inversion bugs due to mcPrefixRegex.
func TestChallenger_ReleaseCandidate_DowngradeDefect(t *testing.T) {
	// A user is on release candidate 1.21.2-rc1. Official 1.21.2 is released.
	// IsNewerVersion("1.21.2", "1.21.2-rc1") SHOULD be true.
	updateFromRcToRelease := IsNewerVersion("1.21.2", "1.21.2-rc1")
	t.Logf("IsNewerVersion(\"1.21.2\", \"1.21.2-rc1\") = %v (expected: true)", updateFromRcToRelease)

	// A user is on stable release 1.21.2. A release candidate 1.21.2-rc1 is evaluated.
	// IsNewerVersion("1.21.2-rc1", "1.21.2") SHOULD be false (downgrade!).
	downgradeToRc := IsNewerVersion("1.21.2-rc1", "1.21.2")
	t.Logf("IsNewerVersion(\"1.21.2-rc1\", \"1.21.2\") = %v (expected: false)", downgradeToRc)

	// A user is on pre-release 1.21.2-pre1. Official 1.21.2 is released.
	updateFromPreToRelease := IsNewerVersion("1.21.2", "1.21.2-pre1")
	t.Logf("IsNewerVersion(\"1.21.2\", \"1.21.2-pre1\") = %v (expected: true)", updateFromPreToRelease)

	downgradeToPre := IsNewerVersion("1.21.2-pre1", "1.21.2")
	t.Logf("IsNewerVersion(\"1.21.2-pre1\", \"1.21.2\") = %v (expected: false)", downgradeToPre)

	if !updateFromRcToRelease {
		t.Errorf("BUG CONFIRMED: Official release 1.21.2 is NOT considered newer than 1.21.2-rc1! Users on rc1 cannot update to release.")
	}
	if downgradeToRc {
		t.Errorf("BUG CONFIRMED: Release candidate 1.21.2-rc1 is considered newer than official release 1.21.2! Downgrade is proposed.")
	}
	if !updateFromPreToRelease {
		t.Errorf("BUG CONFIRMED: Official release 1.21.2 is NOT considered newer than 1.21.2-pre1!")
	}
	if downgradeToPre {
		t.Errorf("BUG CONFIRMED: Pre-release 1.21.2-pre1 is considered newer than official release 1.21.2!")
	}
}

// TestChallenger_AtomicStaging_DownloadFailure simulates download failure and verifies
// that existing files remain intact and no partial .tmp files are left behind.
func TestChallenger_AtomicStaging_DownloadFailure(t *testing.T) {
	// Setup mock server where download fails
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/project/lithium/version", func(w http.ResponseWriter, r *http.Request) {
		versions := []modrinth.Version{
			{
				ID:            "v-lithium-0.11.2",
				ProjectID:     "gvQqBUqZ",
				VersionNumber: "0.11.2",
				Files: []modrinth.VersionFile{
					{
						Filename: "lithium-fabric-0.11.2.jar",
						Primary:  true,
						Hashes: map[string]string{
							"sha512": "badhashbadhashbadhash",
						},
						URL: "http://" + r.Host + "/download/lithium-fabric-0.11.2.jar",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(versions)
	})
	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := modrinth.NewClientWithToken(server.URL+"/v2", "CMM-Test/1.0", "")
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	modsDir := filepath.Join(tmpDir, "mods")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 1. Create pre-existing installed mod file and lockfile
	existingJarPath := filepath.Join(modsDir, "lithium-fabric-0.11.1.jar")
	originalJarContent := []byte("PK\x03\x04original-lithium-jar-content")
	if err := os.WriteFile(existingJarPath, originalJarContent, 0644); err != nil {
		t.Fatal(err)
	}
	existingHash := sha512.Sum512(originalJarContent)

	cfg := &config.Config{
		Profile: config.Profile{
			Loader:           "fabric",
			MinecraftVersion: "1.20.1",
		},
		Paths: config.Paths{
			ModsDir: modsDir,
		},
	}
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	lock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "lithium",
				Name:          "Lithium",
				ProjectID:     "gvQqBUqZ",
				VersionID:     "v-lithium-0.11.1",
				VersionNumber: "0.11.1",
				Version:       "0.11.1",
				FileName:      "lithium-fabric-0.11.1.jar",
				SHA512:        hex.EncodeToString(existingHash[:]),
			},
		},
	}
	if err := config.SaveLockfile(lockPath, lock); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(client, configPath, lockPath)

	// 2. Attempt ApplyUpdates with candidate pointing to the failing download
	candidates := []UpdateCandidate{
		{
			Mod: lock.Mods[0],
			TargetVersion: modrinth.Version{
				ID:            "v-lithium-0.11.2",
				ProjectID:     "gvQqBUqZ",
				VersionNumber: "0.11.2",
				Files: []modrinth.VersionFile{
					{
						Filename: "lithium-fabric-0.11.2.jar",
						Primary:  true,
						Hashes: map[string]string{
							"sha512": "badhash",
						},
						URL: "http://" + server.Listener.Addr().String() + "/download/lithium-fabric-0.11.2.jar",
					},
				},
			},
		},
	}

	err = mgr.ApplyUpdates(candidates)
	if err == nil {
		t.Fatal("expected ApplyUpdates to fail due to HTTP 500 download failure, got nil")
	}
	t.Logf("ApplyUpdates failed as expected: %v", err)

	// 3. Empirically verify existing file is STILL intact and untouched
	if _, err := os.Stat(existingJarPath); os.IsNotExist(err) {
		t.Fatalf("ATOMIC FAILURE: Pre-existing mod file was DELETED during failed update!")
	}
	content, err := os.ReadFile(existingJarPath)
	if err != nil {
		t.Fatalf("failed to read existing jar: %v", err)
	}
	if string(content) != string(originalJarContent) {
		t.Fatalf("ATOMIC FAILURE: Pre-existing mod file content was corrupted!")
	}

	// 4. Verify no .tmp files remain in mods directory
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".tmp" {
			t.Errorf("ATOMIC FAILURE: Leftover temporary file found in mods dir: %s", entry.Name())
		}
	}

	// 5. Verify lockfile was NOT modified
	reloadedLock, err := config.LoadLockfile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloadedLock.Mods) != 1 || reloadedLock.Mods[0].VersionNumber != "0.11.1" {
		t.Errorf("ATOMIC FAILURE: Lockfile was prematurely modified to %v", reloadedLock.Mods)
	}

	t.Log("Atomic staging verified: existing file intact, lockfile intact, no leftover .tmp files.")
}
