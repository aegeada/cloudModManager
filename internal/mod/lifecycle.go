package mod

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cmm/internal/config"
)

// LifecycleResult holds the outcome of an enable/disable operation.
type LifecycleResult struct {
	ModName      string
	Slug         string
	OldFileName  string
	NewFileName  string
	Disabled     bool
	Warning      string
	DependentMods []string
}

// DisableMod disables a mod by renaming its file to .jar.disabled and updating cmm.lock.
func (m *Manager) DisableMod(slugOrID string, force bool) (*LifecycleResult, error) {
	lock, err := config.LoadLockfile(m.LockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load lockfile '%s': %w", m.LockPath, err)
	}

	target := lock.GetMod(slugOrID)
	if target == nil {
		return nil, fmt.Errorf("mod '%s' not found in lockfile", slugOrID)
	}

	if target.Disabled {
		return &LifecycleResult{
			ModName:     target.Name,
			Slug:        target.Slug,
			OldFileName: target.FileName,
			NewFileName: target.FileName,
			Disabled:    true,
			Warning:     fmt.Sprintf("Mod '%s' is already disabled.", target.Name),
		}, nil
	}

	// Check if any other active mods depend on this mod
	var dependents []string
	// Find dependents if client is available
	// Also inspect lockfile for potential dependent tags or relationships
	cfg := m.loadConfig()
	modsDir := "mods"
	if cfg != nil && cfg.Paths.ModsDir != "" {
		modsDir = cfg.Paths.ModsDir
	}

	curFileName := target.FileName
	if curFileName == "" {
		curFileName = target.Slug + ".jar"
	}
	// Clean up .disabled if it was already somehow attached
	cleanBase := strings.TrimSuffix(curFileName, ".disabled")
	if !strings.HasSuffix(cleanBase, ".jar") {
		cleanBase = cleanBase + ".jar"
	}
	newFileName := cleanBase + ".disabled"

	oldPath := filepath.Join(modsDir, cleanBase)
	newPath := filepath.Join(modsDir, newFileName)

	// Check if old file exists on disk
	if _, err := os.Stat(oldPath); err == nil {
		if err := os.Rename(oldPath, newPath); err != nil {
			return nil, fmt.Errorf("failed to rename '%s' to '%s': %w", oldPath, newPath, err)
		}
	}

	target.Disabled = true
	target.FileName = newFileName
	lock.AddOrUpdateMod(*target)

	if err := config.SaveLockfile(m.LockPath, lock); err != nil {
		return nil, fmt.Errorf("failed to save lockfile: %w", err)
	}

	return &LifecycleResult{
		ModName:       target.Name,
		Slug:          target.Slug,
		OldFileName:   curFileName,
		NewFileName:   newFileName,
		Disabled:      true,
		DependentMods: dependents,
	}, nil
}

// EnableMod enables a mod by renaming its file from .jar.disabled to .jar and updating cmm.lock.
func (m *Manager) EnableMod(slugOrID string) (*LifecycleResult, error) {
	lock, err := config.LoadLockfile(m.LockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load lockfile '%s': %w", m.LockPath, err)
	}

	target := lock.GetMod(slugOrID)
	if target == nil {
		return nil, fmt.Errorf("mod '%s' not found in lockfile", slugOrID)
	}

	if !target.Disabled && strings.HasSuffix(target.FileName, ".jar") {
		return &LifecycleResult{
			ModName:     target.Name,
			Slug:        target.Slug,
			OldFileName: target.FileName,
			NewFileName: target.FileName,
			Disabled:    false,
			Warning:     fmt.Sprintf("Mod '%s' is already enabled.", target.Name),
		}, nil
	}

	cfg := m.loadConfig()
	modsDir := "mods"
	if cfg != nil && cfg.Paths.ModsDir != "" {
		modsDir = cfg.Paths.ModsDir
	}

	curFileName := target.FileName
	if curFileName == "" {
		curFileName = target.Slug + ".jar.disabled"
	}

	newFileName := strings.TrimSuffix(curFileName, ".disabled")
	if !strings.HasSuffix(newFileName, ".jar") {
		newFileName = newFileName + ".jar"
	}

	oldPath := filepath.Join(modsDir, curFileName)
	newPath := filepath.Join(modsDir, newFileName)

	// Check if old file exists on disk
	if _, err := os.Stat(oldPath); err == nil {
		if err := os.Rename(oldPath, newPath); err != nil {
			return nil, fmt.Errorf("failed to rename '%s' to '%s': %w", oldPath, newPath, err)
		}
	}

	target.Disabled = false
	target.FileName = newFileName
	lock.AddOrUpdateMod(*target)

	if err := config.SaveLockfile(m.LockPath, lock); err != nil {
		return nil, fmt.Errorf("failed to save lockfile: %w", err)
	}

	return &LifecycleResult{
		ModName:     target.Name,
		Slug:        target.Slug,
		OldFileName: curFileName,
		NewFileName: newFileName,
		Disabled:    false,
	}, nil
}
