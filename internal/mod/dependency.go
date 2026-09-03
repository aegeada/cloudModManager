package mod

import (
	"fmt"
	"strings"

	"cmm/internal/config"
	"cmm/internal/modrinth"
)

type installItem struct {
	Project *modrinth.Project
	Version *modrinth.Version
	File    *modrinth.VersionFile
}

// DependencyResolver resolves mod dependencies recursively and detects orphans.
type DependencyResolver struct {
	client *modrinth.Client
}

func NewDependencyResolver(client *modrinth.Client) *DependencyResolver {
	return &DependencyResolver{client: client}
}

// selectVersion finds the best matching version for a project given targetVersion, loader, gameVersion, and stability channel.
func (r *DependencyResolver) selectVersion(projectID string, targetVersion string, loader, gameVersion, channel string) (*modrinth.Version, error) {
	var loaders []string
	if loader != "" {
		loaders = []string{loader}
	}
	var gameVersions []string
	if gameVersion != "" {
		gameVersions = []string{gameVersion}
	}

	versions, err := r.client.GetProjectVersions(projectID, loaders, gameVersions, nil)
	if err != nil || len(versions) == 0 {
		// Fallback without filters in case of tag mismatch in mock/real endpoints
		versions, err = r.client.GetProjectVersions(projectID, nil, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch versions for project '%s': %w", projectID, err)
		}
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for project '%s'", projectID)
	}

	if targetVersion != "" {
		for _, v := range versions {
			if strings.EqualFold(v.VersionNumber, targetVersion) || strings.EqualFold(v.ID, targetVersion) {
				return &v, nil
			}
		}
		return nil, fmt.Errorf("version '%s' not found for project '%s'", targetVersion, projectID)
	}

	filtered := modrinth.FilterVersionsByChannel(versions, channel)
	if len(filtered) > 0 {
		return &filtered[0], nil
	}

	// Default fallback: pick the first/latest release version
	for _, v := range versions {
		if v.VersionType == "release" || v.VersionType == "" {
			return &v, nil
		}
	}
	return &versions[0], nil
}

// selectFile returns the primary VersionFile or the first file.
func selectFile(v *modrinth.Version) (*modrinth.VersionFile, error) {
	if len(v.Files) == 0 {
		return nil, fmt.Errorf("version '%s' has no downloadable files", v.ID)
	}
	for i := range v.Files {
		if v.Files[i].Primary {
			return &v.Files[i], nil
		}
	}
	return &v.Files[0], nil
}

func getDepProjectID(dep modrinth.Dependency, client *modrinth.Client) string {
	if dep.ProjectID != nil && *dep.ProjectID != "" {
		return *dep.ProjectID
	}
	if dep.VersionID != nil && *dep.VersionID != "" {
		if ver, err := client.GetVersion(*dep.VersionID); err == nil && ver.ProjectID != "" {
			return ver.ProjectID
		}
	}
	return ""
}

// ResolvePlan builds an ordered installation list and collects optional dependencies using default release channel.
func (r *DependencyResolver) ResolvePlan(
	targetSlugOrID string,
	targetVersion string,
	profile config.Profile,
	lock *config.Lockfile,
) ([]*installItem, []string, *modrinth.Project, error) {
	return r.ResolvePlanWithChannel(targetSlugOrID, targetVersion, "release", profile, lock)
}

// ResolvePlanWithChannel builds an ordered installation list and collects optional dependencies with channel filtering.
func (r *DependencyResolver) ResolvePlanWithChannel(
	targetSlugOrID string,
	targetVersion string,
	channel string,
	profile config.Profile,
	lock *config.Lockfile,
) ([]*installItem, []string, *modrinth.Project, error) {
	targetProj, err := r.client.GetProject(targetSlugOrID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to retrieve project '%s': %w", targetSlugOrID, err)
	}

	// If server-side profile and project is client-only (server_side == "unsupported"), return early
	if strings.EqualFold(profile.Side, "server") && strings.EqualFold(targetProj.ServerSide, "unsupported") {
		return nil, nil, targetProj, nil
	}

	targetVer, err := r.selectVersion(targetProj.ID, targetVersion, profile.Loader, profile.MinecraftVersion, channel)
	if err != nil {
		return nil, nil, targetProj, err
	}

	targetFile, err := selectFile(targetVer)
	if err != nil {
		return nil, nil, targetProj, err
	}

	var installQueue []*installItem
	var optionalDeps []string
	visited := make(map[string]bool)

	// Mark target project visited
	visited[strings.ToLower(targetProj.ID)] = true
	visited[strings.ToLower(targetProj.Slug)] = true

	var traverse func(proj *modrinth.Project, ver *modrinth.Version) error
	traverse = func(proj *modrinth.Project, ver *modrinth.Version) error {
		for _, dep := range ver.Dependencies {
			depID := getDepProjectID(dep, r.client)
			if depID == "" {
				continue
			}

			switch dep.DependencyType {
			case "incompatible":
				// Abort if already installed or scheduled
				if lock.GetMod(depID) != nil || visited[strings.ToLower(depID)] {
					return fmt.Errorf("incompatible dependency conflict: '%s' is incompatible with '%s'", proj.Slug, depID)
				}
			case "optional":
				name := depID
				if depProj, err := r.client.GetProject(depID); err == nil && depProj.Title != "" {
					if depProj.Slug != "" && !strings.EqualFold(depProj.Slug, depProj.Title) {
						name = fmt.Sprintf("%s (%s)", depProj.Title, depProj.Slug)
					} else {
						name = depProj.Title
					}
				}
				optionalDeps = append(optionalDeps, name)
			case "required":
				// If already installed in lockfile, skip
				if lock.GetMod(depID) != nil {
					continue
				}
				if visited[strings.ToLower(depID)] {
					continue
				}
				visited[strings.ToLower(depID)] = true

				depProj, err := r.client.GetProject(depID)
				if err != nil {
					return fmt.Errorf("failed to fetch dependency project '%s': %w", depID, err)
				}

				depVer, err := r.selectVersion(depProj.ID, "", profile.Loader, profile.MinecraftVersion, channel)
				if err != nil {
					return fmt.Errorf("failed to find compatible version for dependency '%s': %w", depProj.Slug, err)
				}

				depFile, err := selectFile(depVer)
				if err != nil {
					return err
				}

				// Recurse on dependency's own dependencies
				if err := traverse(depProj, depVer); err != nil {
					return err
				}

				installQueue = append(installQueue, &installItem{
					Project: depProj,
					Version: depVer,
					File:    depFile,
				})
			}
		}
		return nil
	}

	if err := traverse(targetProj, targetVer); err != nil {
		return nil, nil, targetProj, err
	}

	// Add target mod at the end
	installQueue = append(installQueue, &installItem{
		Project: targetProj,
		Version: targetVer,
		File:    targetFile,
	})

	return installQueue, optionalDeps, targetProj, nil
}

// DetectOrphans finds dependencies that were required by modToRemove but are not needed by any other remaining mod.
func (r *DependencyResolver) DetectOrphans(modToRemove string, lock *config.Lockfile) ([]string, error) {
	targetMod := lock.GetMod(modToRemove)
	if targetMod == nil {
		return nil, fmt.Errorf("mod '%s' not found in lockfile", modToRemove)
	}

	// Step 1: Find dependencies of targetMod
	var targetDeps []string
	if targetProj, err := r.client.GetProject(targetMod.Slug); err == nil {
		if versions, err := r.client.GetProjectVersions(targetProj.ID, nil, nil, nil); err == nil && len(versions) > 0 {
			for _, v := range versions {
				if v.VersionNumber == targetMod.GetVersion() || v.ID == targetMod.VersionID || len(versions) == 1 {
					for _, d := range v.Dependencies {
						if d.DependencyType == "required" {
							depID := getDepProjectID(d, r.client)
							if depID != "" {
								targetDeps = append(targetDeps, depID)
							}
						}
					}
					break
				}
			}
		}
	}

	// Only consider dependencies that are currently installed in the lockfile
	var candidateOrphans []string
	for _, depID := range targetDeps {
		if installed := lock.GetMod(depID); installed != nil {
			candidateOrphans = append(candidateOrphans, installed.Slug)
		}
	}

	if len(candidateOrphans) == 0 {
		return nil, nil
	}

	// Step 2: Collect dependencies of all OTHER remaining mods
	remainingRequired := make(map[string]bool)
	for _, otherMod := range lock.Mods {
		if strings.EqualFold(otherMod.Slug, targetMod.Slug) ||
			strings.EqualFold(otherMod.ProjectID, targetMod.ProjectID) ||
			strings.EqualFold(otherMod.Name, targetMod.Name) {
			continue
		}

		if otherProj, err := r.client.GetProject(otherMod.Slug); err == nil {
			if versions, err := r.client.GetProjectVersions(otherProj.ID, nil, nil, nil); err == nil && len(versions) > 0 {
				for _, v := range versions {
					if v.VersionNumber == otherMod.GetVersion() || v.ID == otherMod.VersionID || len(versions) == 1 {
						for _, d := range v.Dependencies {
							if d.DependencyType == "required" {
								depID := getDepProjectID(d, r.client)
								if depID != "" {
									if depMod := lock.GetMod(depID); depMod != nil {
										remainingRequired[strings.ToLower(depMod.Slug)] = true
										remainingRequired[strings.ToLower(depMod.Name)] = true
										remainingRequired[strings.ToLower(depMod.ProjectID)] = true
									}
								}
							}
						}
						break
					}
				}
			}
		}
	}

	// Step 3: Candidate orphan is truly orphaned if not in remainingRequired
	var trueOrphans []string
	for _, orphanSlug := range candidateOrphans {
		if !remainingRequired[strings.ToLower(orphanSlug)] {
			trueOrphans = append(trueOrphans, orphanSlug)
		}
	}

	return trueOrphans, nil
}

// DetectActiveDependents finds all active (!Disabled) installed mods in lockfile that have a required dependency on targetSlugOrID.
func (r *DependencyResolver) DetectActiveDependents(targetSlugOrID string, lock *config.Lockfile) ([]string, error) {
	if lock == nil || len(lock.Mods) == 0 {
		return nil, nil
	}

	targetMod := lock.GetMod(targetSlugOrID)
	if targetMod == nil {
		return nil, fmt.Errorf("mod '%s' not found in lockfile", targetSlugOrID)
	}

	if r.client == nil {
		return nil, nil
	}

	var activeDependents []string
	seen := make(map[string]bool)

	for _, otherMod := range lock.Mods {
		// Skip self
		if strings.EqualFold(otherMod.Slug, targetMod.Slug) ||
			(otherMod.ProjectID != "" && targetMod.ProjectID != "" && strings.EqualFold(otherMod.ProjectID, targetMod.ProjectID)) ||
			strings.EqualFold(otherMod.Name, targetMod.Name) {
			continue
		}

		// Filter ONLY active mods (skip disabled mods)
		if otherMod.Disabled {
			continue
		}

		lookupKey := otherMod.Slug
		if lookupKey == "" {
			lookupKey = otherMod.ProjectID
		}
		if lookupKey == "" {
			lookupKey = otherMod.Name
		}

		otherProj, err := r.client.GetProject(lookupKey)
		if err != nil && otherMod.ProjectID != "" && otherMod.ProjectID != lookupKey {
			otherProj, err = r.client.GetProject(otherMod.ProjectID)
		}
		if err != nil {
			continue
		}

		versions, err := r.client.GetProjectVersions(otherProj.ID, nil, nil, nil)
		if err != nil || len(versions) == 0 {
			continue
		}

		for _, v := range versions {
			if v.VersionNumber == otherMod.GetVersion() || v.ID == otherMod.VersionID || len(versions) == 1 {
				for _, d := range v.Dependencies {
					if d.DependencyType == "required" {
						depID := getDepProjectID(d, r.client)
						if depID != "" {
							if depMod := lock.GetMod(depID); depMod != nil {
								if strings.EqualFold(depMod.Slug, targetMod.Slug) ||
									(depMod.ProjectID != "" && targetMod.ProjectID != "" && strings.EqualFold(depMod.ProjectID, targetMod.ProjectID)) ||
									strings.EqualFold(depMod.Name, targetMod.Name) {
									depSlug := otherMod.Slug
									if depSlug == "" {
										depSlug = otherMod.Name
									}
									if !seen[strings.ToLower(depSlug)] {
										seen[strings.ToLower(depSlug)] = true
										activeDependents = append(activeDependents, depSlug)
									}
								}
							}
						}
					}
				}
				break
			}
		}
	}

	return activeDependents, nil
}

