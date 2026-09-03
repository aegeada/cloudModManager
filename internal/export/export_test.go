package export

import (
	"archive/zip"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"

	"cmm/internal/config"
)

func TestExportMrpack_ValidIndexJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	outPath := filepath.Join(tmpDir, "test.mrpack")

	cfg := &config.Config{
		Profile: config.Profile{
			Name:             "TestPack",
			MinecraftVersion: "1.21.1",
			Loader:           "fabric",
			LoaderVersion:    "0.19.3",
		},
	}
	_ = config.SaveConfig(configPath, cfg)

	lock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:        "sodium",
				Name:        "Sodium",
				FileName:    "sodium.jar",
				SHA512:      "abcd512",
				DownloadURL: "https://example.com/sodium.jar",
				Side:        "client",
			},
		},
	}
	_ = config.SaveLockfile(lockPath, lock)

	err := ExportMrpack(MrpackExportOptions{
		OutputPath: outPath,
		Name:       "TestPack",
		ConfigPath: configPath,
		LockPath:   lockPath,
	})
	if err != nil {
		t.Fatalf("ExportMrpack failed: %v", err)
	}

	// Verify ZIP structure
	zr, err := zip.OpenReader(outPath)
	if err != nil {
		t.Fatalf("failed to open exported zip: %v", err)
	}
	defer zr.Close()

	var foundIndex bool
	for _, f := range zr.File {
		if f.Name == "modrinth.index.json" {
			foundIndex = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("failed to open modrinth.index.json: %v", err)
			}
			defer rc.Close()
			data, _ := io.ReadAll(rc)
			var idx MrpackIndex
			if err := json.Unmarshal(data, &idx); err != nil {
				t.Fatalf("failed to unmarshal index JSON: %v", err)
			}
			if idx.Name != "TestPack" {
				t.Errorf("expected name 'TestPack', got %s", idx.Name)
			}
			if len(idx.Files) != 1 {
				t.Fatalf("expected 1 file, got %d", len(idx.Files))
			}
			if idx.Files[0].Env.Client != "required" || idx.Files[0].Env.Server != "unsupported" {
				t.Errorf("unexpected env mapping: %+v", idx.Files[0].Env)
			}
		}
	}
	if !foundIndex {
		t.Errorf("modrinth.index.json not found in exported zip")
	}
}

func TestExportGitHub_SanitizesSensitiveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")
	outDir := filepath.Join(tmpDir, "gh_out")

	cfg := &config.Config{
		Profile: config.Profile{
			Name: "TestProject",
		},
		SyncToken: "super-secret-sync-token",
		Modrinth: config.Modrinth{
			Token: "secret-modrinth-token",
		},
		SyncSources: []config.SyncSource{
			{Type: "github", Token: "ghp_secret"},
		},
	}
	_ = config.SaveConfig(configPath, cfg)

	lock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{Slug: "sodium", Version: "0.5.8"},
		},
	}
	_ = config.SaveLockfile(lockPath, lock)

	err := ExportGitHub(GitHubExportOptions{
		OutputDir:  outDir,
		ConfigPath: configPath,
		LockPath:   lockPath,
	})
	if err != nil {
		t.Fatalf("ExportGitHub failed: %v", err)
	}

	exportedCfg, err := config.LoadConfig(filepath.Join(outDir, "cmm.toml"))
	if err != nil {
		t.Fatalf("failed to load exported config: %v", err)
	}

	if exportedCfg.SyncToken != "" {
		t.Errorf("expected SyncToken to be stripped, got %q", exportedCfg.SyncToken)
	}
	if exportedCfg.Modrinth.Token != "" {
		t.Errorf("expected Modrinth token to be stripped, got %q", exportedCfg.Modrinth.Token)
	}
	if len(exportedCfg.SyncSources) > 0 && exportedCfg.SyncSources[0].Token != "" {
		t.Errorf("expected SyncSource token to be stripped, got %q", exportedCfg.SyncSources[0].Token)
	}

	exportedLock, err := config.LoadLockfile(filepath.Join(outDir, "cmm.lock"))
	if err != nil {
		t.Fatalf("failed to load exported lockfile: %v", err)
	}
	if len(exportedLock.Mods) != 1 || exportedLock.Mods[0].Slug != "sodium" {
		t.Errorf("unexpected exported lockfile mods: %+v", exportedLock.Mods)
	}
}
