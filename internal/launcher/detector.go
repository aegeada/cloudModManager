package launcher

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// Detector discovers Minecraft launcher instances on the local system.
type Detector struct {
	opts     DetectorOptions
	resolver *PathResolver
}

// NewDetector creates a new Detector with the given options.
func NewDetector(opts DetectorOptions) *Detector {
	return &Detector{
		opts:     opts,
		resolver: NewPathResolver(opts),
	}
}

// DetectAll probes all supported Minecraft launchers and returns all discovered instances.
func (d *Detector) DetectAll() ([]Instance, error) {
	launchers := []LauncherType{
		LauncherVanilla,
		LauncherPrism,
		LauncherModrinth,
		LauncherCurseForge,
	}

	var allInstances []Instance
	seenDirs := make(map[string]bool)

	for _, lt := range launchers {
		insts, err := d.DetectLauncher(lt)
		if err != nil {
			// Skip individual launcher probe errors so others still succeed
			continue
		}
		for _, inst := range insts {
			if !seenDirs[inst.InstanceDir] {
				seenDirs[inst.InstanceDir] = true
				allInstances = append(allInstances, inst)
			}
		}
	}

	// Sort instances by LauncherName, then Name
	sort.Slice(allInstances, func(i, j int) bool {
		if allInstances[i].LauncherName != allInstances[j].LauncherName {
			return allInstances[i].LauncherName < allInstances[j].LauncherName
		}
		return allInstances[i].Name < allInstances[j].Name
	})

	return allInstances, nil
}

// DetectLauncher probes the paths for a specific launcher type.
func (d *Detector) DetectLauncher(lt LauncherType) ([]Instance, error) {
	normType := NormalizeLauncherType(string(lt))
	if normType == LauncherAll {
		return d.DetectAll()
	}

	paths := d.resolver.ResolveCandidatePaths(normType)
	var instances []Instance
	seen := make(map[string]bool)

	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil || !fi.IsDir() {
			continue
		}

		var found []Instance
		switch normType {
		case LauncherVanilla:
			found, _ = ParseVanillaProfiles(p)
		case LauncherPrism:
			found, _ = ParsePrismInstances(p)
		case LauncherModrinth:
			found, _ = ParseModrinthProfiles(p)
		case LauncherCurseForge:
			found, _ = ParseCurseForgeInstances(p)
		}

		for _, inst := range found {
			key := fmt.Sprintf("%s:%s", inst.Launcher, inst.InstanceDir)
			if !seen[key] {
				seen[key] = true
				instances = append(instances, inst)
			}
		}
	}

	return instances, nil
}

// FindInstance searches for an instance matching query across detected launchers.
func (d *Detector) FindInstance(query string, launcherType LauncherType) (*Instance, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("instance name or query cannot be empty")
	}

	var candidates []Instance
	var err error

	normType := NormalizeLauncherType(string(launcherType))
	if normType == LauncherAll || normType == "" {
		candidates, err = d.DetectAll()
	} else {
		candidates, err = d.DetectLauncher(normType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed detecting instances: %w", err)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no launcher instances detected (searching for '%s')", query)
	}

	// 1. Exact match (case-insensitive) on Name or ID
	var exactMatches []Instance
	for _, inst := range candidates {
		if strings.EqualFold(inst.Name, query) || strings.EqualFold(inst.ID, query) {
			exactMatches = append(exactMatches, inst)
		}
	}

	if len(exactMatches) == 1 {
		return &exactMatches[0], nil
	} else if len(exactMatches) > 1 {
		var names []string
		for _, m := range exactMatches {
			names = append(names, fmt.Sprintf("'%s' (%s - %s)", m.Name, m.LauncherName, m.InstanceDir))
		}
		return nil, fmt.Errorf("multiple instances match '%s':\n  - %s\nUse --launcher <type> to disambiguate",
			query, strings.Join(names, "\n  - "))
	}

	// 2. Substring / Prefix match (case-insensitive)
	var partialMatches []Instance
	lowerQuery := strings.ToLower(query)
	for _, inst := range candidates {
		if strings.Contains(strings.ToLower(inst.Name), lowerQuery) ||
			strings.Contains(strings.ToLower(inst.ID), lowerQuery) {
			partialMatches = append(partialMatches, inst)
		}
	}

	if len(partialMatches) == 1 {
		return &partialMatches[0], nil
	} else if len(partialMatches) > 1 {
		var names []string
		for _, m := range partialMatches {
			names = append(names, fmt.Sprintf("'%s' (%s - %s)", m.Name, m.LauncherName, m.InstanceDir))
		}
		return nil, fmt.Errorf("multiple instances partially match '%s':\n  - %s\nPlease specify a more specific name or use --launcher <type>",
			query, strings.Join(names, "\n  - "))
	}

	return nil, fmt.Errorf("no launcher instance found matching '%s'", query)
}
