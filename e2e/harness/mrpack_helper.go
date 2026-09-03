package harness

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type MrpackIndex struct {
	FormatVersion int               `json:"formatVersion"`
	Game          string            `json:"game"`
	VersionID     string            `json:"versionId"`
	Name          string            `json:"name"`
	Summary       string            `json:"summary,omitempty"`
	Files         []MrpackFileEntry `json:"files"`
	Dependencies  map[string]string `json:"dependencies"`
}

type MrpackFileEntry struct {
	Path      string            `json:"path"`
	Hashes    map[string]string `json:"hashes"`
	Env       MrpackEnvEntry    `json:"env"`
	Downloads []string          `json:"downloads"`
	FileSize  int64             `json:"fileSize,omitempty"`
}

type MrpackEnvEntry struct {
	Client string `json:"client"`
	Server string `json:"server"`
}

// CreateMockMrpackZip creates a .mrpack ZIP archive with index and optional overrides.
func CreateMockMrpackZip(destPath string, index MrpackIndex, overrides map[string][]byte) error {
	dir := filepath.Dir(destPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	indexData, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	iw, err := zw.Create("modrinth.index.json")
	if err != nil {
		return err
	}
	if _, err := iw.Write(indexData); err != nil {
		return err
	}

	for relPath, content := range overrides {
		ow, err := zw.Create(filepath.Join("overrides", relPath))
		if err != nil {
			return err
		}
		if _, err := ow.Write(content); err != nil {
			return err
		}
	}

	return nil
}

// ReadMrpackZip extracts all file entries from a .mrpack ZIP archive into memory.
func ReadMrpackZip(zipPath string) (map[string][]byte, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	files := make(map[string][]byte)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		files[f.Name] = data
	}
	return files, nil
}

// ReadMrpackIndex parses modrinth.index.json from a .mrpack ZIP archive.
func ReadMrpackIndex(zipPath string) (*MrpackIndex, error) {
	files, err := ReadMrpackZip(zipPath)
	if err != nil {
		return nil, err
	}
	data, ok := files["modrinth.index.json"]
	if !ok {
		return nil, fmt.Errorf("modrinth.index.json not found in %s", zipPath)
	}
	var index MrpackIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}
	return &index, nil
}
