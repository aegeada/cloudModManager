package mockserver

import (
	"fmt"
	"net/http"
	"strings"
)

func githubRepoHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.ToLower(r.URL.Path)

	// Non-existent repo -> HTTP 404
	if strings.Contains(path, "nonexistent") {
		http.Error(w, "Not Found (404): repository does not exist", http.StatusNotFound)
		return
	}

	// Private repo -> Token validation
	if strings.Contains(path, "private") {
		auth := r.Header.Get("Authorization")
		if !strings.Contains(auth, "ghp_mock") {
			http.Error(w, "Unauthorized (401)", http.StatusUnauthorized)
			return
		}
	}

	// Standard response: TOML lockfile
	content := fmt.Sprintf(`[[mods]]
slug = "sodium"
name = "Sodium"
version = "0.5.8"
file_name = "sodium-fabric-0.5.8.jar"
sha512 = "%s"
download_url = "/download/sodium-fabric-0.5.8.jar"
`, MockSHA512())
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(content))
}
