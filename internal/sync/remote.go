package sync

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cmm/internal/config"
	"cmm/internal/modrinth"
)

type RemoteSynchronizer struct {
	Client     *modrinth.Client
	ConfigPath string
	LockPath   string
}

func NewRemoteSynchronizer(client *modrinth.Client, configPath, lockPath string) *RemoteSynchronizer {
	if configPath == "" {
		configPath = "cmm.toml"
	}
	if lockPath == "" {
		lockPath = "cmm.lock"
	}
	return &RemoteSynchronizer{
		Client:     client,
		ConfigPath: configPath,
		LockPath:   lockPath,
	}
}

func (s *RemoteSynchronizer) Sync(opts RemoteSyncOptions) (*SyncResult, error) {
	if opts.URL == "" {
		return nil, fmt.Errorf("--url <http://server:port> is required for remote sync")
	}

	urlStr := strings.TrimRight(opts.URL, "/")
	if !strings.HasSuffix(urlStr, "/lock") {
		urlStr = urlStr + "/lock"
	}

	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	token := opts.Token
	if token == "" && cfg.SyncToken != "" {
		token = cfg.SyncToken
	}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("User-Agent", "CloudModManager/1.0 (contact: user@domain.local)")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remote server unreachable at %s: %w", opts.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized: invalid or missing sync token (HTTP 401)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote server returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read lockfile from remote server: %w", err)
	}

	remoteLock, err := config.ParseLockfile(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse lockfile from remote server: %w", err)
	}

	localLock, _ := config.LoadLockfile(s.LockPath)
	modsDir := cfg.Paths.ModsDir
	if modsDir == "" {
		modsDir = "mods"
	}
	engine := NewDeltaEngine(s.Client, cfg, localLock, modsDir, s.LockPath)
	return engine.ApplyRemoteLockfile(remoteLock)
}
