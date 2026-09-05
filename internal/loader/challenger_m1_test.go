package loader

import (
	"testing"
)

func TestChallenger_CheckLatestLoaderVersion_NoDowngrade(t *testing.T) {
	_, cleanup := setupMockFabricMetaServer(t)
	defer cleanup()

	// The mock server returns 0.19.3 as latest stable, and 0.19.2 as older stable.

	testCases := []struct {
		currentVersion  string
		wantUpdate      bool
		wantLatest      string
		desc            string
	}{
		{"0.19.2", true, "0.19.3", "older version should update"},
		{"v0.19.2", true, "0.19.3", "older version with 'v' prefix should update"},
		{"0.19.3", false, "0.19.3", "exact version should not update"},
		{"v0.19.3", false, "0.19.3", "exact version with 'v' should not update"},
		{"0.19.4", false, "0.19.3", "newer patch version must NEVER downgrade"},
		{"0.20.0", false, "0.19.3", "newer minor version must NEVER downgrade"},
		{"1.0.0", false, "0.19.3", "newer major version must NEVER downgrade"},
		{"0.20.0-beta.1", false, "0.19.3", "newer pre-release must NEVER downgrade"},
	}

	for _, tc := range testCases {
		latest, updateAvailable, err := CheckLatestLoaderVersion("fabric", tc.currentVersion)
		if err != nil {
			t.Fatalf("[%s] CheckLatestLoaderVersion failed: %v", tc.desc, err)
		}
		if latest != tc.wantLatest {
			t.Errorf("[%s] got latest=%q; want %q", tc.desc, latest, tc.wantLatest)
		}
		if updateAvailable != tc.wantUpdate {
			t.Errorf("[%s] got updateAvailable=%v; want %v (current: %s, latest: %s)",
				tc.desc, updateAvailable, tc.wantUpdate, tc.currentVersion, latest)
		}
	}
}
