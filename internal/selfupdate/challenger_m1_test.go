package selfupdate

import (
	"testing"
)

func TestChallenger_FindMatchingAsset_IgnoresChecksumsAndArchives(t *testing.T) {
	release := &ReleaseInfo{
		TagName: "v1.2.0",
		Assets: []AssetInfo{
			{Name: "cmm-linux-amd64.sha256", BrowserDownloadURL: "https://example.com/cmm-linux-amd64.sha256", Size: 64},
			{Name: "cmm-linux-amd64.sha512", BrowserDownloadURL: "https://example.com/cmm-linux-amd64.sha512", Size: 128},
			{Name: "cmm-linux-amd64.md5", BrowserDownloadURL: "https://example.com/cmm-linux-amd64.md5", Size: 32},
			{Name: "cmm-linux-amd64.txt", BrowserDownloadURL: "https://example.com/cmm-linux-amd64.txt", Size: 100},
			{Name: "cmm-linux-amd64.tar.gz", BrowserDownloadURL: "https://example.com/cmm-linux-amd64.tar.gz", Size: 10000000},
			{Name: "cmm-linux-amd64.zip", BrowserDownloadURL: "https://example.com/cmm-linux-amd64.zip", Size: 10000000},
			{Name: "cmm-windows-amd64.zip", BrowserDownloadURL: "https://example.com/cmm-windows-amd64.zip", Size: 10000000},
			{Name: "cmm-windows-amd64.exe.sha256", BrowserDownloadURL: "https://example.com/cmm-windows-amd64.exe.sha256", Size: 64},
			{Name: "cmm-linux-amd64", BrowserDownloadURL: "https://example.com/cmm-linux-amd64", Size: 15000000},
			{Name: "cmm-windows-amd64.exe", BrowserDownloadURL: "https://example.com/cmm-windows-amd64.exe", Size: 16000000},
		},
	}

	// 1. Verify linux/amd64 selects binary, not checksums or archives
	assetLinux, err := FindMatchingAsset(release, "linux", "amd64")
	if err != nil {
		t.Fatalf("FindMatchingAsset(linux, amd64) failed: %v", err)
	}
	if assetLinux.Name != "cmm-linux-amd64" {
		t.Errorf("expected 'cmm-linux-amd64', got %q", assetLinux.Name)
	}

	// 2. Verify windows/amd64 selects .exe, not .zip or .sha256
	assetWin, err := FindMatchingAsset(release, "windows", "amd64")
	if err != nil {
		t.Fatalf("FindMatchingAsset(windows, amd64) failed: %v", err)
	}
	if assetWin.Name != "cmm-windows-amd64.exe" {
		t.Errorf("expected 'cmm-windows-amd64.exe', got %q", assetWin.Name)
	}

	// 3. Verify only-archive release fails with error
	releaseOnlyArchives := &ReleaseInfo{
		TagName: "v1.2.0",
		Assets: []AssetInfo{
			{Name: "cmm-linux-amd64.tar.gz", BrowserDownloadURL: "https://example.com/cmm-linux-amd64.tar.gz", Size: 10000000},
			{Name: "cmm-linux-amd64.sha256", BrowserDownloadURL: "https://example.com/cmm-linux-amd64.sha256", Size: 64},
			{Name: "cmm-linux-amd64.zip", BrowserDownloadURL: "https://example.com/cmm-linux-amd64.zip", Size: 10000000},
		},
	}
	_, err = FindMatchingAsset(releaseOnlyArchives, "linux", "amd64")
	if err == nil {
		t.Errorf("expected error when only archives/checksums exist, got nil")
	}
}
