package selfupdate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestCheckUpdate_UpdateAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := ReleaseInfo{
			TagName: "v0.2.0",
			Name:    "v0.2.0 Release",
			Assets: []AssetInfo{
				{Name: "cmm-v0.2.0-linux-amd64", BrowserDownloadURL: "http://example.com/bin"},
			},
		}
		json.NewEncoder(w).Encode(rel)
	}))
	defer server.Close()

	os.Setenv("CMM_UPDATE_URL", server.URL)
	defer os.Unsetenv("CMM_UPDATE_URL")

	rel, available, err := CheckUpdate("aegeada/cloudModManager", "0.1.0")
	if err != nil {
		t.Fatalf("CheckUpdate failed: %v", err)
	}
	if !available {
		t.Errorf("expected update available, got false")
	}
	if rel.TagName != "v0.2.0" {
		t.Errorf("expected tag v0.2.0, got %s", rel.TagName)
	}
}

func TestCheckUpdate_AlreadyUpToDate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := ReleaseInfo{
			TagName: "v0.1.0",
		}
		json.NewEncoder(w).Encode(rel)
	}))
	defer server.Close()

	os.Setenv("CMM_UPDATE_URL", server.URL)
	defer os.Unsetenv("CMM_UPDATE_URL")

	_, available, err := CheckUpdate("aegeada/cloudModManager", "0.1.0")
	if err != nil {
		t.Fatalf("CheckUpdate failed: %v", err)
	}
	if available {
		t.Errorf("expected up to date, got available=true")
	}
}

func TestFindMatchingAsset(t *testing.T) {
	rel := &ReleaseInfo{
		Assets: []AssetInfo{
			{Name: "cmm-v0.2.0-linux-arm64", BrowserDownloadURL: "http://example.com/arm64"},
			{Name: "cmm-v0.2.0-linux-amd64", BrowserDownloadURL: "http://example.com/amd64"},
			{Name: "cmm-v0.2.0-windows-amd64.exe", BrowserDownloadURL: "http://example.com/win"},
		},
	}

	asset, err := FindMatchingAsset(rel, "linux", "amd64")
	if err != nil {
		t.Fatalf("failed to find asset: %v", err)
	}
	if asset.Name != "cmm-v0.2.0-linux-amd64" {
		t.Errorf("expected cmm-v0.2.0-linux-amd64, got %s", asset.Name)
	}

	winAsset, err := FindMatchingAsset(rel, "windows", "amd64")
	if err != nil {
		t.Fatalf("failed to find windows asset: %v", err)
	}
	if winAsset.Name != "cmm-v0.2.0-windows-amd64.exe" {
		t.Errorf("expected windows asset, got %s", winAsset.Name)
	}

	_, err = FindMatchingAsset(rel, "darwin", "arm64")
	if err == nil {
		t.Errorf("expected error for missing darwin asset")
	}
}

func TestFindMatchingAsset_ExcludeChecksumsAndArchives(t *testing.T) {
	rel := &ReleaseInfo{
		Assets: []AssetInfo{
			{Name: "cmm-v0.2.0-linux-amd64.sha256", BrowserDownloadURL: "http://example.com/sha256"},
			{Name: "cmm-v0.2.0-linux-amd64.sha512", BrowserDownloadURL: "http://example.com/sha512"},
			{Name: "cmm-v0.2.0-linux-amd64.md5", BrowserDownloadURL: "http://example.com/md5"},
			{Name: "cmm-v0.2.0-linux-amd64.txt", BrowserDownloadURL: "http://example.com/txt"},
			{Name: "cmm-v0.2.0-linux-amd64.tar.gz", BrowserDownloadURL: "http://example.com/targz"},
			{Name: "cmm-v0.2.0-linux-amd64.zip", BrowserDownloadURL: "http://example.com/zip"},
			{Name: "cmm-v0.2.0-linux-amd64", BrowserDownloadURL: "http://example.com/bin"},
		},
	}

	asset, err := FindMatchingAsset(rel, "linux", "amd64")
	if err != nil {
		t.Fatalf("FindMatchingAsset failed: %v", err)
	}
	if asset.Name != "cmm-v0.2.0-linux-amd64" {
		t.Errorf("expected cmm-v0.2.0-linux-amd64, got %s (did not exclude checksum/archive)", asset.Name)
	}
}

func TestCheckUpdate_RateLimitForbidden403(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer server.Close()

	os.Setenv("CMM_UPDATE_URL", server.URL)
	defer os.Unsetenv("CMM_UPDATE_URL")

	_, _, err := CheckUpdate("aegeada/cloudModManager", "0.1.0")
	if err == nil {
		t.Fatalf("expected error on HTTP 403 rate limit, got nil")
	}
}
