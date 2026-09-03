package mockserver

import (
	"net/http"
	"net/http/httptest"
)

type MockServer struct {
	Server *httptest.Server
	mux    *http.ServeMux
}

func New() *MockServer {
	mux := http.NewServeMux()
	s := &MockServer{
		mux: mux,
	}
	s.registerRoutes()
	s.Server = httptest.NewServer(mux)
	return s
}

func (s *MockServer) Close() {
	s.Server.Close()
}

func (s *MockServer) URL() string {
	return s.Server.URL
}

func mockLockHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("[mods]\n"))
}

func (s *MockServer) registerRoutes() {
	s.mux.HandleFunc("/v2/search", modrinthSearchHandler)
	s.mux.HandleFunc("/v2/project/", modrinthProjectHandler)
	s.mux.HandleFunc("/v2/version/", modrinthVersionHandler)
	s.mux.HandleFunc("/v2/version_file/", modrinthVersionFileHandler)
	s.mux.HandleFunc("/v2/version_files", modrinthVersionFilesHandler)
	s.mux.HandleFunc("/v2/tag/loader", modrinthTagLoaderHandler)
	s.mux.HandleFunc("/v2/tag/game_version", modrinthTagGameVersionHandler)

	s.mux.HandleFunc("/download/", fileDownloadHandler)

	s.mux.HandleFunc("/fabric-meta/v2/versions/loader", fabricMetaLoaderHandler)
	s.mux.HandleFunc("/github/repos/", githubRepoHandler)
	s.mux.HandleFunc("/repos/", githubRepoHandler)
	s.mux.HandleFunc("/lock", mockLockHandler)
}
