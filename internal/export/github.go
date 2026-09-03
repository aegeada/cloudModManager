package export

import (
	"fmt"
	"os"
	"path/filepath"

	"cmm/internal/config"
)

type GitHubExportOptions struct {
	OutputDir  string
	ConfigPath string
	LockPath   string
}

// ExportGitHub generates a clean repository structure containing sanitized cmm.toml and cmm.lock.
func ExportGitHub(opts GitHubExportOptions) error {
	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = "export"
	}
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = "cmm.toml"
	}
	lockPath := opts.LockPath
	if lockPath == "" {
		lockPath = "cmm.lock"
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Sanitize sensitive tokens
	cfg.SyncToken = ""
	cfg.Modrinth.Token = ""
	for i := range cfg.SyncSources {
		cfg.SyncSources[i].Token = ""
	}

	outConfigPath := filepath.Join(outputDir, "cmm.toml")
	if err := config.SaveConfig(outConfigPath, cfg); err != nil {
		return fmt.Errorf("failed to export cmm.toml: %w", err)
	}

	lock, err := config.LoadLockfile(lockPath)
	if err != nil || lock == nil {
		lock = &config.Lockfile{Mods: []config.LockfileMod{}}
	}

	outLockPath := filepath.Join(outputDir, "cmm.lock")
	if err := config.SaveLockfile(outLockPath, lock); err != nil {
		return fmt.Errorf("failed to export cmm.lock: %w", err)
	}

	return nil
}
