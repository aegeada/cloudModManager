package mockserver

import (
	"encoding/json"
	"net/http"
)

type mockFabricLoader struct {
	Separator string `json:"separator"`
	Build     int    `json:"build"`
	Maven     string `json:"maven"`
	Version   string `json:"version"`
	Stable    bool   `json:"stable"`
}

func fabricMetaLoaderHandler(w http.ResponseWriter, r *http.Request) {
	loaders := []mockFabricLoader{
		{
			Separator: ".",
			Build:     1903,
			Maven:     "net.fabricmc:fabric-loader:0.19.3",
			Version:   "0.19.3",
			Stable:    true,
		},
		{
			Separator: ".",
			Build:     1902,
			Maven:     "net.fabricmc:fabric-loader:0.19.2",
			Version:   "0.19.2",
			Stable:    true,
		},
		{
			Separator: ".",
			Build:     1901,
			Maven:     "net.fabricmc:fabric-loader:0.19.1",
			Version:   "0.19.1",
			Stable:    false,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loaders)
}
