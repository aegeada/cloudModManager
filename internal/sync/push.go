package sync

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cmm/internal/config"
)

// PushOptions specifies parameters for client-to-server push synchronization.
type PushOptions struct {
	URL           string
	Token         string
	IncludeConfig bool
	DryRun        bool
	ConfigPath    string
	LockPath      string
	ConfigDir     string
}

// PushResult captures the server response after a push synchronization.
type PushResult struct {
	Success        bool     `json:"success"`
	Message        string   `json:"message"`
	AddedMods      []string `json:"added_mods"`
	UpdatedMods    []string `json:"updated_mods"`
	PrunedMods     []string `json:"pruned_mods"`
	ConfigsUpdated int      `json:"configs_updated"`
}

// PushSynchronizer manages the client push workflow.
type PushSynchronizer struct {
	ConfigPath string
	LockPath   string
	ConfigDir  string
	HTTPClient *http.Client
}

// NewPushSynchronizer creates a new PushSynchronizer instance.
func NewPushSynchronizer(configPath, lockPath, configDir string) *PushSynchronizer {
	if configPath == "" {
		configPath = "cmm.toml"
	}
	if lockPath == "" {
		lockPath = "cmm.lock"
	}
	if configDir == "" {
		configDir = "config"
	}
	return &PushSynchronizer{
		ConfigPath: configPath,
		LockPath:   lockPath,
		ConfigDir:  configDir,
	}
}

// PackageConfigZip creates an in-memory zip archive of the specified configuration directory.
func PackageConfigZip(configDir string) ([]byte, error) {
	if configDir == "" {
		configDir = "config"
	}

	info, err := os.Stat(configDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read config directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("config path '%s' is not a directory", configDir)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	hasFiles := false

	err = filepath.Walk(configDir, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(configDir, path)
		if err != nil {
			return err
		}
		if relPath == "." || relPath == "" {
			return nil
		}

		zipEntryPath := filepath.ToSlash(relPath)
		if f.IsDir() {
			_, err = zw.Create(zipEntryPath + "/")
			return err
		}

		hasFiles = true
		w, err := zw.Create(zipEntryPath)
		if err != nil {
			return err
		}

		fileData, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		_, err = w.Write(fileData)
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("failed to package config directory: %w", err)
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize config zip: %w", err)
	}

	if !hasFiles {
		return nil, nil
	}

	return buf.Bytes(), nil
}

// Push packages local lockfile and config, sending them to the remote CMM server daemon.
func (s *PushSynchronizer) Push(opts PushOptions) (*PushResult, error) {
	if opts.URL == "" {
		return nil, fmt.Errorf("--url <http://server:port> is required for push")
	}

	urlStr := strings.TrimRight(opts.URL, "/")
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "http://" + urlStr
	}
	if !strings.HasSuffix(urlStr, "/push") {
		urlStr = urlStr + "/push"
	}

	cfgPath := opts.ConfigPath
	if cfgPath == "" {
		cfgPath = s.ConfigPath
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	token := opts.Token
	if token == "" && cfg != nil && cfg.SyncToken != "" {
		token = cfg.SyncToken
	}

	lockPath := opts.LockPath
	if lockPath == "" {
		lockPath = s.LockPath
	}

	lockBytes, err := os.ReadFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read lockfile '%s': %w", lockPath, err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add lockfile part
	lockPart, err := writer.CreateFormFile("lockfile", "cmm.lock")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file for lockfile: %w", err)
	}
	if _, err := lockPart.Write(lockBytes); err != nil {
		return nil, fmt.Errorf("failed to write lockfile to multipart form: %w", err)
	}

	// Add config part if requested
	if opts.IncludeConfig {
		configDir := opts.ConfigDir
		if configDir == "" {
			configDir = s.ConfigDir
		}
		if configDir == "" && cfg != nil && cfg.Paths.ConfigDir != "" {
			configDir = cfg.Paths.ConfigDir
		}
		zipBytes, err := PackageConfigZip(configDir)
		if err != nil {
			return nil, fmt.Errorf("failed to package config zip: %w", err)
		}
		if len(zipBytes) > 0 {
			configPart, err := writer.CreateFormFile("config", "config.zip")
			if err != nil {
				return nil, fmt.Errorf("failed to create form file for config: %w", err)
			}
			if _, err := configPart.Write(zipBytes); err != nil {
				return nil, fmt.Errorf("failed to write config zip to multipart form: %w", err)
			}
		}
	}

	// Add dry_run flag
	if opts.DryRun {
		_ = writer.WriteField("dry_run", "true")
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequest("POST", urlStr, &body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "CloudModManager/1.0 (contact: user@domain.local)")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := s.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: 60 * time.Second,
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remote server unreachable at %s: %w", opts.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized: invalid or missing sync token (HTTP 401)")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from remote server: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remote server returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result PushResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response from remote server: %w", err)
	}

	return &result, nil
}
