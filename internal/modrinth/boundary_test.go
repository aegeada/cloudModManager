package modrinth

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestModrinth_NewClient_MissingUserAgent(t *testing.T) {
	_, err := NewClient("")
	if err == nil {
		t.Error("expected error when User-Agent is empty, got nil")
	}
	_, err = NewClientWithToken("http://localhost", "", "token")
	if err == nil {
		t.Error("expected error when User-Agent is empty in NewClientWithToken, got nil")
	}
}

func TestModrinth_HttpErrorResponses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/project/not-found":
			http.Error(w, `{"error":"Not Found","description":"Project not found"}`, http.StatusNotFound)
		case "/project/server-err":
			http.Error(w, `{"error":"Internal Error"}`, http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	client, err := NewClientWithToken(ts.URL, "cmm-test", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetProject("not-found")
	if err == nil {
		t.Error("expected error for 404 project, got nil")
	}

	_, err = client.GetProject("server-err")
	if err == nil {
		t.Error("expected error for 500 project, got nil")
	}
}

func TestModrinth_DownloadFile_ChecksumMismatch(t *testing.T) {
	content := "actual downloaded bytes"
	wrongHash := "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	}))
	defer ts.Close()

	client, err := NewClientWithToken(ts.URL, "cmm-test", "")
	if err != nil {
		t.Fatal(err)
	}

	tmpDir, err := os.MkdirTemp("", "cmm-chk-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dest := filepath.Join(tmpDir, "bad_checksum.jar")
	err = client.DownloadFile(ts.URL+"/file.jar", dest, wrongHash)
	if err == nil {
		t.Error("expected error on SHA-512 checksum mismatch, got nil")
	}

	// Verify the corrupted/mismatched file was cleaned up
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("expected destination file to be removed after checksum failure")
	}
}

func TestModrinth_DownloadFile_ValidChecksum(t *testing.T) {
	content := "correct downloaded bytes"
	hasher := sha512.New()
	hasher.Write([]byte(content))
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	}))
	defer ts.Close()

	client, err := NewClientWithToken(ts.URL, "cmm-test", "")
	if err != nil {
		t.Fatal(err)
	}

	tmpDir, err := os.MkdirTemp("", "cmm-chk-pass-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dest := filepath.Join(tmpDir, "valid.jar")
	err = client.DownloadFile(ts.URL+"/file.jar", dest, expectedHash)
	if err != nil {
		t.Fatalf("expected successful download with matching checksum, got: %v", err)
	}

	read, err := os.ReadFile(dest)
	if err != nil || string(read) != content {
		t.Errorf("downloaded file content mismatch")
	}
}

func TestModrinth_BatchHashesAndTags(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/version_files":
			result := map[string]Version{
				"hash123": {ID: "v-123", VersionNumber: "1.0.0"},
			}
			json.NewEncoder(w).Encode(result)
		case "/tag/loader":
			loaders := []LoaderTag{
				{Name: "fabric"},
				{Name: "forge"},
				{Name: "neoforge"},
			}
			json.NewEncoder(w).Encode(loaders)
		case "/tag/game_version":
			versions := []GameVersionTag{
				{Version: "1.21.1", Major: true},
				{Version: "26.2", Major: true},
			}
			json.NewEncoder(w).Encode(versions)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	client, err := NewClientWithToken(ts.URL, "cmm-test", "")
	if err != nil {
		t.Fatal(err)
	}

	batchRes, err := client.GetMultipleVersionsFromHashes([]string{"hash123"}, "sha512")
	if err != nil || len(batchRes) != 1 {
		t.Errorf("batch hash lookup failed: %v", err)
	}

	loaders, err := client.GetLoaderTags()
	if err != nil || len(loaders) != 3 {
		t.Errorf("loader tags failed: %v", err)
	}

	gVers, err := client.GetGameVersionTags()
	if err != nil || len(gVers) != 2 {
		t.Errorf("game version tags failed: %v", err)
	}
}

func TestModrinth_RateLimiter_ConcurrentStress(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Remaining", "100")
		w.Header().Set("X-Ratelimit-Reset", "1")
		w.Write([]byte(`{"hits":[],"total_hits":0,"limit":10}`))
	}))
	defer ts.Close()

	client, err := NewClientWithToken(ts.URL, "cmm-test", "")
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.Search("stress")
			if err != nil {
				t.Errorf("concurrent search failed: %v", err)
			}
		}()
	}
	wg.Wait()
}
