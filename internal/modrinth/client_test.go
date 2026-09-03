package modrinth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestModrinthClient_Search(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("expected /search, got %s", r.URL.Path)
		}
		if r.Header.Get("User-Agent") != "cmm-test" {
			t.Errorf("expected User-Agent cmm-test, got %s", r.Header.Get("User-Agent"))
		}

		resp := SearchResponse{
			Hits: []SearchHit{
				{
					ProjectID:   "AANobbMI",
					Slug:        "sodium",
					Title:       "Sodium",
					Description: "Blocky optimization mod",
					Downloads:   1000000,
					ClientSide:  "required",
					ServerSide:  "unsupported",
				},
			},
			TotalHits: 1,
			Limit:     10,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, err := NewClientWithToken(ts.URL, "cmm-test", "")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	res, err := client.Search("sodium")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(res.Hits) != 1 || res.Hits[0].Slug != "sodium" {
		t.Errorf("unexpected hits: %+v", res.Hits)
	}
}

func TestModrinthClient_GetProjectAndVersions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/project/sodium":
			proj := Project{
				ID:           "AANobbMI",
				Slug:         "sodium",
				Title:        "Sodium",
				ClientSide:   "required",
				ServerSide:   "unsupported",
				GameVersions: []string{"1.21.1"},
			}
			json.NewEncoder(w).Encode(proj)
		case "/project/sodium/version":
			versions := []Version{
				{
					ID:            "v123",
					ProjectID:     "AANobbMI",
					VersionNumber: "0.5.8",
					GameVersions:  []string{"1.21.1"},
					Loaders:       []string{"fabric"},
				},
			}
			json.NewEncoder(w).Encode(versions)
		case "/version/v123":
			v := Version{
				ID:            "v123",
				VersionNumber: "0.5.8",
			}
			json.NewEncoder(w).Encode(v)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	client, err := NewClientWithToken(ts.URL, "cmm-test", "")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	p, err := client.GetProject("sodium")
	if err != nil {
		t.Fatalf("get project failed: %v", err)
	}
	if p.Slug != "sodium" {
		t.Errorf("expected sodium, got %s", p.Slug)
	}

	versions, err := client.GetProjectVersions("sodium", []string{"fabric"}, []string{"1.21.1"}, nil)
	if err != nil {
		t.Fatalf("get versions failed: %v", err)
	}
	if len(versions) != 1 || versions[0].ID != "v123" {
		t.Errorf("unexpected versions: %+v", versions)
	}

	ver, err := client.GetVersion("v123")
	if err != nil {
		t.Fatalf("get version failed: %v", err)
	}
	if ver.VersionNumber != "0.5.8" {
		t.Errorf("expected 0.5.8, got %s", ver.VersionNumber)
	}
}

func TestModrinthClient_DownloadFile(t *testing.T) {
	content := "dummy mod file content"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	}))
	defer ts.Close()

	client, err := NewClientWithToken(ts.URL, "cmm-test", "")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "cmm-dl-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dest := filepath.Join(tmpDir, "test.jar")
	err = client.DownloadFile(ts.URL+"/test.jar", dest, "")
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}

	read, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	if string(read) != content {
		t.Errorf("content mismatch: expected %s, got %s", content, string(read))
	}
}
