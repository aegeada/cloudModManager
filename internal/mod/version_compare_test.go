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
		{"1.3.7+mc26.2", "1.3.8-beta.1+mc26.2", false},                 // Downgrade
		{"30.29.0.201", "30.30.0.204", false},                         // Downgrade
		{"4.8.3", "4.9.0-beta.6", false},                               // Downgrade
		{"0.2.1+fabric.2b08348", "0.3.0-alpha.0.3+26.2", false},       // Downgrade

		// Legitimate upgrades:
		{"0.5.11", "0.5.8", true},
		{"1.3.8", "1.3.8-beta.1", true},                                // Stable > Pre-release
		{"1.3.8-beta.2", "1.3.8-beta.1", true},                          // Beta bump
		{"2.0.0", "1.9.9", true},
		{"30.30.0.205", "30.30.0.204", true},

		// Equals:
		{"1.0.0", "1.0.0", false},
		{"1.21-0.5.8", "1.21-0.5.8", false},
		{"v1.0.0", "1.0.0", false},
	}

	for _, tt := range tests {
		got := IsNewerVersion(tt.candidate, tt.current)
		if got != tt.isNewer {
			t.Errorf("IsNewerVersion(%q, %q) = %v; want %v", tt.candidate, tt.current, got, tt.isNewer)
		}
	}
}
