package sync

import (
	"crypto/sha512"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cmm/internal/config"
	"cmm/internal/modrinth"
)

func hashStr(b []byte) string {
	sum := sha512.Sum512(b)
	return hex.EncodeToString(sum[:])
}

func TestDeltaEngine_TwoPhaseStaging_RollbackOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 1. Setup existing mods on disk
	modAOrigContent := []byte("mod-A-original-content-v1")
	modBOrigContent := []byte("mod-B-original-content-to-be-retained-on-failure")
	if err := os.WriteFile(filepath.Join(modsDir, "modA.jar"), modAOrigContent, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modsDir, "modB.jar"), modBOrigContent, 0644); err != nil {
		t.Fatal(err)
	}

	initialLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{Slug: "modA", Name: "Mod A", Version: "1.0.0", FileName: "modA.jar", SHA512: hashStr(modAOrigContent)},
			{Slug: "modB", Name: "Mod B", Version: "1.0.0", FileName: "modB.jar", SHA512: hashStr(modBOrigContent)},
		},
	}
	if err := config.SaveLockfile(lockPath, initialLock); err != nil {
		t.Fatal(err)
	}

	// 2. Setup mock server where modC fails with HTTP 500
	modANewContent := []byte("mod-A-updated-content-v2")
	mux := http.NewServeMux()
	mux.HandleFunc("/files/modA-v2.jar", func(w http.ResponseWriter, r *http.Request) {
		w.Write(modANewContent)
	})
	mux.HandleFunc("/files/modC.jar", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, err := modrinth.NewClientWithToken(srv.URL+"/v2", "CMM-Test/1.0", "")
	if err != nil {
		t.Fatal(err)
	}

	engine := NewDeltaEngine(client, nil, initialLock, modsDir, lockPath)

	targets := []TargetFile{
		{
			FileName:    "modA.jar",
			SHA512:      hashStr(modANewContent),
			DownloadURL: srv.URL + "/files/modA-v2.jar",
			Slug:        "modA",
			Name:        "Mod A",
			Version:     "2.0.0",
		},
		{
			FileName:    "modC.jar",
			SHA512:      "nonexistent-hash",
			DownloadURL: srv.URL + "/files/modC.jar",
			Slug:        "modC",
			Name:        "Mod C",
			Version:     "1.0.0",
		},
	}

	// 3. Apply target files - should fail because modC download fails
	_, err = engine.ApplyTargetFiles(targets)
	if err == nil {
		t.Fatalf("expected error from failed download, got nil")
	}

	// 4. CRITICAL VERIFICATIONS:
	// - modA.jar must still have its ORIGINAL content (not updated/corrupted)
	curModA, err := os.ReadFile(filepath.Join(modsDir, "modA.jar"))
	if err != nil {
		t.Fatalf("modA.jar missing after failed sync: %v", err)
	}
	if string(curModA) != string(modAOrigContent) {
		t.Errorf("CRITICAL: modA.jar was overwritten before all downloads succeeded! Expected original content")
	}

	// - modB.jar must NOT have been pruned!
	curModB, err := os.ReadFile(filepath.Join(modsDir, "modB.jar"))
	if err != nil {
		t.Fatalf("CRITICAL: modB.jar was pruned despite download failure: %v", err)
	}
	if string(curModB) != string(modBOrigContent) {
		t.Errorf("modB.jar content corrupted")
	}

	// - No temporary files left behind
	entries, _ := os.ReadDir(modsDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") || strings.Contains(e.Name(), ".tmp.") {
			t.Errorf("orphan temporary staging file left on disk: %s", e.Name())
		}
	}

	// - Lockfile must NOT have been updated
	savedLock, err := config.LoadLockfile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(savedLock.Mods) != 2 || savedLock.GetMod("modA").Version != "1.0.0" {
		t.Errorf("lockfile was modified despite download failure: %+v", savedLock.Mods)
	}
}

func TestDeltaEngine_PinnedModPreservation_NoOverwriteOrPrune(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Setup:
	// 1. pinnedMod: pinned mod on disk v1.0.0. Modpack wants to update it to v2.0.0.
	// 2. customPinned: pinned mod on disk v3.0.0. Modpack doesn't mention it at all.
	// 3. unpinnedMod: unpinned mod on disk v1.0.0. Modpack doesn't mention it (should be pruned).
	// 4. newMod: new mod in modpack (should be downloaded).

	pinnedModContent := []byte("pinned-mod-v1.0.0")
	customPinnedContent := []byte("custom-pinned-mod-v3.0.0")
	unpinnedModContent := []byte("unpinned-mod-to-be-deleted")
	newModContent := []byte("new-mod-v1.0.0")

	os.WriteFile(filepath.Join(modsDir, "pinned-mod.jar"), pinnedModContent, 0644)
	os.WriteFile(filepath.Join(modsDir, "custom-pinned.jar"), customPinnedContent, 0644)
	os.WriteFile(filepath.Join(modsDir, "unpinned-mod.jar"), unpinnedModContent, 0644)

	initialLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "pinned-mod",
				Name:          "Pinned Mod",
				Version:       "1.0.0",
				VersionNumber: "1.0.0",
				FileName:      "pinned-mod.jar",
				SHA512:        hashStr(pinnedModContent),
				Pinned:        true,
			},
			{
				Slug:          "custom-pinned",
				Name:          "Custom Pinned Mod",
				Version:       "3.0.0",
				VersionNumber: "3.0.0",
				FileName:      "custom-pinned.jar",
				SHA512:        hashStr(customPinnedContent),
				Pinned:        true,
			},
			{
				Slug:          "unpinned-mod",
				Name:          "Unpinned Mod",
				Version:       "1.0.0",
				VersionNumber: "1.0.0",
				FileName:      "unpinned-mod.jar",
				SHA512:        hashStr(unpinnedModContent),
				Pinned:        false,
			},
		},
	}
	config.SaveLockfile(lockPath, initialLock)

	mux := http.NewServeMux()
	mux.HandleFunc("/files/pinned-mod-v2.jar", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("CRITICAL: Server was asked to download newer version for pinned mod!")
		w.Write([]byte("pinned-mod-v2.0.0"))
	})
	mux.HandleFunc("/files/new-mod.jar", func(w http.ResponseWriter, r *http.Request) {
		w.Write(newModContent)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, _ := modrinth.NewClientWithToken(srv.URL+"/v2", "CMM-Test/1.0", "")
	engine := NewDeltaEngine(client, nil, initialLock, modsDir, lockPath)

	targets := []TargetFile{
		{
			FileName:    "pinned-mod.jar",
			SHA512:      hashStr([]byte("pinned-mod-v2.0.0")),
			DownloadURL: srv.URL + "/files/pinned-mod-v2.jar",
			Slug:        "pinned-mod",
			Name:        "Pinned Mod",
			Version:     "2.0.0",
		},
		{
			FileName:    "new-mod.jar",
			SHA512:      hashStr(newModContent),
			DownloadURL: srv.URL + "/files/new-mod.jar",
			Slug:        "new-mod",
			Name:        "New Mod",
			Version:     "1.0.0",
		},
	}

	res, err := engine.ApplyTargetFiles(targets)
	if err != nil {
		t.Fatalf("delta sync failed: %v", err)
	}

	// 1. Verify pinned-mod.jar content is still v1.0.0 (NOT overwritten)
	curPinned, err := os.ReadFile(filepath.Join(modsDir, "pinned-mod.jar"))
	if err != nil {
		t.Fatalf("pinned-mod.jar missing: %v", err)
	}
	if string(curPinned) != string(pinnedModContent) {
		t.Errorf("CRITICAL: pinned-mod.jar was overwritten by newer version!")
	}

	// 2. Verify custom-pinned.jar was NOT pruned
	if _, err := os.Stat(filepath.Join(modsDir, "custom-pinned.jar")); err != nil {
		t.Errorf("CRITICAL: custom-pinned.jar was pruned despite being pinned!")
	}

	// 3. Verify unpinned-mod.jar WAS pruned
	if _, err := os.Stat(filepath.Join(modsDir, "unpinned-mod.jar")); !os.IsNotExist(err) {
		t.Errorf("expected unpinned-mod.jar to be pruned, but still exists")
	}

	// 4. Verify new-mod.jar was downloaded
	curNew, err := os.ReadFile(filepath.Join(modsDir, "new-mod.jar"))
	if err != nil {
		t.Fatalf("new-mod.jar was not downloaded: %v", err)
	}
	if string(curNew) != string(newModContent) {
		t.Errorf("new-mod.jar content mismatch")
	}

	// 5. Verify lockfile preserves pinned status and versions
	savedLock, err := config.LoadLockfile(lockPath)
	if err != nil {
		t.Fatal(err)
	}

	pMod := savedLock.GetMod("pinned-mod")
	if pMod == nil || !pMod.Pinned || pMod.Version != "1.0.0" {
		t.Errorf("expected pinned-mod in lockfile with Pinned=true and Version=1.0.0, got: %+v", pMod)
	}

	cpMod := savedLock.GetMod("custom-pinned")
	if cpMod == nil || !cpMod.Pinned || cpMod.Version != "3.0.0" {
		t.Errorf("expected custom-pinned in lockfile with Pinned=true and Version=3.0.0, got: %+v", cpMod)
	}

	newM := savedLock.GetMod("new-mod")
	if newM == nil || newM.Pinned {
		t.Errorf("expected new-mod in lockfile with Pinned=false, got: %+v", newM)
	}

	_ = res
}
