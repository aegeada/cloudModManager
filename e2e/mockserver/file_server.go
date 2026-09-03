package mockserver

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
)

var MockJarContent = []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00mock-jar-content-for-cmm-testing")

var (
	mockMrpackOnce    sync.Once
	mockMrpackContent []byte
)

func GetMockMrpackContent() []byte {
	mockMrpackOnce.Do(func() {
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)

		indexJSON := `{
  "formatVersion": 1,
  "game": "minecraft",
  "versionId": "1.0.0",
  "name": "Test Modpack",
  "summary": "Mock modpack for testing",
  "files": [
    {
      "path": "mods/sodium-fabric-0.5.8.jar",
      "hashes": {
        "sha512": "` + MockSHA512() + `",
        "sha1": "` + MockSHA1() + `"
      },
      "env": {
        "client": "required",
        "server": "required"
      },
      "downloads": [
        "/download/sodium-fabric-0.5.8.jar"
      ]
    },
    {
      "path": "mods/client-only-mod.jar",
      "hashes": {
        "sha512": "` + MockSHA512() + `",
        "sha1": "` + MockSHA1() + `"
      },
      "env": {
        "client": "required",
        "server": "unsupported"
      },
      "downloads": [
        "/download/client-only-mod.jar"
      ]
    }
  ],
  "dependencies": {
    "minecraft": "1.21.1",
    "fabric-loader": "0.19.3"
  }
}`
		iw, _ := zw.Create("modrinth.index.json")
		iw.Write([]byte(indexJSON))
		zw.Close()
		mockMrpackContent = buf.Bytes()
	})
	return mockMrpackContent
}

func MockSHA512() string {
	sum := sha512.Sum512(MockJarContent)
	return hex.EncodeToString(sum[:])
}

func MockSHA1() string {
	sum := sha1.Sum(MockJarContent)
	return hex.EncodeToString(sum[:])
}

func MockMrpackSHA512() string {
	sum := sha512.Sum512(GetMockMrpackContent())
	return hex.EncodeToString(sum[:])
}

func MockMrpackSHA1() string {
	sum := sha1.Sum(GetMockMrpackContent())
	return hex.EncodeToString(sum[:])
}

func fileDownloadHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, ".mrpack") {
		w.Header().Set("Content-Type", "application/x-modrinth-modpack+zip")
		w.WriteHeader(http.StatusOK)
		w.Write(GetMockMrpackContent())
		return
	}

	w.Header().Set("Content-Type", "application/java-archive")
	w.WriteHeader(http.StatusOK)
	w.Write(MockJarContent)
}
