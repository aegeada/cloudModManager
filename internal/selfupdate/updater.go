package selfupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"cmm/internal/mod"
)

const DefaultRepo = "aegeada/cloudModManager"

// AssetInfo describes a downloadable binary asset.
type AssetInfo struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// ReleaseInfo holds details about a GitHub release.
type ReleaseInfo struct {
	TagName     string      `json:"tag_name"`
	Name        string      `json:"name"`
	Body        string      `json:"body"`
	PublishedAt time.Time   `json:"published_at"`
	HTMLURL     string      `json:"html_url"`
	Assets      []AssetInfo `json:"assets"`
}

// CheckUpdate checks GitHub for a newer version than currentVersion.
func CheckUpdate(repo string, currentVersion string) (*ReleaseInfo, bool, error) {
	if repo == "" {
		repo = DefaultRepo
	}

	apiURL := os.Getenv("CMM_UPDATE_URL")
	if apiURL == "" {
		ghAPI := os.Getenv("GITHUB_API_URL")
		if ghAPI == "" {
			ghAPI = "https://api.github.com"
		}
		apiURL = strings.TrimSuffix(ghAPI, "/") + "/repos/" + repo + "/releases/latest"
	}

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", "CloudModManager-Updater/"+currentVersion)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("network request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, fmt.Errorf("no releases found for %s", repo)
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, false, fmt.Errorf("GitHub API rate limit exceeded or access denied (HTTP 403)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, false, fmt.Errorf("failed to decode release info: %w", err)
	}

	remoteVer := strings.TrimPrefix(release.TagName, "v")
	curVer := strings.TrimPrefix(currentVersion, "v")

	isNewer := mod.IsNewerVersion(remoteVer, curVer)
	return &release, isNewer, nil
}

// FindMatchingAsset finds the binary asset matching current OS and architecture.
func FindMatchingAsset(release *ReleaseInfo, goos, goarch string) (*AssetInfo, error) {
	if release == nil || len(release.Assets) == 0 {
		return nil, fmt.Errorf("release has no downloadable assets")
	}

	isWindows := goos == "windows"

	for _, a := range release.Assets {
		name := strings.ToLower(a.Name)
		// Exclude checksums, archives, and non-binaries
		if strings.HasSuffix(name, ".sha256") || strings.HasSuffix(name, ".sha512") ||
			strings.HasSuffix(name, ".md5") || strings.HasSuffix(name, ".txt") ||
			strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".zip") {
			continue
		}

		if !strings.Contains(name, goos) || !strings.Contains(name, goarch) {
			continue
		}

		if isWindows && !strings.HasSuffix(name, ".exe") {
			continue
		}
		if !isWindows && strings.HasSuffix(name, ".exe") {
			continue
		}

		return &a, nil
	}

	return nil, fmt.Errorf("no compatible binary asset found for %s/%s", goos, goarch)
}

// ApplyUpdate downloads the matching binary and replaces the running executable.
func ApplyUpdate(release *ReleaseInfo) error {
	asset, err := FindMatchingAsset(release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve current executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable symlinks: %w", err)
	}

	execDir := filepath.Dir(execPath)

	// Download asset to a temporary file in the same directory (enabling atomic rename across same mount)
	tmpFile, err := os.CreateTemp(execDir, "cmm-update-*")
	if err != nil {
		// Fallback to os.TempDir if directory is not directly writable
		tmpFile, err = os.CreateTemp("", "cmm-update-*")
		if err != nil {
			return fmt.Errorf("failed to create temporary file for update: %w", err)
		}
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", asset.BrowserDownloadURL, nil)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", "CloudModManager-Updater")

	resp, err := client.Do(req)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to download update binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("download failed with HTTP status %d", resp.StatusCode)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed writing downloaded binary: %w", err)
	}
	tmpFile.Close()

	fi, err := os.Stat(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to stat downloaded binary: %w", err)
	}
	if fi.Size() == 0 {
		return fmt.Errorf("downloaded binary is empty (0 bytes)")
	}
	if asset.Size > 0 && fi.Size() != asset.Size {
		return fmt.Errorf("downloaded binary size mismatch: expected %d bytes, got %d bytes", asset.Size, fi.Size())
	}

	// Set executable permission
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed setting executable permissions: %w", err)
	}

	// Atomic binary replacement
	if runtime.GOOS == "windows" {
		oldPath := execPath + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(execPath, oldPath); err != nil {
			return fmt.Errorf("failed renaming current binary: %w", err)
		}
		if err := copyOrMove(tmpPath, execPath); err != nil {
			_ = os.Rename(oldPath, execPath) // Rollback
			return fmt.Errorf("failed replacing binary: %w", err)
		}
		_ = os.Remove(oldPath)
	} else {
		// Unix atomic rename
		if err := os.Rename(tmpPath, execPath); err != nil {
			// Cross-device fallback without ETXTBSY:
			// In Linux, truncating an open executable triggers ETXTBSY.
			// Renaming or unlinking the running executable first avoids ETXTBSY.
			oldPath := execPath + ".old"
			_ = os.Remove(oldPath)
			if renameErr := os.Rename(execPath, oldPath); renameErr == nil {
				if err := copyOrMove(tmpPath, execPath); err != nil {
					_ = os.Rename(oldPath, execPath) // Rollback
					return fmt.Errorf("failed to install updated binary: %w", err)
				}
				_ = os.Remove(oldPath)
			} else {
				_ = os.Remove(execPath)
				if err := copyOrMove(tmpPath, execPath); err != nil {
					return fmt.Errorf("failed to install updated binary: %w", err)
				}
			}
		}
		_ = os.Chmod(execPath, 0755)
	}

	return nil
}

func copyOrMove(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
