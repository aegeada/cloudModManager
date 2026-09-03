package sync

import (
	"os"
	"path/filepath"
	"testing"

	"cmm/internal/config"
)

func TestDeltaEngine_ApplyTargetFiles_PruneAndAdd(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods")
	os.MkdirAll(modsDir, 0755)

	// Create an extraneous JAR
	os.WriteFile(filepath.Join(modsDir, "old-mod.jar"), []byte("old content"), 0644)
	// Create an existing target JAR
	os.WriteFile(filepath.Join(modsDir, "existing.jar"), []byte("existing content"), 0644)

	lockPath := filepath.Join(tmpDir, "cmm.lock")
	localLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{Slug: "old-mod", FileName: "old-mod.jar", Pinned: false},
			{Slug: "existing", FileName: "existing.jar", Pinned: true},
		},
	}

	engine := NewDeltaEngine(nil, nil, localLock, modsDir, lockPath)
	targets := []TargetFile{
		{
			FileName: "existing.jar",
			Slug:     "existing",
			Name:     "Existing Mod",
		},
	}

	res, err := engine.ApplyTargetFiles(targets)
	if err != nil {
		t.Fatalf("ApplyTargetFiles failed: %v", err)
	}

	if len(res.RemovedMods) != 1 || res.RemovedMods[0] != "old-mod.jar" {
		t.Errorf("expected old-mod.jar removed, got %v", res.RemovedMods)
	}

	// Verify old-mod.jar was deleted from disk
	if _, err := os.Stat(filepath.Join(modsDir, "old-mod.jar")); !os.IsNotExist(err) {
		t.Errorf("expected old-mod.jar to be removed from disk")
	}

	// Verify existing.jar pin was preserved
	savedLock, err := config.LoadLockfile(lockPath)
	if err != nil {
		t.Fatalf("failed to load saved lockfile: %v", err)
	}
	if !savedLock.IsPinned("existing") {
		t.Errorf("expected existing mod pin to be preserved")
	}
}

func TestParseMrpack_NonExistentFile(t *testing.T) {
	_, err := ParseMrpack("/non/existent/path.mrpack")
	if err == nil {
		t.Errorf("expected error for non-existent file")
	}
}
