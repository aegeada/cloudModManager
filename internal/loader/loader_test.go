package loader

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupMockFabricMetaServer(t *testing.T) (*httptest.Server, func()) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/versions/loader" {
			versions := []FabricVersion{
				{
					Separator: ".",
					Build:     1903,
					Maven:     "net.fabricmc:fabric-loader:0.19.3",
					Version:   "0.19.3",
					Stable:    true,
				},
				{
					Separator: ".",
					Build:     1902,
					Maven:     "net.fabricmc:fabric-loader:0.19.2",
					Version:   "0.19.2",
					Stable:    true,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(versions)
			return
		}
		http.NotFound(w, r)
	}))

	origURL := os.Getenv("FABRIC_META_URL")
	os.Setenv("FABRIC_META_URL", server.URL)

	cleanup := func() {
		server.Close()
		if origURL != "" {
			os.Setenv("FABRIC_META_URL", origURL)
		} else {
			os.Unsetenv("FABRIC_META_URL")
		}
	}

	return server, cleanup
}

func TestGetFabricLoaderVersions(t *testing.T) {
	_, cleanup := setupMockFabricMetaServer(t)
	defer cleanup()

	versions, err := GetFabricLoaderVersions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if versions[0].Version != "0.19.3" {
		t.Errorf("expected version 0.19.3, got %s", versions[0].Version)
	}
}

func TestValidateFabricVersion(t *testing.T) {
	_, cleanup := setupMockFabricMetaServer(t)
	defer cleanup()

	valid, err := ValidateFabricVersion("0.19.3")
	if err != nil || !valid {
		t.Errorf("expected 0.19.3 to be valid, got valid=%v, err=%v", valid, err)
	}

	valid, err = ValidateFabricVersion("99.9")
	if valid || err == nil {
		t.Errorf("expected 99.9 to be invalid, got valid=%v, err=%v", valid, err)
	}
	if err != nil && !strings.Contains(err.Error(), "version") {
		t.Errorf("expected error message to mention version, got %v", err)
	}
}

func TestListLoaders(t *testing.T) {
	_, cleanup := setupMockFabricMetaServer(t)
	defer cleanup()

	// All loaders
	loaders, err := ListLoaders("")
	if err != nil || len(loaders) < 3 {
		t.Errorf("ListLoaders('') failed: %v", err)
	}

	// Fabric
	fabricLoaders, err := ListLoaders("fabric")
	if err != nil || len(fabricLoaders) != 2 {
		t.Errorf("ListLoaders('fabric') failed: %v", err)
	}

	// Forge
	forgeLoaders, err := ListLoaders("forge")
	if err != nil || len(forgeLoaders) == 0 {
		t.Errorf("ListLoaders('forge') failed: %v", err)
	}

	// Unknown
	_, err = ListLoaders("unknown-loader")
	if err == nil {
		t.Error("expected error for unknown loader")
	}
}

func TestInstallLoader(t *testing.T) {
	_, cleanup := setupMockFabricMetaServer(t)
	defer cleanup()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cmm.toml")

	// Install fabric with specific valid version
	err := InstallLoader(configPath, "fabric", "0.19.3")
	if err != nil {
		t.Fatalf("InstallLoader fabric failed: %v", err)
	}

	// Install fabric with invalid version
	err = InstallLoader(configPath, "fabric", "99.9")
	if err == nil {
		t.Fatalf("expected error installing invalid fabric version 99.9")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("expected error to contain 'version', got %v", err)
	}

	// Install neoforge
	err = InstallLoader(configPath, "neoforge", "")
	if err != nil {
		t.Fatalf("InstallLoader neoforge failed: %v", err)
	}
}
