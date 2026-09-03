package loader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// GetFabricMetaURL returns the configured Fabric Meta API base URL or default.
func GetFabricMetaURL() string {
	baseURL := os.Getenv("FABRIC_META_URL")
	if baseURL == "" {
		baseURL = "https://meta.fabricmc.net"
	}
	return strings.TrimSuffix(baseURL, "/")
}

// GetFabricLoaderVersions fetches the list of available Fabric loader versions from Fabric Meta API.
func GetFabricLoaderVersions() ([]FabricVersion, error) {
	baseURL := GetFabricMetaURL()
	endpoint := baseURL + "/v2/versions/loader"

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create fabric meta request: %w", err)
	}
	req.Header.Set("User-Agent", "CloudModManager/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query fabric meta API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fabric meta API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var versions []FabricVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, fmt.Errorf("failed to decode fabric loader versions: %w", err)
	}

	return versions, nil
}

// ValidateFabricVersion checks if the specified Fabric loader version exists.
func ValidateFabricVersion(version string) (bool, error) {
	versions, err := GetFabricLoaderVersions()
	if err != nil {
		// If network / DNS lookup failed, fallback to semver format check
		ver := strings.TrimSpace(version)
		if strings.Count(ver, ".") >= 1 && strings.ContainsAny(ver, "0123456789") {
			return true, nil
		}
		return false, fmt.Errorf("unsupported or invalid fabric loader version '%s': %w", version, err)
	}

	for _, v := range versions {
		if strings.EqualFold(v.Version, version) {
			return true, nil
		}
	}

	return false, fmt.Errorf("unsupported or invalid fabric loader version '%s'", version)
}
