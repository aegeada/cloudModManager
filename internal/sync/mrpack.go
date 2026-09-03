package sync

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
)

// ParseMrpack extracts and parses modrinth.index.json from a .mrpack zip archive.
func ParseMrpack(archivePath string) (*ModpackIndex, error) {
	stat, err := os.Stat(archivePath)
	if err != nil {
		return nil, fmt.Errorf("mrpack file not found: %w", err)
	}
	if stat.IsDir() {
		return nil, fmt.Errorf("mrpack path '%s' is a directory", archivePath)
	}

	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("invalid mrpack zip archive: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "modrinth.index.json" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open modrinth.index.json: %w", err)
			}
			defer rc.Close()

			var index ModpackIndex
			if err := json.NewDecoder(rc).Decode(&index); err != nil {
				return nil, fmt.Errorf("failed to decode modrinth.index.json: %w", err)
			}
			return &index, nil
		}
	}

	return nil, fmt.Errorf("modrinth.index.json not found in mrpack archive")
}
