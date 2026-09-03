package config

import (
	"fmt"
	"testing"
)

// TestChallenger_ExtractModSlug_RealisticPermutations tests standard Minecraft mod naming conventions
// across active, disabled, loader-tagged, and versioned filenames.
func TestChallenger_ExtractModSlug_RealisticPermutations(t *testing.T) {
	loaders := []string{"", "-fabric", "+fabric", "_fabric", "-forge", "+forge", "_forge", "-quilt", "+quilt", "_quilt", "-neoforge", "+neoforge", "_neoforge"}
	mcVersions := []string{"", "-mc1.20.1", "+mc1.21", "-1.21.1", "+1.20.4", "_1.19.2", "-1.20"}
	semvers := []string{"", "-0.5.8", "-v1.0.0", "-15.0.127", "-2.5.1", "-0.11.0"}
	extensions := []string{".jar", ".jar.disabled"}

	baseMods := []struct {
		baseSlug string
		expected string
	}{
		{"sodium", "sodium"},
		{"lithium", "lithium"},
		{"iris", "iris"},
		{"appleskin", "appleskin"},
		{"cloth-config", "cloth-config"},
		{"ferrite-core", "ferrite-core"},
		{"mod_with_underscores", "mod_with_underscores"},
	}

	for _, bm := range baseMods {
		for _, l := range loaders {
			for _, mc := range mcVersions {
				for _, sv := range semvers {
					for _, ext := range extensions {
						filename := fmt.Sprintf("%s%s%s%s%s", bm.baseSlug, l, mc, sv, ext)
						slug := ExtractModSlug(filename)
						if slug != bm.expected {
							t.Errorf("ExtractModSlug(%q) = %q, expected %q (loader: %q, mc: %q, semver: %q, ext: %q)",
								filename, slug, bm.expected, l, mc, sv, ext)
						}
					}
				}
			}
		}
	}
}

// TestChallenger_ExtractModSlug_ProtectedPrefixes verifies that loader-named mods
// (like fabric-api and forge-config-api) are not corrupted during slug extraction.
func TestChallenger_ExtractModSlug_ProtectedPrefixes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"fabric-api-0.92.0+1.21.jar", "fabric-api"},
		{"fabric-api-0.92.0+1.21.jar.disabled", "fabric-api"},
		{"fabric-api-0.100.0-1.21.1.jar.disabled", "fabric-api"},
		{"fabric-api.jar", "fabric-api"},
		{"fabric-api.jar.disabled", "fabric-api"},
		{"forge-config-api-port-fabric-10.0.0.jar", "forge-config-api-port"},
		{"forge-config-api-port-fabric-10.0.0.jar.disabled", "forge-config-api-port"},
		{"forge-config-api-1.20.1.jar.disabled", "forge-config-api"},
	}

	for _, tt := range tests {
		got := ExtractModSlug(tt.input)
		if got != tt.expected {
			t.Errorf("ExtractModSlug(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

// TestChallenger_ExtractModSlug_EdgeCases tests boundary and special input formats.
func TestChallenger_ExtractModSlug_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"whitespace only", "   \t\n  ", ""},
		{"only .jar", ".jar", ""},
		{"only .jar.disabled", ".jar.disabled", ""},
		{"only .disabled", ".disabled", ""},
		{"multiple disabled suffixes", "mod-1.0.0.jar.disabled.disabled.disabled", "mod"},
		{"nested unix path", "path/to/mods/dir/sodium-fabric-0.5.8.jar.disabled", "sodium"},
		{"uppercase extensions", "MOD-FABRIC-1.0.0.JAR.DISABLED", "mod"},
		{"dots in mod name", "my.great.mod-1.0.0.jar.disabled", "my.great.mod"},
		{"digits in mod name", "xaero9minimap-fabric-1.21.jar.disabled", "xaero9minimap"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractModSlug(tt.input)
			if got != tt.expected {
				t.Errorf("ExtractModSlug(%q) = %q, expected %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestChallenger_MatchMod_Permutations tests matchMod across all query variants and mod states.
func TestChallenger_MatchMod_Permutations(t *testing.T) {
	activeMod := &LockfileMod{
		Slug:          "sodium",
		Name:          "Sodium",
		ProjectID:     "AANobbMI",
		VersionNumber: "0.5.8",
		FileName:      "sodium-fabric-0.5.8.jar",
		Disabled:      false,
	}

	disabledMod := &LockfileMod{
		Slug:          "sodium",
		Name:          "Sodium",
		ProjectID:     "AANobbMI",
		VersionNumber: "0.5.8",
		FileName:      "sodium-fabric-0.5.8.jar.disabled",
		Disabled:      true,
	}

	validQueries := []string{
		"sodium",
		"Sodium",
		"SODIUM",
		"AANobbMI",
		"aanobbmi",
		"sodium-fabric-0.5.8.jar",
		"sodium-fabric-0.5.8.jar.disabled",
		"sodium-fabric-0.5.8",
		"sodium-fabric",
		"Sodium-Fabric-0.5.8.jar",
		"Sodium-Fabric-0.5.8.JAR.DISABLED",
	}

	for _, q := range validQueries {
		if !matchMod(activeMod, q) {
			t.Errorf("matchMod(activeMod, %q) = false, expected true", q)
		}
		if !matchMod(disabledMod, q) {
			t.Errorf("matchMod(disabledMod, %q) = false, expected true", q)
		}
	}

	invalidQueries := []string{
		"",
		"lithium",
		"sodium-extra",
		"fabric-api",
		"random-mod",
		"AANobbMI2",
	}

	for _, q := range invalidQueries {
		if matchMod(activeMod, q) {
			t.Errorf("matchMod(activeMod, %q) = true, expected false", q)
		}
		if matchMod(disabledMod, q) {
			t.Errorf("matchMod(disabledMod, %q) = true, expected false", q)
		}
	}

	// Nil mod handling
	if matchMod(nil, "sodium") {
		t.Errorf("matchMod(nil, %q) = true, expected false", "sodium")
	}
}

// TestChallenger_Vulnerability_NumberedSlugCollision demonstrates the matchMod false positive collision
// bug between distinct numbered slugs like "mod-1" and "mod-2" or "chain-mod-00" and "chain-mod-49".
func TestChallenger_Vulnerability_NumberedSlugCollision(t *testing.T) {
	lock := &Lockfile{
		Mods: []LockfileMod{
			{
				Slug:          "mod-01",
				Name:          "Mod 01",
				ProjectID:     "id-01",
				VersionNumber: "1.0",
				FileName:      "mod-01-1.0.jar",
			},
			{
				Slug:          "mod-02",
				Name:          "Mod 02",
				ProjectID:     "id-02",
				VersionNumber: "1.0",
				FileName:      "mod-02-1.0.jar",
			},
		},
	}

	// Direct query for "mod-02" should return mod-02, NOT mod-01
	target := lock.GetMod("mod-02")
	if target == nil {
		t.Fatalf("GetMod(\"mod-02\") returned nil")
	}
	if target.Slug != "mod-02" {
		t.Errorf("BUG CONFIRMED: GetMod(\"mod-02\") returned %q instead of \"mod-02\" due to ExtractModSlug stripping trailing numbers in matchMod", target.Slug)
	}
}

// TestChallenger_Vulnerability_DisabledWithoutJarExtension demonstrates the bug where
// filenames ending in .disabled without .jar (e.g. "mod+mc1.21.disabled") have numeric version digits
// stripped by filepath.Ext before regex parsing.
func TestChallenger_Vulnerability_DisabledWithoutJarExtension(t *testing.T) {
	input := "ferrite-core+fabric+mc1.21.disabled"
	expected := "ferrite-core"
	got := ExtractModSlug(input)
	if got != expected {
		t.Errorf("BUG CONFIRMED: ExtractModSlug(%q) = %q, expected %q (filepath.Ext treated .21 as file extension)", input, got, expected)
	}
}
