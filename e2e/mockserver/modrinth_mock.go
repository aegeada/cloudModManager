package mockserver

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"cmm/internal/modrinth"
)

func modrinthSearchHandler(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("MOCK_SEARCH_ERROR") == "500" || r.Header.Get("X-Mock-Error") == "500" {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	query := r.URL.Query().Get("query")
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if query == "unknown_mod_xyz" {
		resp := modrinth.SearchResponse{
			Hits:      []modrinth.SearchHit{},
			TotalHits: 0,
			Limit:     limit,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	hits := []modrinth.SearchHit{
		{
			ProjectID:   "AANobbMI",
			ProjectType: "mod",
			Slug:        "sodium",
			Author:      "jellysquid3",
			Title:       "Sodium",
			Description: "Modern rendering engine and client-side optimization mod for Minecraft",
			Categories:  []string{"fabric", "optimization"},
			Versions:    []string{"1.21.1", "26.2"},
			Downloads:   15234890,
			Follows:     25430,
			ClientSide:  "required",
			ServerSide:  "unsupported",
		},
		{
			ProjectID:   "gvQqBUqZ",
			ProjectType: "mod",
			Slug:        "lithium",
			Author:      "jellysquid3",
			Title:       "Lithium",
			Description: "General-purpose optimization mod for Minecraft which works on both client and server",
			Categories:  []string{"fabric", "optimization"},
			Versions:    []string{"1.21.1", "26.2"},
			Downloads:   8452300,
			Follows:     18400,
			ClientSide:  "optional",
			ServerSide:  "required",
		},
		{
			ProjectID:   "P7dR8mSH",
			ProjectType: "mod",
			Slug:        "fabric-api",
			Author:      "modmuss50",
			Title:       "Fabric API",
			Description: "Core API and library mod for Fabric",
			Categories:  []string{"fabric", "library"},
			Versions:    []string{"1.21.1", "26.2"},
			Downloads:   45000000,
			Follows:     50000,
			ClientSide:  "required",
			ServerSide:  "required",
		},
	}

	if limit < len(hits) {
		hits = hits[:limit]
	}

	resp := modrinth.SearchResponse{
		Hits:      hits,
		TotalHits: len(hits),
		Limit:     limit,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func getMockProject(idOrSlug string) *modrinth.Project {
	switch strings.ToLower(idOrSlug) {
	case "non_existent_slug_404", "unknown_mod", "bad404":
		return nil
	case "sodium", "aanobbmi":
		return &modrinth.Project{
			ID:          "AANobbMI",
			Slug:        "sodium",
			Title:       "Sodium",
			Description: "Modern rendering engine and client-side optimization mod for Minecraft",
			ClientSide:  "required",
			ServerSide:  "unsupported",
		}
	case "lithium", "gvqqbuqz":
		return &modrinth.Project{
			ID:          "gvQqBUqZ",
			Slug:        "lithium",
			Title:       "Lithium",
			Description: "General-purpose optimization mod for Minecraft which works on both client and server",
			ClientSide:  "optional",
			ServerSide:  "required",
		}
	case "fabric-api", "p7dr8msh":
		return &modrinth.Project{
			ID:          "P7dR8mSH",
			Slug:        "fabric-api",
			Title:       "Fabric API",
			Description: "Core API and library mod for Fabric",
			ClientSide:  "required",
			ServerSide:  "required",
		}
	case "iris", "yl57xq9u":
		return &modrinth.Project{
			ID:          "YL57xq9U",
			Slug:        "iris",
			Title:       "Iris Shaders",
			Description: "Modern shader mod for Minecraft",
			ClientSide:  "required",
			ServerSide:  "unsupported",
		}
	case "a", "proj_a":
		return &modrinth.Project{
			ID:          "proj_a",
			Slug:        "a",
			Title:       "Mod A",
			Description: "Mod A description",
			ClientSide:  "optional",
			ServerSide:  "required",
		}
	case "b", "proj_b":
		return &modrinth.Project{
			ID:          "proj_b",
			Slug:        "b",
			Title:       "Mod B",
			Description: "Mod B description",
			ClientSide:  "optional",
			ServerSide:  "required",
		}
	case "c", "proj_c":
		return &modrinth.Project{
			ID:          "proj_c",
			Slug:        "c",
			Title:       "Mod C",
			Description: "Mod C description",
			ClientSide:  "optional",
			ServerSide:  "required",
		}
	case "known", "proj_known":
		return &modrinth.Project{
			ID:          "proj_known",
			Slug:        "known",
			Title:       "Known Mod",
			Description: "Known mod description",
			ClientSide:  "optional",
			ServerSide:  "required",
		}
	case "reliable-gliders", "proj_gliders":
		return &modrinth.Project{
			ID:          "proj_gliders",
			Slug:        "reliable-gliders",
			Title:       "Reliable Gliders",
			Description: "Simple and reliable hang gliders for Minecraft",
			ClientSide:  "required",
			ServerSide:  "required",
		}
	case "5aawibi9":
		return &modrinth.Project{
			ID:          "5aaWibi9",
			Slug:        "cloth-config",
			Title:       "Cloth Config v13",
			Description: "Config API for Fabric",
			ClientSide:  "required",
			ServerSide:  "required",
		}
	case "vvuo3imh":
		return &modrinth.Project{
			ID:          "vvuO3ImH",
			Slug:        "architectury-api",
			Title:       "Architectury API",
			Description: "An intermediary api aimed to ease developing multiplatform mods",
			ClientSide:  "required",
			ServerSide:  "required",
		}
	case "9s6osm5g":
		return &modrinth.Project{
			ID:          "9s6osm5g",
			Slug:        "cloth-config-api",
			Title:       "Cloth Config API",
			Description: "Cloth Config API for Minecraft",
			ClientSide:  "required",
			ServerSide:  "required",
		}
	case "pack":
		return &modrinth.Project{
			ID:          "pack",
			Slug:        "pack",
			Title:       "Test Pack",
			Description: "Test Modrinth modpack",
			ProjectType: "modpack",
			ClientSide:  "required",
			ServerSide:  "required",
		}
	default:
		return &modrinth.Project{
			ID:          idOrSlug,
			Slug:        idOrSlug,
			Title:       strings.Title(idOrSlug),
			Description: "Mock description for " + idOrSlug,
			ClientSide:  "optional",
			ServerSide:  "required",
		}
	}
}

func strPtr(s string) *string {
	return &s
}

func getMockVersions(idOrSlug string, host string) []modrinth.Version {
	scheme := "http://"
	baseURL := scheme + host
	sha512 := MockSHA512()
	sha1 := MockSHA1()

	makeFile := func(filename string) []modrinth.VersionFile {
		return []modrinth.VersionFile{
			{
				Hashes: map[string]string{
					"sha512": sha512,
					"sha1":   sha1,
				},
				URL:      baseURL + "/download/" + filename,
				Filename: filename,
				Primary:  true,
				Size:     1024,
			},
		}
	}

	switch strings.ToLower(idOrSlug) {
	case "non_existent_slug_404", "unknown_mod", "bad404":
		return nil
	case "pack":
		mrpackFile := "pack.mrpack"
		return []modrinth.Version{
			{
				ID:            "v-pack-1.0.0",
				ProjectID:     "pack",
				Name:          "Test Pack 1.0.0",
				VersionNumber: "1.0.0",
				GameVersions:  []string{"1.21.1", "26.2"},
				Loaders:       []string{"fabric"},
				Files: []modrinth.VersionFile{
					{
						Hashes: map[string]string{
							"sha512": MockMrpackSHA512(),
							"sha1":   MockMrpackSHA1(),
						},
						URL:      baseURL + "/download/" + mrpackFile,
						Filename: mrpackFile,
						Primary:  true,
						Size:     int64(len(GetMockMrpackContent())),
					},
				},
			},
		}
	case "reliable-gliders", "proj_gliders":
		return []modrinth.Version{
			{
				ID:            "v-gliders-1.3.3",
				ProjectID:     "proj_gliders",
				Name:          "Reliable Gliders 1.3.3-26.2-fabric",
				VersionNumber: "1.3.3-26.2-fabric",
				GameVersions:  []string{"1.21.1", "26.2"},
				Loaders:       []string{"fabric"},
				Dependencies: []modrinth.Dependency{
					{ProjectID: strPtr("5aaWibi9"), DependencyType: "optional"},
					{ProjectID: strPtr("vvuO3ImH"), DependencyType: "optional"},
					{ProjectID: strPtr("9s6osm5g"), DependencyType: "optional"},
				},
				Files: makeFile("reliablegliders-1.3.3-26.2-fabric.jar"),
			},
		}
	case "sodium", "aanobbmi":
		return []modrinth.Version{
			{
				ID:            "v-sodium-0.5.8",
				ProjectID:     "AANobbMI",
				Name:          "Sodium 0.5.8",
				VersionNumber: "0.5.8",
				GameVersions:  []string{"1.21.1", "26.2"},
				Loaders:       []string{"fabric"},
				Changelog:     "Fix rendering bug and improve chunk updates in Sodium 0.5.8",
				Dependencies: []modrinth.Dependency{
					{ProjectID: strPtr("P7dR8mSH"), DependencyType: "required"},
				},
				Files: makeFile("sodium-fabric-0.5.8.jar"),
			},
			{
				ID:            "v-sodium-0.5.3",
				ProjectID:     "AANobbMI",
				Name:          "Sodium 0.5.3",
				VersionNumber: "0.5.3",
				GameVersions:  []string{"1.21.1", "26.2"},
				Loaders:       []string{"fabric"},
				Changelog:     "Sodium 0.5.3 changelog",
				Dependencies: []modrinth.Dependency{
					{ProjectID: strPtr("P7dR8mSH"), DependencyType: "required"},
				},
				Files: makeFile("sodium-fabric-0.5.3.jar"),
			},
			{
				ID:            "v-sodium-0.5.0",
				ProjectID:     "AANobbMI",
				Name:          "Sodium 0.5.0",
				VersionNumber: "0.5.0",
				GameVersions:  []string{"1.21.1", "26.2"},
				Loaders:       []string{"fabric"},
				Changelog:     "Sodium 0.5.0 release",
				Dependencies: []modrinth.Dependency{
					{ProjectID: strPtr("P7dR8mSH"), DependencyType: "required"},
				},
				Files: makeFile("sodium-fabric-0.5.0.jar"),
			},
			{
				ID:            "v-sodium-0.4.0",
				ProjectID:     "AANobbMI",
				Name:          "Sodium 0.4.0",
				VersionNumber: "0.4.0",
				GameVersions:  []string{"1.21.1", "26.2"},
				Loaders:       []string{"fabric"},
				Changelog:     "Sodium 0.4.0 release",
				Dependencies: []modrinth.Dependency{
					{ProjectID: strPtr("P7dR8mSH"), DependencyType: "required"},
				},
				Files: makeFile("sodium-fabric-0.4.0.jar"),
			},
			{
				ID:            "v-sodium-1.0",
				ProjectID:     "AANobbMI",
				Name:          "Sodium 1.0",
				VersionNumber: "1.0",
				GameVersions:  []string{"1.21.1", "26.2"},
				Loaders:       []string{"fabric"},
				Changelog:     "Sodium 1.0 release",
				Dependencies: []modrinth.Dependency{
					{ProjectID: strPtr("P7dR8mSH"), DependencyType: "required"},
				},
				Files: makeFile("sodium-fabric-1.0.jar"),
			},
		}
	case "lithium", "gvqqbuqz":
		return []modrinth.Version{
			{
				ID:            "v-lithium-0.11.2",
				ProjectID:     "gvQqBUqZ",
				Name:          "Lithium 0.11.2",
				VersionNumber: "0.11.2",
				GameVersions:  []string{"1.21.1", "26.2"},
				Loaders:       []string{"fabric"},
				Changelog:     "Lithium 0.11.2 performance optimizations and bugfixes",
				Files:         makeFile("lithium-fabric-0.11.2.jar"),
			},
			{
				ID:            "v-lithium-0.11.0",
				ProjectID:     "gvQqBUqZ",
				Name:          "Lithium 0.11.0",
				VersionNumber: "0.11.0",
				GameVersions:  []string{"1.21.1", "26.2"},
				Loaders:       []string{"fabric"},
				Changelog:     "Lithium 0.11.0 initial release",
				Files:         makeFile("lithium-fabric-0.11.0.jar"),
			},
		}
	case "fabric-api", "p7dr8msh":
		return []modrinth.Version{
			{
				ID:            "v-fapi-0.100.0",
				ProjectID:     "P7dR8mSH",
				Name:          "Fabric API 0.100.0",
				VersionNumber: "0.100.0",
				GameVersions:  []string{"1.21.1", "26.2"},
				Loaders:       []string{"fabric"},
				Changelog:     "Fabric API 0.100.0 for 1.21.1",
				Files:         makeFile("fabric-api-0.100.0.jar"),
			},
		}
	case "iris", "yl57xq9u":
		return []modrinth.Version{
			{
				ID:            "v-iris-1.7.0",
				ProjectID:     "YL57xq9U",
				Name:          "Iris 1.7.0",
				VersionNumber: "1.7.0",
				GameVersions:  []string{"1.21.1", "26.2"},
				Loaders:       []string{"fabric"},
				Changelog:     "Iris Shaders 1.7.0 release",
				Files:         makeFile("iris-fabric-1.7.0.jar"),
			},
		}
	case "a", "proj_a":
		return []modrinth.Version{
			{
				ID:            "v-a-1.0",
				ProjectID:     "proj_a",
				Name:          "Mod A 1.0",
				VersionNumber: "1.0",
				GameVersions:  []string{"1.21.1", "26.2"},
				Loaders:       []string{"fabric"},
				Dependencies: []modrinth.Dependency{
					{ProjectID: strPtr("P7dR8mSH"), DependencyType: "required"},
				},
				Files: makeFile("a-1.0.jar"),
			},
		}
	case "b", "proj_b":
		return []modrinth.Version{
			{
				ID:            "v-b-1.0",
				ProjectID:     "proj_b",
				Name:          "Mod B 1.0",
				VersionNumber: "1.0",
				GameVersions:  []string{"1.21.1", "26.2"},
				Loaders:       []string{"fabric"},
				Dependencies: []modrinth.Dependency{
					{ProjectID: strPtr("P7dR8mSH"), DependencyType: "required"},
				},
				Files: makeFile("b-1.0.jar"),
			},
		}
	case "c", "proj_c":
		return []modrinth.Version{
			{
				ID:            "v-c-1.0",
				ProjectID:     "proj_c",
				Name:          "Mod C 1.0",
				VersionNumber: "1.0",
				GameVersions:  []string{"1.21.1", "26.2"},
				Loaders:       []string{"fabric"},
				Files:         makeFile("c-1.0.jar"),
			},
		}
	case "known", "proj_known":
		return []modrinth.Version{
			{
				ID:            "v-known-1.0.0",
				ProjectID:     "proj_known",
				Name:          "Known 1.0.0",
				VersionNumber: "1.0.0",
				GameVersions:  []string{"1.21.1", "26.2"},
				Loaders:       []string{"fabric"},
				Files:         makeFile("known.jar"),
			},
		}
	default:
		return []modrinth.Version{
			{
				ID:            "v-" + idOrSlug + "-1.0",
				ProjectID:     idOrSlug,
				Name:          strings.Title(idOrSlug) + " 1.0",
				VersionNumber: "1.0",
				GameVersions:  []string{"1.21.1", "26.2"},
				Loaders:       []string{"fabric"},
				Files:         makeFile(idOrSlug + "-1.0.jar"),
			},
		}
	}
}

func modrinthProjectHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v2/project/")
	if strings.HasSuffix(path, "/version") {
		slug := strings.TrimSuffix(path, "/version")
		proj := getMockProject(slug)
		if proj == nil {
			http.Error(w, "Project not found", http.StatusNotFound)
			return
		}
		versions := getMockVersions(slug, r.Host)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(versions)
		return
	}

	slug := path
	proj := getMockProject(slug)
	if proj == nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proj)
}

func modrinthVersionHandler(w http.ResponseWriter, r *http.Request) {
	vID := strings.TrimPrefix(r.URL.Path, "/v2/version/")
	allSlugs := []string{"sodium", "lithium", "fabric-api", "iris", "a", "b", "c", "known", "pack"}
	for _, slug := range allSlugs {
		versions := getMockVersions(slug, r.Host)
		for _, v := range versions {
			if v.ID == vID {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(v)
				return
			}
		}
	}

	ver := modrinth.Version{
		ID:            vID,
		VersionNumber: "1.0.0",
		Name:          "Version " + vID,
		Files: []modrinth.VersionFile{
			{
				Hashes: map[string]string{
					"sha512": MockSHA512(),
					"sha1":   MockSHA1(),
				},
				URL:      "http://" + r.Host + "/download/" + vID + ".jar",
				Filename: vID + ".jar",
				Primary:  true,
				Size:     1024,
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ver)
}

func modrinthVersionFileHandler(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimPrefix(r.URL.Path, "/v2/version_file/")
	ver := modrinth.Version{
		ID:            "v-" + hash,
		VersionNumber: "1.0.0",
		Files: []modrinth.VersionFile{
			{
				Hashes: map[string]string{
					"sha512": MockSHA512(),
					"sha1":   MockSHA1(),
				},
				URL:      "http://" + r.Host + "/download/file-" + hash + ".jar",
				Filename: "file-" + hash + ".jar",
				Primary:  true,
				Size:     1024,
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ver)
}

func modrinthVersionFilesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Hashes    []string `json:"hashes"`
		Algorithm string   `json:"algorithm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	result := make(map[string]modrinth.Version)

	unknownSha512 := "ba8f0d3937ddaf252e41e89a1f9ae52b80a7e7347545098bdeab3d0aa90e865dc4056e7d69b3a623fb19beb2d9fb284089e688f99f6afa131b1bb4b053174246"
	unknownSha1 := "50d8b4a941c26b89482c94ab324b5a274f9ced66"

	sodium1Sha512 := "81f5a9b8b9f843a2acd8424860ad392e011dc410fead9d62e71dd92290fe2100d991935024814e0b37bedb69abcd42cd5c457c74bba483f492b6c8ce4dbc126b"
	lithium2Sha512 := "b253d437c1a11dfe22a65f9108f2113564c682dbe520efa09fa13330b6a94a7f06d8b0d3dafc49dc36dc6380c44334185f129c433351d753e729db8f55627a0d"
	knownSha512 := "b8bf8adab1032b322722e2a2b66295659cacfed48ce2612df48b23f0979a232ca954fbdfc3801f68f1aa6cceaf75e3b93bda3ee7d57be8233e0465d0b76b2762"
	sodiumMockSha512 := "ed266610ed44d2758d08bbaef6942c268de37393d578f5f594bba61d6f370d67d13667b0c09668c8b644741137ccb9a9b54f9dcee572e38d87bdc66bf1e9cb9f"

	for _, h := range req.Hashes {
		lowerH := strings.ToLower(h)
		if lowerH == unknownSha512 || lowerH == unknownSha1 || strings.Contains(lowerH, "unknown") {
			continue
		}

		if lowerH == sodium1Sha512 || lowerH == sodiumMockSha512 || lowerH == strings.ToLower(MockSHA512()) {
			result[h] = modrinth.Version{
				ID:            "v-sodium-0.5.8",
				ProjectID:     "AANobbMI",
				Name:          "Sodium 0.5.8",
				VersionNumber: "0.5.8",
				Files: []modrinth.VersionFile{
					{
						Hashes:   map[string]string{"sha512": h, "sha1": MockSHA1()},
						URL:      "http://" + r.Host + "/download/sodium.jar",
						Filename: "sodium.jar",
						Primary:  true,
						Size:     1024,
					},
				},
			}
		} else if lowerH == lithium2Sha512 {
			result[h] = modrinth.Version{
				ID:            "v-lithium-0.11.2",
				ProjectID:     "gvQqBUqZ",
				Name:          "Lithium 0.11.2",
				VersionNumber: "0.11.2",
				Files: []modrinth.VersionFile{
					{
						Hashes:   map[string]string{"sha512": h, "sha1": MockSHA1()},
						URL:      "http://" + r.Host + "/download/lithium.jar",
						Filename: "lithium.jar",
						Primary:  true,
						Size:     1024,
					},
				},
			}
		} else if lowerH == knownSha512 {
			result[h] = modrinth.Version{
				ID:            "v-known-1.0.0",
				ProjectID:     "proj_known",
				Name:          "Known 1.0.0",
				VersionNumber: "1.0.0",
				Files: []modrinth.VersionFile{
					{
						Hashes:   map[string]string{"sha512": h, "sha1": MockSHA1()},
						URL:      "http://" + r.Host + "/download/known.jar",
						Filename: "known.jar",
						Primary:  true,
						Size:     1024,
					},
				},
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func modrinthTagLoaderHandler(w http.ResponseWriter, r *http.Request) {
	loaders := []modrinth.LoaderTag{
		{Name: "fabric", SupportedProjectTypes: []string{"mod"}},
		{Name: "forge", SupportedProjectTypes: []string{"mod"}},
		{Name: "neoforge", SupportedProjectTypes: []string{"mod"}},
		{Name: "quilt", SupportedProjectTypes: []string{"mod"}},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loaders)
}

func modrinthTagGameVersionHandler(w http.ResponseWriter, r *http.Request) {
	versions := []modrinth.GameVersionTag{
		{Version: "1.21.1", VersionType: "release", Major: true},
		{Version: "26.2", VersionType: "release", Major: true},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(versions)
}
