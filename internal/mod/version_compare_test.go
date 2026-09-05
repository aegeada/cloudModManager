package mod

import (
	"testing"
)

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		candidate string
		current   string
		isNewer   bool
	}{
		// User's exact reported bugs:
		{"1.3.7+mc26.2", "1.3.8-beta.1+mc26.2", false},          // Downgrade
		{"30.29.0.201", "30.30.0.204", false},                   // Downgrade
		{"4.8.3", "4.9.0-beta.6", false},                        // Downgrade
		{"0.2.1+fabric.2b08348", "0.3.0-alpha.0.3+26.2", false}, // Downgrade

		// Legitimate upgrades:
		{"0.5.11", "0.5.8", true},
		{"1.3.8", "1.3.8-beta.1", true},        // Stable > Pre-release
		{"1.3.8-beta.2", "1.3.8-beta.1", true}, // Beta bump
		{"2.0.0", "1.9.9", true},
		{"30.30.0.205", "30.30.0.204", true},

		// Equals:
		{"1.0.0", "1.0.0", false},
		{"1.21-0.5.8", "1.21-0.5.8", false},
		{"v1.0.0", "1.0.0", false},

		// Hyphenated Minecraft prefixes (e.g. 1.21-0.5.8):
		{"1.21-0.5.9", "1.21-0.5.8", true},
		{"1.21-0.5.7", "1.21-0.5.8", false},
		{"1.21.1-0.5.8", "1.21-0.5.8", false}, // Same mod version 0.5.8 across MC patch
		{"1.21-0.5.0", "1.20-1.0.0", false},   // 0.5.0 is older than 1.0.0 despite higher MC prefix
		{"mc1.20.1-1.3.9", "mc1.20.1-1.3.8", true},
		{"mc1.21-0.6.0", "mc1.21-0.6.0-beta.1", true},

		// Cross-channel transition downgrade prevention:
		{"1.1.0", "1.2.0-beta.2", false},         // Release 1.1.0 is older than beta 1.2.0
		{"1.2.0", "1.2.0-beta.2", true},          // Release 1.2.0 is newer than beta 1.2.0
		{"1.2.0-beta.2", "1.2.0-alpha.1", true},  // Beta is newer than alpha of same version
		{"1.2.0-alpha.1", "1.2.0-beta.2", false}, // Alpha is older than beta
		{"1.3.0-alpha.1", "1.2.0", true},         // Alpha of next minor is newer than release of previous
		{"1.2.0-beta.1", "1.3.0-alpha.1", false}, // Older minor beta is not newer than newer minor alpha
		{"1.2.0-alpha.2", "1.2.0-alpha.1", true}, // Alpha bump

		// Release candidate & pre-release downgrade prevention:
		{"1.21.2", "1.21.2-rc1", true},          // Official release is newer than release candidate
		{"1.21.2-rc1", "1.21.2", false},         // Release candidate is not newer than official release
		{"1.21.2", "1.21.2-pre1", true},         // Official release is newer than pre-release
		{"1.21.2-pre1", "1.21.2", false},        // Pre-release is not newer than official release
		{"1.21.2-rc2", "1.21.2-rc1", true},      // RC bump
		{"1.21.2-rc1", "1.21.2-rc2", false},     // RC downgrade
		{"1.21.2-rc1", "1.21.2-pre2", true},     // RC is newer than pre of same version
		{"1.21.2-pre2", "1.21.2-rc1", false},    // Pre is older than RC

		// Empty/boundary inputs:
		{"", "1.0.0", false},
		{"1.0.0", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		got := IsNewerVersion(tt.candidate, tt.current)
		if got != tt.isNewer {
			t.Errorf("IsNewerVersion(%q, %q) = %v; want %v", tt.candidate, tt.current, got, tt.isNewer)
		}
	}
}

func TestCleanVersion(t *testing.T) {
	tests := []struct {
		input        string
		expectedCore string
		expectedPre  string
	}{
		{"1.21-0.5.8", "0.5.8", ""},
		{"mc1.20.1-1.0.0", "1.0.0", ""},
		{"1.21.2-rc1", "1.21.2", "rc1"},
		{"1.21.2-pre1", "1.21.2", "pre1"},
		{"1.21.2", "1.21.2", ""},
		{"1.21-v0.5.8", "0.5.8", ""},
		{"1.21.1_v2.0.0", "2.0.0", ""},
		{"mc1.21-0.6.0-beta.1", "0.6.0", "beta.1"},
	}

	for _, tt := range tests {
		core, pre := cleanVersion(tt.input)
		if core != tt.expectedCore || pre != tt.expectedPre {
			t.Errorf("cleanVersion(%q) = (%q, %q); want (%q, %q)",
				tt.input, core, pre, tt.expectedCore, tt.expectedPre)
		}
	}
}
