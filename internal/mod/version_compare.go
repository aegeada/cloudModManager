package mod

import (
	"regexp"
	"strconv"
	"strings"
)

// cleanVersion cleans version string by trimming 'v', 'mc' prefixes and build metadata.
func cleanVersion(v string) (core string, prerelease string) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")

	// Strip build metadata after '+'
	if idx := strings.Index(v, "+"); idx != -1 {
		v = v[:idx]
	}

	// Split core and pre-release (e.g., 1.3.8-beta.1)
	if idx := strings.Index(v, "-"); idx != -1 {
		core = v[:idx]
		prerelease = v[idx+1:]
	} else {
		core = v
	}

	// Strip common prefixes like mc1.21- or 1.21.1_ from core if followed by delimiter
	mcPrefixRegex := regexp.MustCompile(`^(?:mc)?1\.(?:1[2-9]|2[0-9])(?:\.[0-9]+)?[-_]`)
	core = mcPrefixRegex.ReplaceAllString(core, "")

	return core, prerelease
}

// parseParts parses dot-separated tokens
func parseParts(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ".")
}

// CompareVersions compares two version strings according to SemVer principles.
// Returns:
//   1 if v1 > v2
//  -1 if v1 < v2
//   0 if v1 == v2
func CompareVersions(v1, v2 string) int {
	if strings.EqualFold(strings.TrimSpace(v1), strings.TrimSpace(v2)) {
		return 0
	}

	core1, pre1 := cleanVersion(v1)
	core2, pre2 := cleanVersion(v2)

	parts1 := parseParts(core1)
	parts2 := parseParts(core2)

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 string
		if i < len(parts1) {
			p1 = parts1[i]
		}
		if i < len(parts2) {
			p2 = parts2[i]
		}

		n1, err1 := strconv.Atoi(p1)
		n2, err2 := strconv.Atoi(p2)

		if err1 == nil && err2 == nil {
			if n1 > n2 {
				return 1
			}
			if n1 < n2 {
				return -1
			}
		} else {
			cmp := strings.Compare(p1, p2)
			if cmp != 0 {
				return cmp
			}
		}
	}

	// Core versions are equal. Check pre-release.
	// Version WITHOUT pre-release is GREATER than version WITH pre-release (e.g. 1.3.8 > 1.3.8-beta.1).
	if pre1 == "" && pre2 != "" {
		return 1
	}
	if pre1 != "" && pre2 == "" {
		return -1
	}
	if pre1 != "" && pre2 != "" {
		preParts1 := parseParts(pre1)
		preParts2 := parseParts(pre2)
		preMax := len(preParts1)
		if len(preParts2) > preMax {
			preMax = len(preParts2)
		}

		for i := 0; i < preMax; i++ {
			var p1, p2 string
			if i < len(preParts1) {
				p1 = preParts1[i]
			}
			if i < len(preParts2) {
				p2 = preParts2[i]
			}

			n1, err1 := strconv.Atoi(p1)
			n2, err2 := strconv.Atoi(p2)
			if err1 == nil && err2 == nil {
				if n1 > n2 {
					return 1
				}
				if n1 < n2 {
					return -1
				}
			} else {
				cmp := strings.Compare(p1, p2)
				if cmp != 0 {
					return cmp
				}
			}
		}
	}

	return 0
}

// IsNewerVersion returns true if candidate is strictly greater than current according to SemVer.
func IsNewerVersion(candidate, current string) bool {
	if candidate == "" || current == "" {
		return false
	}
	return CompareVersions(candidate, current) > 0
}
