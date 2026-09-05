package sync

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"cmm/internal/config"
)

// DiffCategory defines the classification category of a mod difference.
type DiffCategory string

const (
	DiffOK       DiffCategory = "OK"
	DiffMismatch DiffCategory = "MISMATCH"
	DiffClient   DiffCategory = "CLIENT"
	DiffServer   DiffCategory = "SERVER"
	DiffMissing  DiffCategory = "MISSING"
)

// ModDiffEntry represents the difference status for a single mod.
type ModDiffEntry struct {
	Slug           string       `json:"slug"`
	Name           string       `json:"name"`
	Category       DiffCategory `json:"category"`
	Status         string       `json:"status"` // "[OK]", "[MISMATCH]", "[CLIENT]", "[SERVER]", "[MISSING]"
	LocalVersion   string       `json:"local_version,omitempty"`
	RemoteVersion  string       `json:"remote_version,omitempty"`
	LocalSide      string       `json:"local_side,omitempty"`
	RemoteSide     string       `json:"remote_side,omitempty"`
	LocalDisabled  bool         `json:"local_disabled,omitempty"`
	RemoteDisabled bool         `json:"remote_disabled,omitempty"`
	MissingOn      string       `json:"missing_on,omitempty"` // "server" or "client"
	Notes          string       `json:"notes,omitempty"`
}

// DiffResult captures the full comparison summary and mod-by-mod entries.
type DiffResult struct {
	Target       string         `json:"target,omitempty"`
	Total        int            `json:"total"`
	Synchronized int            `json:"synchronized"`
	Mismatches   int            `json:"mismatches"`
	ClientOnly   int            `json:"client_only"`
	ServerOnly   int            `json:"server_only"`
	Missing      int            `json:"missing"`
	Entries      []ModDiffEntry `json:"entries"`
}

// DiffEngine performs lockfile comparisons against local files and remote CMM daemons.
type DiffEngine struct {
	HTTPClient *http.Client
}

// NewDiffEngine creates a new DiffEngine with sensible HTTP defaults.
func NewDiffEngine() *DiffEngine {
	return &DiffEngine{
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// canonicalModSlug normalizes slug, filename, project ID, or name into a canonical slug string.
func canonicalModSlug(m config.LockfileMod) string {
	if m.Slug != "" {
		extracted := config.ExtractModSlug(m.Slug)
		if extracted != "" {
			return extracted
		}
		return strings.ToLower(strings.TrimSpace(m.Slug))
	}
	if m.FileName != "" {
		extracted := config.ExtractModSlug(m.FileName)
		if extracted != "" {
			return extracted
		}
	}
	if m.ProjectID != "" {
		return strings.ToLower(strings.TrimSpace(m.ProjectID))
	}
	if m.Name != "" {
		extracted := config.ExtractModSlug(m.Name)
		if extracted != "" {
			return extracted
		}
		return strings.ToLower(strings.TrimSpace(m.Name))
	}
	return ""
}

// modsMatch checks whether two LockfileMod instances represent the same mod.
func modsMatch(m1, m2 config.LockfileMod) bool {
	if m1.ProjectID != "" && m2.ProjectID != "" && strings.EqualFold(m1.ProjectID, m2.ProjectID) {
		return true
	}
	if m1.Slug != "" && m2.Slug != "" && strings.EqualFold(m1.Slug, m2.Slug) {
		return true
	}
	slug1 := canonicalModSlug(m1)
	slug2 := canonicalModSlug(m2)
	if slug1 != "" && slug2 != "" && strings.EqualFold(slug1, slug2) {
		return true
	}
	if m1.Slug != "" {
		if strings.EqualFold(m1.Slug, slug2) ||
			(m2.FileName != "" && strings.EqualFold(m1.Slug, m2.FileName)) ||
			(m2.Name != "" && strings.EqualFold(m1.Slug, m2.Name)) {
			return true
		}
	}
	if m2.Slug != "" {
		if strings.EqualFold(m2.Slug, slug1) ||
			(m1.FileName != "" && strings.EqualFold(m2.Slug, m1.FileName)) ||
			(m1.Name != "" && strings.EqualFold(m2.Slug, m1.Name)) {
			return true
		}
	}
	if m1.Name != "" && m2.Name != "" && strings.EqualFold(m1.Name, m2.Name) {
		return true
	}
	return false
}

// bestModSlug derives the most readable slug for diff reporting.
func bestModSlug(m1, m2 config.LockfileMod) string {
	if m1.Slug != "" {
		return m1.Slug
	}
	if m2.Slug != "" {
		return m2.Slug
	}
	s1 := canonicalModSlug(m1)
	if s1 != "" {
		return s1
	}
	s2 := canonicalModSlug(m2)
	if s2 != "" {
		return s2
	}
	if m1.Name != "" {
		return strings.ToLower(m1.Name)
	}
	if m2.Name != "" {
		return strings.ToLower(m2.Name)
	}
	return "unknown-mod"
}

// bestModName derives the most readable display name for diff reporting.
func bestModName(m1, m2 config.LockfileMod) string {
	if m1.Name != "" {
		return m1.Name
	}
	if m2.Name != "" {
		return m2.Name
	}
	if m1.Slug != "" {
		return m1.Slug
	}
	if m2.Slug != "" {
		return m2.Slug
	}
	return "Unknown Mod"
}

// CompareLockfiles compares two lockfiles and produces a categorized DiffResult.
func (e *DiffEngine) CompareLockfiles(leftLock, rightLock *config.Lockfile) *DiffResult {
	res := &DiffResult{
		Entries: []ModDiffEntry{},
	}

	var leftMods []config.LockfileMod
	if leftLock != nil {
		leftMods = leftLock.Mods
	}
	var rightMods []config.LockfileMod
	if rightLock != nil {
		rightMods = rightLock.Mods
	}

	matchedRight := make(map[int]bool)

	// 1. Process left (local) mods
	for _, l := range leftMods {
		matchedIndex := -1
		for j, r := range rightMods {
			if !matchedRight[j] && modsMatch(l, r) {
				matchedIndex = j
				break
			}
		}

		if matchedIndex >= 0 {
			matchedRight[matchedIndex] = true
			r := rightMods[matchedIndex]

			lVer := l.GetVersion()
			rVer := r.GetVersion()
			slug := bestModSlug(l, r)
			name := bestModName(l, r)

			var entry ModDiffEntry
			entry.Slug = slug
			entry.Name = name
			entry.LocalVersion = lVer
			entry.RemoteVersion = rVer
			entry.LocalSide = config.NormalizeSide(l.Side)
			entry.RemoteSide = config.NormalizeSide(r.Side)
			entry.LocalDisabled = l.Disabled
			entry.RemoteDisabled = r.Disabled

			if lVer != rVer && !(lVer == "" && rVer == "") {
				entry.Category = DiffMismatch
				entry.Status = "[MISMATCH]"
				entry.Notes = fmt.Sprintf("Version mismatch (client: %s vs server: %s)", lVer, rVer)
				res.Mismatches++
			} else if l.Disabled != r.Disabled {
				entry.Category = DiffMismatch
				entry.Status = "[MISMATCH]"
				if l.Disabled {
					entry.Notes = "Status mismatch: disabled locally, enabled remotely"
				} else {
					entry.Notes = "Status mismatch: enabled locally, disabled remotely"
				}
				res.Mismatches++
			} else if l.SHA512 != "" && r.SHA512 != "" && !strings.EqualFold(l.SHA512, r.SHA512) {
				entry.Category = DiffMismatch
				entry.Status = "[MISMATCH]"
				entry.Notes = "Checksum mismatch: SHA-512 hashes differ"
				res.Mismatches++
			} else {
				entry.Category = DiffOK
				entry.Status = "[OK]"
				if l.Disabled {
					entry.Notes = "Synchronized (disabled)"
				} else {
					entry.Notes = "Synchronized"
				}
				res.Synchronized++
			}
			res.Entries = append(res.Entries, entry)
		} else {
			// Mod is on left (local) ONLY
			slug := bestModSlug(l, config.LockfileMod{})
			name := bestModName(l, config.LockfileMod{})
			lVer := l.GetVersion()

			var entry ModDiffEntry
			entry.Slug = slug
			entry.Name = name
			entry.LocalVersion = lVer
			entry.LocalSide = config.NormalizeSide(l.Side)
			entry.LocalDisabled = l.Disabled

			normSide := strings.ToLower(config.NormalizeSide(l.Side))
			if strings.HasPrefix(normSide, "client") && !strings.Contains(normSide, "client & server") && !strings.Contains(normSide, "both") {
				entry.Category = DiffClient
				entry.Status = "[CLIENT]"
				entry.Notes = "Client-only mod (omitted on server)"
				res.ClientOnly++
			} else {
				entry.Category = DiffMissing
				entry.Status = "[MISSING]"
				entry.MissingOn = "server"
				entry.Notes = "Missing on server"
				res.Missing++
			}
			res.Entries = append(res.Entries, entry)
		}
	}

	// 2. Process remaining right (remote) mods
	for j, r := range rightMods {
		if matchedRight[j] {
			continue
		}

		slug := bestModSlug(r, config.LockfileMod{})
		name := bestModName(r, config.LockfileMod{})
		rVer := r.GetVersion()

		var entry ModDiffEntry
		entry.Slug = slug
		entry.Name = name
		entry.RemoteVersion = rVer
		entry.RemoteSide = config.NormalizeSide(r.Side)
		entry.RemoteDisabled = r.Disabled

		normSide := strings.ToLower(config.NormalizeSide(r.Side))
		if strings.HasPrefix(normSide, "server") && !strings.Contains(normSide, "client & server") && !strings.Contains(normSide, "both") {
			entry.Category = DiffServer
			entry.Status = "[SERVER]"
			entry.Notes = "Server-only mod (omitted on client)"
			res.ServerOnly++
		} else {
			entry.Category = DiffMissing
			entry.Status = "[MISSING]"
			entry.MissingOn = "client"
			entry.Notes = "Missing on client"
			res.Missing++
		}
		res.Entries = append(res.Entries, entry)
	}

	res.Total = len(res.Entries)

	// Sort entries deterministically by slug (case-insensitive)
	sort.Slice(res.Entries, func(i, j int) bool {
		s1 := strings.ToLower(res.Entries[i].Slug)
		s2 := strings.ToLower(res.Entries[j].Slug)
		if s1 == s2 {
			return strings.ToLower(res.Entries[i].Name) < strings.ToLower(res.Entries[j].Name)
		}
		return s1 < s2
	})

	return res
}

// CompareRemote fetches a remote server's lockfile via HTTP GET /lock and diffs against a local lockfile.
func (e *DiffEngine) CompareRemote(localLockPath, remoteURL, token string) (*DiffResult, error) {
	if localLockPath == "" {
		localLockPath = "cmm.lock"
	}

	var localLock *config.Lockfile
	if _, err := os.Stat(localLockPath); err == nil {
		var errLoad error
		localLock, errLoad = config.LoadLockfile(localLockPath)
		if errLoad != nil {
			return nil, fmt.Errorf("failed to load local lockfile '%s': %w", localLockPath, errLoad)
		}
	} else {
		localLock = &config.Lockfile{Mods: []config.LockfileMod{}}
	}

	url := strings.TrimSpace(remoteURL)
	if url == "" {
		return nil, fmt.Errorf("remote URL cannot be empty")
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "http://" + url
	}
	url = strings.TrimRight(url, "/")

	reqURL := url + "/lock"
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	httpClient := e.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to remote server '%s': %w", reqURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized: invalid or missing authentication token for %s", url)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("remote server returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote response: %w", err)
	}

	remoteLock, err := config.ParseLockfile(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse remote lockfile: %w", err)
	}

	res := e.CompareLockfiles(localLock, remoteLock)
	res.Target = url
	return res, nil
}

// CompareFiles loads two lockfiles by path and compares them.
func (e *DiffEngine) CompareFiles(leftPath, rightPath string) (*DiffResult, error) {
	leftLock, err := config.LoadLockfile(leftPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load left lockfile '%s': %w", leftPath, err)
	}
	rightLock, err := config.LoadLockfile(rightPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load right lockfile '%s': %w", rightPath, err)
	}

	res := e.CompareLockfiles(leftLock, rightLock)
	res.Target = fmt.Sprintf("%s vs %s", leftPath, rightPath)
	return res, nil
}

// CompareLockfiles is a package-level helper that invokes DiffEngine.CompareLockfiles.
func CompareLockfiles(leftLock, rightLock *config.Lockfile) *DiffResult {
	return NewDiffEngine().CompareLockfiles(leftLock, rightLock)
}

// CompareRemote is a package-level helper that invokes DiffEngine.CompareRemote.
func CompareRemote(localLockPath, remoteURL, token string) (*DiffResult, error) {
	return NewDiffEngine().CompareRemote(localLockPath, remoteURL, token)
}

// CompareFiles is a package-level helper that invokes DiffEngine.CompareFiles.
func CompareFiles(leftPath, rightPath string) (*DiffResult, error) {
	return NewDiffEngine().CompareFiles(leftPath, rightPath)
}

// ToJSON formats DiffResult as an indented JSON string.
func (r *DiffResult) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatJSON writes formatted JSON to the specified writer.
func FormatJSON(w io.Writer, res *DiffResult) error {
	data, err := res.ToJSON()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, data)
	return err
}

// RenderTable writes aligned tabwriter output with summary footer to the specified writer.
func (r *DiffResult) RenderTable(w io.Writer) {
	if len(r.Entries) == 0 {
		fmt.Fprintln(w, "No mods to compare or lockfiles are empty.")
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "STATUS\tNAME\tSLUG\tLOCAL\tREMOTE\tNOTES")
	for _, entry := range r.Entries {
		locVer := entry.LocalVersion
		if locVer == "" {
			locVer = "-"
		}
		remVer := entry.RemoteVersion
		if remVer == "" {
			remVer = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			entry.Status,
			entry.Name,
			entry.Slug,
			locVer,
			remVer,
			entry.Notes,
		)
	}
	tw.Flush()

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Summary: %d total mods | %d synchronized | %d mismatches | %d client-only | %d server-only | %d missing\n",
		r.Total, r.Synchronized, r.Mismatches, r.ClientOnly, r.ServerOnly, r.Missing)
}

// FormatTable is a package-level helper that invokes DiffResult.RenderTable.
func FormatTable(w io.Writer, res *DiffResult) {
	res.RenderTable(w)
}

// TableString returns the table formatted as a string.
func (r *DiffResult) TableString() string {
	var buf strings.Builder
	r.RenderTable(&buf)
	return buf.String()
}
