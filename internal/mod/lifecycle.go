package mod

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cmm/internal/config"
)

// DetectActiveDependents finds all active installed mods in lockfile that have a required dependency on targetSlugOrID.
func (m *Manager) DetectActiveDependents(slugOrID string, lock *config.Lockfile) ([]string, error) {
	resolver := m.Resolver
	if resolver == nil {
		resolver = NewDependencyResolver(m.Client)
	}
	return resolver.DetectActiveDependents(slugOrID, lock)
}

// DisableMod disables a mod by renaming its file to .jar.disabled and updating cmm.lock.
func (m *Manager) DisableMod(slugOrID string, force bool) (*DisableResult, error) {
	return m.DisableModWithOptions(slugOrID, DisableOptions{Force: force})
}

// DisableModWithOptions disables a mod according to DisableOptions.
func (m *Manager) DisableModWithOptions(slugOrID string, opts DisableOptions) (*DisableResult, error) {
	lock, err := config.LoadLockfile(m.LockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load lockfile '%s': %w", m.LockPath, err)
	}

	target := lock.GetMod(slugOrID)
	if target == nil {
		return nil, fmt.Errorf("mod '%s' not found in lockfile", slugOrID)
	}

	if target.Disabled {
		return &DisableResult{
			Slug:            target.Slug,
			Name:            target.Name,
			OldFileName:     target.FileName,
			NewFileName:     target.FileName,
			AlreadyDisabled: true,
		}, nil
	}

	// Detect active dependents
	resolver := m.Resolver
	if resolver == nil {
		resolver = NewDependencyResolver(m.Client)
	}
	dependents, _ := resolver.DetectActiveDependents(slugOrID, lock)
	if len(dependents) == 0 && target.Slug != slugOrID {
		dependents, _ = resolver.DetectActiveDependents(target.Slug, lock)
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

	// If active dependents exist and !opts.Force, return warning and DO NOT modify disk or lockfile
	if len(dependents) > 0 && !opts.Force {
		return &DisableResult{
			Slug:                 target.Slug,
			Name:                 target.Name,
			OldFileName:          curFileName,
			NewFileName:          curFileName,
			HasDependentsWarning: true,
			ActiveDependents:     dependents,
		}, nil
	}

	if opts.DryRun {
		return &DisableResult{
			Slug:             target.Slug,
			Name:             target.Name,
			OldFileName:      curFileName,
			NewFileName:      newFileName,
			DryRun:           true,
			ActiveDependents: dependents,
		}, nil
	}

	cfg := m.loadConfig()
	modsDir := "mods"
	if cfg != nil && cfg.Paths.ModsDir != "" {
		modsDir = cfg.Paths.ModsDir
	}

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

	return &DisableResult{
		Slug:             target.Slug,
		Name:             target.Name,
		OldFileName:      curFileName,
		NewFileName:      newFileName,
		ActiveDependents: dependents,
	}, nil
}

// EnableMod enables a mod by renaming its file from .jar.disabled to .jar and updating cmm.lock.
func (m *Manager) EnableMod(slugOrID string) (*EnableResult, error) {
	return m.EnableModWithOptions(slugOrID, EnableOptions{})
}

// EnableModWithOptions enables a mod according to EnableOptions.
func (m *Manager) EnableModWithOptions(slugOrID string, opts EnableOptions) (*EnableResult, error) {
	lock, err := config.LoadLockfile(m.LockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load lockfile '%s': %w", m.LockPath, err)
	}

	target := lock.GetMod(slugOrID)
	if target == nil {
		return nil, fmt.Errorf("mod '%s' not found in lockfile", slugOrID)
	}

	if !target.Disabled && strings.HasSuffix(target.FileName, ".jar") {
		return &EnableResult{
			Slug:           target.Slug,
			Name:           target.Name,
			OldFileName:    target.FileName,
			NewFileName:    target.FileName,
			AlreadyEnabled: true,
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

	if opts.DryRun {
		return &EnableResult{
			Slug:        target.Slug,
			Name:        target.Name,
			OldFileName: curFileName,
			NewFileName: newFileName,
			DryRun:      true,
		}, nil
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

	return &EnableResult{
		Slug:        target.Slug,
		Name:        target.Name,
		OldFileName: curFileName,
		NewFileName: newFileName,
	}, nil
}
