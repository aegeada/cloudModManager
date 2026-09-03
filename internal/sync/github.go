package sync

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"cmm/internal/config"
	"cmm/internal/modrinth"
)

type GitHubSynchronizer struct {
	Client     *modrinth.Client
	ConfigPath string
	LockPath   string
}

func NewGitHubSynchronizer(client *modrinth.Client, configPath, lockPath string) *GitHubSynchronizer {
	if configPath == "" {
		configPath = "cmm.toml"
	}
	if lockPath == "" {
		lockPath = "cmm.lock"
	}
	return &GitHubSynchronizer{
		Client:     client,
		ConfigPath: configPath,
		LockPath:   lockPath,
	}
}

func (s *GitHubSynchronizer) Sync(opts GitHubSyncOptions) (*SyncResult, error) {
	if opts.Repo == "" {
		return nil, fmt.Errorf("--repo <owner/repo> is required for github sync")
	}

	branch := opts.Branch
	if branch == "" {
		branch = "main"
	}

	cfg, err := config.LoadConfig(s.ConfigPath)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	token := opts.Token
	if token == "" && cfg.SyncToken != "" {
		token = cfg.SyncToken
	}
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	// 1. Resolve Target URL
	var targetURL string
	if ghBase := os.Getenv("GITHUB_API_URL"); ghBase != "" {
		targetURL = fmt.Sprintf("%s/repos/%s/contents/cmm.lock?ref=%s", strings.TrimSuffix(ghBase, "/"), opts.Repo, branch)
	} else if mockBase := os.Getenv("MODRINTH_API_URL"); mockBase != "" {
		baseURL := strings.TrimSuffix(mockBase, "/v2")
		baseURL = strings.TrimSuffix(baseURL, "/")
		targetURL = fmt.Sprintf("%s/github/repos/%s/contents/cmm.lock?ref=%s", baseURL, opts.Repo, branch)
	} else {
		targetURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/cmm.lock", opts.Repo, branch)
	}

	// 2. Fetch Remote Lockfile
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("User-Agent", "CloudModManager/1.0 (contact: user@domain.local)")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("GitHub repository '%s' or lockfile not found (404)", opts.Repo)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("GitHub authentication failed (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub error (HTTP %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote lockfile: %w", err)
	}

	// 3. Parse Remote Lockfile (using dual TOML parser)
	remoteLock, err := config.ParseLockfile(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse remote lockfile: %w", err)
	}

	// 4. Delta Synchronization
	localLock, _ := config.LoadLockfile(s.LockPath)
	modsDir := cfg.Paths.ModsDir
	if modsDir == "" {
		modsDir = "mods"
	}
	engine := NewDeltaEngine(s.Client, cfg, localLock, modsDir, s.LockPath)
	return engine.ApplyRemoteLockfile(remoteLock)
}
