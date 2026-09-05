package sync

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	stdsync "sync"
	"syscall"
	"time"

	"cmm/internal/config"
	"cmm/internal/mod"
	"cmm/internal/modrinth"
)

// PushResponse represents the JSON response returned by the POST /push endpoint.
type PushResponse struct {
	Success        bool     `json:"success"`
	Message        string   `json:"message"`
	AddedMods      []string `json:"added_mods"`
	UpdatedMods    []string `json:"updated_mods"`
	PrunedMods     []string `json:"pruned_mods"`
	ConfigsUpdated int      `json:"configs_updated"`
}

// Server represents the CMM HTTP daemon.
type Server struct {
	Port       int
	Token      string
	LockPath   string
	ConfigPath string
	ModsDir    string
	ConfigDir  string
	Client     *modrinth.Client
	httpServer *http.Server
	mu         stdsync.Mutex
}

// NewServer creates a new CMM HTTP daemon instance.
func NewServer(port int, token string, lockPath string) *Server {
	if port <= 0 {
		port = 8080
	}
	if lockPath == "" {
		lockPath = "cmm.lock"
	}
	s := &Server{
		Port:       port,
		Token:      token,
		LockPath:   lockPath,
		ConfigPath: "cmm.toml",
		ModsDir:    "mods",
		ConfigDir:  "config",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/lock", s.HandleLock)
	mux.HandleFunc("/health", s.HandleHealth)
	mux.HandleFunc("/push", s.HandlePush)

	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return s
}

// HTTPServer returns the internal http.Server instance for inspection.
func (s *Server) HTTPServer() *http.Server {
	return s.httpServer
}

// HandleLock handles GET /lock requests to return the server lockfile.
func (s *Server) HandleLock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.Token != "" {
		authHeader := r.Header.Get("Authorization")
		var reqToken string
		if strings.HasPrefix(authHeader, "Bearer ") {
			reqToken = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			reqToken = authHeader
		}

		if reqToken == "" || subtle.ConstantTimeCompare([]byte(reqToken), []byte(s.Token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="cmm"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	data, err := os.ReadFile(s.LockPath)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("[mods]\n"))
			return
		}
		http.Error(w, fmt.Sprintf("Failed to read lockfile: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// HandleHealth handles GET /health requests for daemon liveness checks.
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

// HandlePush handles POST /push requests for remote lockfile and config deployment.
func (s *Server) HandlePush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Authenticate request via Bearer token (mutations require configured token)
	if s.Token == "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="cmm"`)
		http.Error(w, "Unauthorized: push requires configured authentication token", http.StatusUnauthorized)
		return
	}

	authHeader := r.Header.Get("Authorization")
	var reqToken string
	if strings.HasPrefix(authHeader, "Bearer ") {
		reqToken = strings.TrimPrefix(authHeader, "Bearer ")
	} else {
		reqToken = authHeader
	}

	if reqToken == "" || subtle.ConstantTimeCompare([]byte(reqToken), []byte(s.Token)) != 1 {
		w.Header().Set("WWW-Authenticate", `Bearer realm="cmm"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 2. Ingest payload (multipart/form-data or JSON fallback)
	contentType := r.Header.Get("Content-Type")
	var lockData []byte
	var configZipBytes []byte
	var dryRun bool

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Parse up to 32MB in RAM
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse multipart payload: %v", err), http.StatusBadRequest)
			return
		}

		// Read lockfile
		if f, _, err := r.FormFile("lockfile"); err == nil {
			lockData, _ = io.ReadAll(f)
			f.Close()
		} else if f, _, err := r.FormFile("cmm.lock"); err == nil {
			lockData, _ = io.ReadAll(f)
			f.Close()
		} else if val := r.FormValue("lockfile"); val != "" {
			lockData = []byte(val)
		} else if val := r.FormValue("cmm.lock"); val != "" {
			lockData = []byte(val)
		}

		// Read config.zip
		if f, _, err := r.FormFile("config"); err == nil {
			configZipBytes, _ = io.ReadAll(f)
			f.Close()
		} else if f, _, err := r.FormFile("config.zip"); err == nil {
			configZipBytes, _ = io.ReadAll(f)
			f.Close()
		}

		// Read dry_run flag
		if r.FormValue("dry_run") == "true" || r.FormValue("dry_run") == "1" {
			dryRun = true
		}
	} else if strings.Contains(contentType, "application/json") {
		type jsonPushReq struct {
			Lockfile  string `json:"lockfile"`
			ConfigZip []byte `json:"config_zip,omitempty"`
			DryRun    bool   `json:"dry_run,omitempty"`
		}
		var jreq jsonPushReq
		if err := json.NewDecoder(r.Body).Decode(&jreq); err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON payload: %v", err), http.StatusBadRequest)
			return
		}
		lockData = []byte(jreq.Lockfile)
		configZipBytes = jreq.ConfigZip
		dryRun = jreq.DryRun
	} else {
		// Fallback: direct body
		rawBody, _ := io.ReadAll(r.Body)
		if len(rawBody) > 0 {
			lockData = rawBody
		}
	}

	if len(lockData) == 0 {
		http.Error(w, "Missing lockfile in request", http.StatusBadRequest)
		return
	}

	// 3. Parse and validate received lockfile
	newLock, err := config.ParseLockfile(lockData)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid lockfile: %v", err), http.StatusBadRequest)
		return
	}

	// Resolve directories
	cfgPath := s.ConfigPath
	if cfgPath == "" {
		cfgPath = "cmm.toml"
	}
	cfg, _ := config.LoadConfig(cfgPath)
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	destModsDir := s.ModsDir
	if destModsDir == "" {
		if cfg.Paths.ModsDir != "" {
			destModsDir = cfg.Paths.ModsDir
		} else {
			destModsDir = "mods"
		}
	}

	destConfigDir := s.ConfigDir
	if destConfigDir == "" {
		if cfg.Paths.ConfigDir != "" {
			destConfigDir = cfg.Paths.ConfigDir
		} else {
			destConfigDir = "config"
		}
	}

	lockPath := s.LockPath
	if lockPath == "" {
		lockPath = "cmm.lock"
	}

	// 4. Handle Config Archive Extraction (with Zip-Slip prevention)
	configsUpdated := 0
	if len(configZipBytes) > 0 {
		zr, err := zip.NewReader(bytes.NewReader(configZipBytes), int64(len(configZipBytes)))
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid config.zip archive: %v", err), http.StatusBadRequest)
			return
		}

		if dryRun {
			count, err := validateConfigZip(zr)
			if err != nil {
				http.Error(w, fmt.Sprintf("Config validation failed: %v", err), http.StatusBadRequest)
				return
			}
			configsUpdated = count
		} else {
			count, err := ExtractConfigZip(zr, destConfigDir)
			if err != nil {
				http.Error(w, fmt.Sprintf("Config extraction failed: %v", err), http.StatusBadRequest)
				return
			}
			configsUpdated = count
		}
	}

	// 5. Prepare Delta Targets (filtering out client-only mods for server daemon)
	var targets []TargetFile
	for _, m := range newLock.Mods {
		if strings.EqualFold(m.Side, "client") {
			continue // skip client-only mods on server daemon
		}
		fileName := m.FileName
		if fileName == "" {
			if m.Slug != "" {
				fileName = m.Slug + ".jar"
			} else if m.Name != "" {
				fileName = m.Name + ".jar"
			} else {
				fileName = "mod.jar"
			}
		}
		targets = append(targets, TargetFile{
			FileName:    fileName,
			SHA512:      m.SHA512,
			DownloadURL: m.DownloadURL,
			Slug:        m.Slug,
			Name:        m.Name,
			ProjectID:   m.ProjectID,
			VersionID:   m.VersionID,
			Version:     m.GetVersion(),
			Side:        m.Side,
		})
	}

	// 6. Handle Dry-Run Mode
	if dryRun {
		added, updated, pruned := calculateDeltaDiff(destModsDir, lockPath, targets)
		resp := PushResponse{
			Success:        true,
			Message:        "Dry-run simulation completed successfully",
			AddedMods:      added,
			UpdatedMods:    updated,
			PrunedMods:     pruned,
			ConfigsUpdated: configsUpdated,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// 7. Atomic Lockfile Persistence (tmp file + rename)
	if parentDir := filepath.Dir(lockPath); parentDir != "" && parentDir != "." {
		_ = os.MkdirAll(parentDir, 0755)
	}
	tmpLockPath := fmt.Sprintf("%s.tmp.%d", lockPath, time.Now().UnixNano())
	if err := config.SaveLockfile(tmpLockPath, newLock); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save lockfile: %v", err), http.StatusInternalServerError)
		return
	}
	if err := os.Rename(tmpLockPath, lockPath); err != nil {
		http.Error(w, fmt.Sprintf("Failed to commit lockfile: %v", err), http.StatusInternalServerError)
		return
	}

	// 8. Execute Server Delta Synchronization
	client := s.Client

	localLock, _ := config.LoadLockfile(lockPath)
	engine := NewDeltaEngine(client, cfg, localLock, destModsDir, lockPath)
	syncRes, err := engine.ApplyTargetFiles(targets)
	if err != nil {
		http.Error(w, fmt.Sprintf("Delta sync failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Ensure the full lockfile (with client mods intact) is re-saved to lockPath
	_ = config.SaveLockfile(lockPath, newLock)

	resp := PushResponse{
		Success:        true,
		Message:        "Push synchronized successfully",
		AddedMods:      syncRes.AddedMods,
		UpdatedMods:    syncRes.UpdatedMods,
		PrunedMods:     syncRes.RemovedMods,
		ConfigsUpdated: configsUpdated,
	}
	if resp.AddedMods == nil {
		resp.AddedMods = []string{}
	}
	if resp.UpdatedMods == nil {
		resp.UpdatedMods = []string{}
	}
	if resp.PrunedMods == nil {
		resp.PrunedMods = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// ExtractConfigZip safely extracts a zip archive into destDir with Zip-Slip protection.
func ExtractConfigZip(zipReader *zip.Reader, destDir string) (int, error) {
	if destDir == "" {
		destDir = "config"
	}
	destDirAbs, err := filepath.Abs(destDir)
	if err != nil {
		return 0, fmt.Errorf("invalid destination directory: %w", err)
	}

	if err := os.MkdirAll(destDirAbs, 0755); err != nil {
		return 0, fmt.Errorf("failed to create destination directory: %w", err)
	}

	count := 0
	for _, f := range zipReader.File {
		// 1. Direct path traversal check on raw name
		if strings.Contains(f.Name, "../") || strings.Contains(f.Name, "..\\") {
			return count, fmt.Errorf("security violation: path traversal detected in entry '%s'", f.Name)
		}

		cleanName := filepath.Clean(f.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return count, fmt.Errorf("security violation: path traversal detected in entry '%s'", f.Name)
		}

		// 2. Strip redundant top-level directory prefixes if present
		normalizedRel := cleanName
		if strings.HasPrefix(normalizedRel, "config/") || strings.HasPrefix(normalizedRel, "config\\") {
			normalizedRel = strings.TrimPrefix(strings.TrimPrefix(normalizedRel, "config/"), "config\\")
		} else if strings.HasPrefix(normalizedRel, "overrides/config/") || strings.HasPrefix(normalizedRel, "overrides/config\\") {
			normalizedRel = strings.TrimPrefix(strings.TrimPrefix(normalizedRel, "overrides/config/"), "overrides/config\\")
		} else if normalizedRel == "config" || normalizedRel == "overrides/config" || normalizedRel == "overrides" {
			continue
		}

		normalizedRel = filepath.Clean(normalizedRel)
		if normalizedRel == "" || normalizedRel == "." {
			continue
		}

		if strings.HasPrefix(normalizedRel, "..") || filepath.IsAbs(normalizedRel) {
			return count, fmt.Errorf("security violation: path traversal detected in entry '%s'", f.Name)
		}

		targetPath := filepath.Join(destDirAbs, normalizedRel)
		targetPath = filepath.Clean(targetPath)

		// 3. Strict containment check under destDirAbs
		rel, err := filepath.Rel(destDirAbs, targetPath)
		if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
			if f.FileInfo().IsDir() && targetPath == destDirAbs {
				continue
			}
			return count, fmt.Errorf("security violation: zip entry '%s' escapes target directory", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return count, fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return count, fmt.Errorf("failed to create parent directory: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return count, fmt.Errorf("failed to open zip entry '%s': %w", f.Name, err)
		}

		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			rc.Close()
			return count, fmt.Errorf("failed to create file '%s': %w", targetPath, err)
		}

		// Security: Defend against decompression bombs (max 50MB per file)
		const maxDecompressedBytes = 50 << 20 // 50MB
		written, copyErr := io.Copy(outFile, io.LimitReader(rc, maxDecompressedBytes+1))
		rc.Close()
		outFile.Close()

		if copyErr != nil {
			_ = os.Remove(targetPath)
			return count, fmt.Errorf("failed to write file '%s': %w", targetPath, copyErr)
		}
		if written > maxDecompressedBytes {
			_ = os.Remove(targetPath)
			return count, fmt.Errorf("security violation: decompressed file '%s' exceeds 50MB limit", f.Name)
		}
		count++
	}

	return count, nil
}

// validateConfigZip performs dry-run validation on zip entries without extracting.
func validateConfigZip(zipReader *zip.Reader) (int, error) {
	count := 0
	for _, f := range zipReader.File {
		if strings.Contains(f.Name, "../") || strings.Contains(f.Name, "..\\") {
			return count, fmt.Errorf("security violation: path traversal detected in entry '%s'", f.Name)
		}

		cleanName := filepath.Clean(f.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return count, fmt.Errorf("security violation: path traversal detected in entry '%s'", f.Name)
		}

		normalizedRel := cleanName
		if strings.HasPrefix(normalizedRel, "config/") || strings.HasPrefix(normalizedRel, "config\\") {
			normalizedRel = strings.TrimPrefix(strings.TrimPrefix(normalizedRel, "config/"), "config\\")
		} else if strings.HasPrefix(normalizedRel, "overrides/config/") || strings.HasPrefix(normalizedRel, "overrides/config\\") {
			normalizedRel = strings.TrimPrefix(strings.TrimPrefix(normalizedRel, "overrides/config/"), "overrides/config\\")
		} else if normalizedRel == "config" || normalizedRel == "overrides/config" || normalizedRel == "overrides" {
			continue
		}

		normalizedRel = filepath.Clean(normalizedRel)
		if normalizedRel == "" || normalizedRel == "." {
			continue
		}

		if strings.HasPrefix(normalizedRel, "..") || filepath.IsAbs(normalizedRel) {
			return count, fmt.Errorf("security violation: path traversal detected in entry '%s'", f.Name)
		}

		if !f.FileInfo().IsDir() {
			if f.UncompressedSize64 > 50<<20 {
				return count, fmt.Errorf("security violation: uncompressed file '%s' exceeds 50MB limit", f.Name)
			}
			count++
		}
	}
	return count, nil
}

// calculateDeltaDiff simulates delta synchronization changes for dry-run requests.
func calculateDeltaDiff(modsDir, lockPath string, targets []TargetFile) (added []string, updated []string, pruned []string) {
	added = []string{}
	updated = []string{}
	pruned = []string{}

	targetMap := make(map[string]TargetFile)
	for _, t := range targets {
		targetMap[t.FileName] = t
		targetMap[strings.ToLower(t.FileName)] = t
	}

	// 1. Pruned check
	entries, err := os.ReadDir(modsDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jar") {
				continue
			}
			fn := entry.Name()
			if _, exists := targetMap[fn]; !exists {
				if _, existsLower := targetMap[strings.ToLower(fn)]; !existsLower {
					pruned = append(pruned, fn)
				}
			}
		}
	}

	// 2. Added / Updated check
	localLock, _ := config.LoadLockfile(lockPath)
	if localLock == nil {
		localLock = &config.Lockfile{}
	}

	for _, target := range targets {
		destPath := filepath.Join(modsDir, target.FileName)
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			existingMod := localLock.GetMod(target.Slug)
			if existingMod == nil {
				existingMod = localLock.GetMod(target.Name)
			}
			if existingMod == nil && target.ProjectID != "" {
				existingMod = localLock.GetMod(target.ProjectID)
			}
			if existingMod != nil && existingMod.FileName != "" && existingMod.FileName != target.FileName {
				updated = append(updated, target.FileName)
			} else {
				added = append(added, target.FileName)
			}
		} else {
			if target.SHA512 != "" {
				localHash, err := mod.ComputeSHA512(destPath)
				if err == nil && !strings.EqualFold(localHash, target.SHA512) {
					updated = append(updated, target.FileName)
				}
			}
		}
	}

	return added, updated, pruned
}

// Start runs the HTTP server and handles graceful shutdown.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/lock", s.HandleLock)
	mux.HandleFunc("/health", s.HandleHealth)
	mux.HandleFunc("/push", s.HandlePush)

	addr := fmt.Sprintf(":%d", s.Port)
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	fmt.Printf("CMM HTTP sync daemon listening on port %d...\n", s.Port)

	select {
	case err := <-serverErr:
		return fmt.Errorf("server error on port %d: %w", s.Port, err)
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal %s, initiating graceful shutdown...\n", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		fmt.Println("Server successfully stopped.")
		return nil
	}
}

// Shutdown stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
