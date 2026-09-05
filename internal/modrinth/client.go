package modrinth

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Client struct {
	BaseURL    string
	UserAgent  string
	Token      string
	HTTPClient *http.Client
}

// NewClient creates a new Modrinth API client.
func NewClient(userAgent string) (*Client, error) {
	if userAgent == "" {
		return nil, fmt.Errorf("User-Agent is required by Modrinth API")
	}

	baseURL := os.Getenv("MODRINTH_API_URL")
	if baseURL == "" {
		baseURL = "https://api.modrinth.com/v2"
	}

	return &Client{
		BaseURL:   baseURL,
		UserAgent: userAgent,
		HTTPClient: &http.Client{
			Transport: NewRateLimitTransport(nil),
		},
	}, nil
}

// NewClientWithToken creates a new client with authentication token and custom base URL if provided.
func NewClientWithToken(baseURL, userAgent, token string) (*Client, error) {
	if userAgent == "" {
		return nil, fmt.Errorf("User-Agent is required by Modrinth API")
	}

	if baseURL == "" {
		baseURL = os.Getenv("MODRINTH_API_URL")
		if baseURL == "" {
			baseURL = "https://api.modrinth.com/v2"
		}
	}

	return &Client{
		BaseURL:   baseURL,
		UserAgent: userAgent,
		Token:     token,
		HTTPClient: &http.Client{
			Transport: NewRateLimitTransport(nil),
		},
	}, nil
}

func (c *Client) newRequest(method, endpoint string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, c.BaseURL+endpoint, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", c.UserAgent)
	if c.Token != "" {
		req.Header.Set("Authorization", c.Token)
	}
	if mockErr := os.Getenv("MOCK_SEARCH_ERROR"); mockErr != "" {
		req.Header.Set("X-Mock-Error", mockErr)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *Client) doRequest(req *http.Request, target interface{}) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Modrinth API error (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}

	if target != nil {
		return json.NewDecoder(resp.Body).Decode(target)
	}
	return nil
}

// Search searches projects on Modrinth with simple query.
func (c *Client) Search(query string) (*SearchResponse, error) {
	return c.SearchWithOptions(query, nil, "relevance", 0, 10)
}

// SearchWithOptions searches projects with facets, index sorting, offset, and limit.
func (c *Client) SearchWithOptions(query string, facets [][]string, index string, offset, limit int) (*SearchResponse, error) {
	q := url.Values{}
	if query != "" {
		q.Set("query", query)
	}
	if index != "" {
		q.Set("index", index)
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if len(facets) > 0 {
		facetsJSON, err := json.Marshal(facets)
		if err == nil {
			q.Set("facets", string(facetsJSON))
		}
	}

	req, err := c.newRequest("GET", "/search?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var result SearchResponse
	if err := c.doRequest(req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetProject retrieves a single project by ID or slug.
func (c *Client) GetProject(idOrSlug string) (*Project, error) {
	req, err := c.newRequest("GET", "/project/"+url.PathEscape(idOrSlug), nil)
	if err != nil {
		return nil, err
	}

	var project Project
	if err := c.doRequest(req, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// GetProjectVersions retrieves versions for a project with optional filters.
func (c *Client) GetProjectVersions(idOrSlug string, loaders []string, gameVersions []string, featured *bool) ([]Version, error) {
	q := url.Values{}
	if len(loaders) > 0 {
		loadersJSON, _ := json.Marshal(loaders)
		q.Set("loaders", string(loadersJSON))
	}
	if len(gameVersions) > 0 {
		gameVersionsJSON, _ := json.Marshal(gameVersions)
		q.Set("game_versions", string(gameVersionsJSON))
	}
	if featured != nil {
		q.Set("featured", strconv.FormatBool(*featured))
	}

	endpoint := "/project/" + url.PathEscape(idOrSlug) + "/version"
	if len(q) > 0 {
		endpoint += "?" + q.Encode()
	}

	req, err := c.newRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var versions []Version
	if err := c.doRequest(req, &versions); err != nil {
		return nil, err
	}
	return versions, nil
}

// GetVersion retrieves a single version by ID.
func (c *Client) GetVersion(versionID string) (*Version, error) {
	req, err := c.newRequest("GET", "/version/"+url.PathEscape(versionID), nil)
	if err != nil {
		return nil, err
	}

	var version Version
	if err := c.doRequest(req, &version); err != nil {
		return nil, err
	}
	return &version, nil
}

// GetVersionFromHash retrieves version details from a file hash (sha1 or sha512).
func (c *Client) GetVersionFromHash(hash string, algorithm string) (*Version, error) {
	if algorithm == "" {
		algorithm = "sha512"
	}
	q := url.Values{}
	q.Set("algorithm", algorithm)

	req, err := c.newRequest("GET", "/version_file/"+url.PathEscape(hash)+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var version Version
	if err := c.doRequest(req, &version); err != nil {
		return nil, err
	}
	return &version, nil
}

// GetMultipleVersionsFromHashes batches hash lookups.
func (c *Client) GetMultipleVersionsFromHashes(hashes []string, algorithm string) (map[string]Version, error) {
	if algorithm == "" {
		algorithm = "sha512"
	}

	reqBody := struct {
		Hashes    []string `json:"hashes"`
		Algorithm string   `json:"algorithm"`
	}{
		Hashes:    hashes,
		Algorithm: algorithm,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := c.newRequest("POST", "/version_files", bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, err
	}

	var result map[string]Version
	if err := c.doRequest(req, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetLoaderTags retrieves the list of loader tags.
func (c *Client) GetLoaderTags() ([]LoaderTag, error) {
	req, err := c.newRequest("GET", "/tag/loader", nil)
	if err != nil {
		return nil, err
	}

	var loaders []LoaderTag
	if err := c.doRequest(req, &loaders); err != nil {
		return nil, err
	}
	return loaders, nil
}

// GetGameVersionTags retrieves the list of game version tags.
func (c *Client) GetGameVersionTags() ([]GameVersionTag, error) {
	req, err := c.newRequest("GET", "/tag/game_version", nil)
	if err != nil {
		return nil, err
	}

	var gameVersions []GameVersionTag
	if err := c.doRequest(req, &gameVersions); err != nil {
		return nil, err
	}
	return gameVersions, nil
}

// DownloadFile downloads a file from URL to destPath, verifying sha512 if provided.
// Downloads to a temporary staging file (.tmp) and verifies SHA-512 before renaming to destPath.
func (c *Client) DownloadFile(urlStr, destPath string, expectedSha512 string) error {
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file (HTTP %d)", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp(destDir, "."+filepath.Base(destPath)+"-*.tmp")
	if err != nil {
		tmpPath := destPath + ".tmp"
		tmpFile, err = os.Create(tmpPath)
		if err != nil {
			return err
		}
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	sha512Hasher := sha512.New()
	sha1Hasher := sha1.New()
	multiWriter := io.MultiWriter(tmpFile, sha512Hasher, sha1Hasher)

	if _, err := io.Copy(multiWriter, resp.Body); err != nil {
		tmpFile.Close()
		return err
	}

	if expectedSha512 != "" {
		calculatedHash := hex.EncodeToString(sha512Hasher.Sum(nil))
		if !strings.EqualFold(calculatedHash, expectedSha512) {
			tmpFile.Close()
			return fmt.Errorf("SHA-512 checksum mismatch: expected %s, got %s", expectedSha512, calculatedHash)
		}
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		return err
	}
	tmpPath = ""

	return nil
}

// FilterVersionsByChannel filters versions according to the stability channel.
// "release": only "release" versions
// "beta": "release" and "beta" versions
// "alpha": all versions ("release", "beta", "alpha")
func FilterVersionsByChannel(versions []Version, channel string) []Version {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" || channel == "release" {
		var filtered []Version
		for _, v := range versions {
			if v.VersionType == "" || strings.EqualFold(v.VersionType, "release") {
				filtered = append(filtered, v)
			}
		}
		return filtered
	}
	if channel == "beta" {
		var filtered []Version
		for _, v := range versions {
			if v.VersionType == "" || strings.EqualFold(v.VersionType, "release") || strings.EqualFold(v.VersionType, "beta") {
				filtered = append(filtered, v)
			}
		}
		return filtered
	}
	return versions
}
