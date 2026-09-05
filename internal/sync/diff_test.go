package sync

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cmm/internal/config"
)

func TestDiffEngine_AllFiveCategories(t *testing.T) {
	leftLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "fabric-api",
				Name:          "Fabric API",
				VersionNumber: "0.92.0+1.21",
				Side:          "both",
			},
			{
				Slug:          "sodium",
				Name:          "Sodium",
				VersionNumber: "0.5.8",
				Side:          "client",
			},
			{
				Slug:          "lithium",
				Name:          "Lithium",
				VersionNumber: "0.11.2",
				Side:          "both",
			},
			{
				Slug:          "iris",
				Name:          "Iris Shaders",
				VersionNumber: "1.7.0",
				Side:          "client",
			},
			{
				Slug:          "jei",
				Name:          "Just Enough Items",
				VersionNumber: "19.0.0",
				Side:          "both",
			},
		},
	}

	rightLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "fabric-api",
				Name:          "Fabric API",
				VersionNumber: "0.92.0+1.21",
				Side:          "both",
			},
			{
				Slug:          "lithium",
				Name:          "Lithium",
				VersionNumber: "0.11.0",
				Side:          "both",
			},
			{
				Slug:          "chunky",
				Name:          "Chunky",
				VersionNumber: "1.4.2",
				Side:          "server",
			},
			{
				Slug:          "worldedit",
				Name:          "WorldEdit",
				VersionNumber: "7.3.0",
				Side:          "both",
			},
		},
	}

	engine := NewDiffEngine()
	res := engine.CompareLockfiles(leftLock, rightLock)

	if res == nil {
		t.Fatalf("expected non-nil DiffResult")
	}

	// Total expected:
	// fabric-api -> [OK] (sync)
	// lithium -> [MISMATCH] (version mismatch)
	// iris -> [CLIENT] (client-only mod)
	// sodium -> [CLIENT] (client-only mod)
	// jei -> [MISSING] (missing on server)
	// chunky -> [SERVER] (server-only mod)
	// worldedit -> [MISSING] (missing on client)
	// Total: 7 mods
	if res.Total != 7 {
		t.Errorf("expected Total=7, got %d", res.Total)
	}
	if res.Synchronized != 1 {
		t.Errorf("expected Synchronized=1 (fabric-api), got %d", res.Synchronized)
	}
	if res.Mismatches != 1 {
		t.Errorf("expected Mismatches=1 (lithium), got %d", res.Mismatches)
	}
	if res.ClientOnly != 2 {
		t.Errorf("expected ClientOnly=2 (iris, sodium), got %d", res.ClientOnly)
	}
	if res.ServerOnly != 1 {
		t.Errorf("expected ServerOnly=1 (chunky), got %d", res.ServerOnly)
	}
	if res.Missing != 2 {
		t.Errorf("expected Missing=2 (jei, worldedit), got %d", res.Missing)
	}

	// Verify entries exist and have expected category and status tag
	entryMap := make(map[string]ModDiffEntry)
	for _, e := range res.Entries {
		entryMap[e.Slug] = e
	}

	// 1. fabric-api -> OK
	fapi, ok := entryMap["fabric-api"]
	if !ok || fapi.Category != DiffOK || fapi.Status != "[OK]" || fapi.LocalVersion != "0.92.0+1.21" || fapi.RemoteVersion != "0.92.0+1.21" {
		t.Errorf("unexpected fabric-api entry: %+v", fapi)
	}

	// 2. lithium -> MISMATCH
	lith, ok := entryMap["lithium"]
	if !ok || lith.Category != DiffMismatch || lith.Status != "[MISMATCH]" || lith.LocalVersion != "0.11.2" || lith.RemoteVersion != "0.11.0" {
		t.Errorf("unexpected lithium entry: %+v", lith)
	}

	// 3. iris -> CLIENT
	iris, ok := entryMap["iris"]
	if !ok || iris.Category != DiffClient || iris.Status != "[CLIENT]" || iris.LocalVersion != "1.7.0" || iris.RemoteVersion != "" {
		t.Errorf("unexpected iris entry: %+v", iris)
	}

	// 4. chunky -> SERVER
	chunky, ok := entryMap["chunky"]
	if !ok || chunky.Category != DiffServer || chunky.Status != "[SERVER]" || chunky.LocalVersion != "" || chunky.RemoteVersion != "1.4.2" {
		t.Errorf("unexpected chunky entry: %+v", chunky)
	}

	// 5. jei -> MISSING on server
	jei, ok := entryMap["jei"]
	if !ok || jei.Category != DiffMissing || jei.Status != "[MISSING]" || jei.MissingOn != "server" {
		t.Errorf("unexpected jei entry: %+v", jei)
	}

	// 6. worldedit -> MISSING on client
	we, ok := entryMap["worldedit"]
	if !ok || we.Category != DiffMissing || we.Status != "[MISSING]" || we.MissingOn != "client" {
		t.Errorf("unexpected worldedit entry: %+v", we)
	}
}

func TestDiffEngine_SlugExtractionAndMatching(t *testing.T) {
	// Left lockfile has filename and extracted slug
	leftLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				FileName: "sodium-fabric-mc1.21.1-0.5.8.jar",
				Name:     "Sodium",
				Version:  "0.5.8",
				Side:     "both",
			},
			{
				ProjectID: "P7dR8mSH",
				Slug:      "fabric-api",
				Name:      "Fabric API",
				Version:   "0.92.0",
				Side:      "both",
			},
		},
	}

	// Right lockfile has clean slug and project ID
	rightLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "sodium",
				Name:          "Sodium",
				VersionNumber: "0.5.8",
				Side:          "both",
			},
			{
				ProjectID:     "P7dR8mSH",
				VersionNumber: "0.92.0",
				Side:          "both",
			},
		},
	}

	res := CompareLockfiles(leftLock, rightLock)
	if res.Total != 2 {
		t.Fatalf("expected 2 matched mods, got %d", res.Total)
	}
	if res.Synchronized != 2 {
		t.Errorf("expected 2 synchronized mods, got %d", res.Synchronized)
	}
	if res.Mismatches != 0 || res.Missing != 0 {
		t.Errorf("expected 0 mismatches/missing, got mismatches=%d missing=%d", res.Mismatches, res.Missing)
	}
}

func TestDiffEngine_EmptyAndNilLockfiles(t *testing.T) {
	engine := NewDiffEngine()

	// Both nil
	res1 := engine.CompareLockfiles(nil, nil)
	if res1.Total != 0 || len(res1.Entries) != 0 {
		t.Errorf("expected empty result for nil lockfiles, got %+v", res1)
	}

	// Left empty, right has 1 server mod
	res2 := engine.CompareLockfiles(&config.Lockfile{}, &config.Lockfile{
		Mods: []config.LockfileMod{
			{Slug: "luckperms", Name: "LuckPerms", VersionNumber: "5.4.102", Side: "server"},
		},
	})
	if res2.Total != 1 || res2.ServerOnly != 1 || res2.Entries[0].Category != DiffServer {
		t.Errorf("expected 1 server-only mod, got %+v", res2)
	}

	// Left has 1 client mod, right empty
	res3 := engine.CompareLockfiles(&config.Lockfile{
		Mods: []config.LockfileMod{
			{Slug: "iris", Name: "Iris", VersionNumber: "1.7.0", Side: "client"},
		},
	}, &config.Lockfile{})
	if res3.Total != 1 || res3.ClientOnly != 1 || res3.Entries[0].Category != DiffClient {
		t.Errorf("expected 1 client-only mod, got %+v", res3)
	}
}

func TestDiffEngine_JSONSerialization(t *testing.T) {
	leftLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{Slug: "sodium", Name: "Sodium", VersionNumber: "0.5.8", Side: "client"},
		},
	}
	rightLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{Slug: "sodium", Name: "Sodium", VersionNumber: "0.5.8", Side: "client"},
		},
	}

	res := CompareLockfiles(leftLock, rightLock)
	jsonStr, err := res.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}

	var decoded DiffResult
	if err := json.Unmarshal([]byte(jsonStr), &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if decoded.Total != 1 || decoded.Synchronized != 1 || len(decoded.Entries) != 1 {
		t.Errorf("unexpected decoded JSON: %+v", decoded)
	}

	// Test FormatJSON
	var buf bytes.Buffer
	if err := FormatJSON(&buf, res); err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}
	if !strings.Contains(buf.String(), `"synchronized": 1`) {
		t.Errorf("FormatJSON output missing synchronized count: %s", buf.String())
	}
}

func TestDiffEngine_TableFormatting(t *testing.T) {
	leftLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{Slug: "fabric-api", Name: "Fabric API", VersionNumber: "0.92.0", Side: "both"},
			{Slug: "sodium", Name: "Sodium", VersionNumber: "0.5.8", Side: "client"},
		},
	}
	rightLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{Slug: "fabric-api", Name: "Fabric API", VersionNumber: "0.92.0", Side: "both"},
		},
	}

	res := CompareLockfiles(leftLock, rightLock)
	tableStr := res.TableString()

	if !strings.Contains(tableStr, "STATUS") || !strings.Contains(tableStr, "SLUG") {
		t.Errorf("table missing header columns: %s", tableStr)
	}
	if !strings.Contains(tableStr, "[OK]") {
		t.Errorf("table missing [OK] badge: %s", tableStr)
	}
	if !strings.Contains(tableStr, "[CLIENT]") {
		t.Errorf("table missing [CLIENT] badge: %s", tableStr)
	}
	if !strings.Contains(tableStr, "Summary: 2 total mods | 1 synchronized | 0 mismatches | 1 client-only | 0 server-only | 0 missing") {
		t.Errorf("table missing or incorrect summary footer: %s", tableStr)
	}

	// Test empty table
	emptyRes := CompareLockfiles(nil, nil)
	emptyTable := emptyRes.TableString()
	if !strings.Contains(emptyTable, "No mods to compare or lockfiles are empty.") {
		t.Errorf("empty table expected empty message, got: %s", emptyTable)
	}
}

func TestDiffEngine_CompareFiles(t *testing.T) {
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "client.lock")
	file2 := filepath.Join(tempDir, "server.lock")

	lock1 := `
[[mods]]
slug = "fabric-api"
name = "Fabric API"
version_number = "0.92.0"
side = "both"

[[mods]]
slug = "sodium"
name = "Sodium"
version_number = "0.5.8"
side = "client"
`
	lock2 := `
[[mods]]
slug = "fabric-api"
name = "Fabric API"
version_number = "0.91.0"
side = "both"

[[mods]]
slug = "chunky"
name = "Chunky"
version_number = "1.4.2"
side = "server"
`
	if err := os.WriteFile(file1, []byte(lock1), 0644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte(lock2), 0644); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}

	res, err := CompareFiles(file1, file2)
	if err != nil {
		t.Fatalf("CompareFiles failed: %v", err)
	}

	if res.Total != 3 {
		t.Errorf("expected 3 mods, got %d", res.Total)
	}
	if res.Mismatches != 1 { // fabric-api version mismatch (0.92.0 vs 0.91.0)
		t.Errorf("expected 1 mismatch, got %d", res.Mismatches)
	}
	if res.ClientOnly != 1 { // sodium
		t.Errorf("expected 1 client-only, got %d", res.ClientOnly)
	}
	if res.ServerOnly != 1 { // chunky
		t.Errorf("expected 1 server-only, got %d", res.ServerOnly)
	}

	// Test non-existent file error
	_, err = CompareFiles(filepath.Join(tempDir, "nonexistent.lock"), file2)
	if err == nil {
		t.Errorf("expected error for non-existent file")
	}
}

func TestDiffEngine_CompareRemote(t *testing.T) {
	remoteLockTOML := `
[[mods]]
slug = "fabric-api"
name = "Fabric API"
version_number = "0.92.0"
side = "both"

[[mods]]
slug = "chunky"
name = "Chunky"
version_number = "1.4.2"
side = "server"
`
	const validToken = "secret-diff-token"

	// Setup mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lock" {
			http.NotFound(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+validToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(remoteLockTOML))
	}))
	defer ts.Close()

	tempDir := t.TempDir()
	localLockPath := filepath.Join(tempDir, "cmm.lock")
	localLockTOML := `
[[mods]]
slug = "fabric-api"
name = "Fabric API"
version_number = "0.92.0"
side = "both"

[[mods]]
slug = "iris"
name = "Iris"
version_number = "1.7.0"
side = "client"
`
	if err := os.WriteFile(localLockPath, []byte(localLockTOML), 0644); err != nil {
		t.Fatalf("failed to write local lockfile: %v", err)
	}

	engine := NewDiffEngine()

	// 1. Success with valid token
	res, err := engine.CompareRemote(localLockPath, ts.URL, validToken)
	if err != nil {
		t.Fatalf("CompareRemote error: %v", err)
	}
	if res.Target != ts.URL {
		t.Errorf("expected target=%s, got %s", ts.URL, res.Target)
	}
	if res.Total != 3 {
		t.Errorf("expected Total=3, got %d", res.Total)
	}
	if res.Synchronized != 1 || res.ClientOnly != 1 || res.ServerOnly != 1 {
		t.Errorf("unexpected counts: %+v", res)
	}

	// 2. Unauthorized error with invalid token
	_, err = engine.CompareRemote(localLockPath, ts.URL, "wrong-token")
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("expected unauthorized error, got: %v", err)
	}

	// 3. Fallback when local lockfile does not exist (e.g. clean client)
	nonExistentLock := filepath.Join(tempDir, "missing.lock")
	resEmptyLocal, err := engine.CompareRemote(nonExistentLock, ts.URL, validToken)
	if err != nil {
		t.Fatalf("CompareRemote with non-existent local lock failed: %v", err)
	}
	if resEmptyLocal.Total != 2 {
		t.Errorf("expected 2 server mods, got %d", resEmptyLocal.Total)
	}
	if resEmptyLocal.ServerOnly != 1 || resEmptyLocal.Missing != 1 {
		t.Errorf("expected 1 server-only and 1 missing (fabric-api), got: %+v", resEmptyLocal)
	}

	// 4. Empty URL error
	_, err = engine.CompareRemote(localLockPath, "", validToken)
	if err == nil {
		t.Errorf("expected error for empty URL")
	}

	// 5. Connection refused / bad server
	_, err = engine.CompareRemote(localLockPath, "http://127.0.0.1:1", validToken)
	if err == nil {
		t.Errorf("expected connection error for unreachable host")
	}
}

func TestDiffEngine_DisabledStatusComparison(t *testing.T) {
	leftLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "mod-disabled-locally",
				Name:          "Mod Disabled Locally",
				VersionNumber: "1.0.0",
				Disabled:      true,
			},
			{
				Slug:          "mod-enabled-locally",
				Name:          "Mod Enabled Locally",
				VersionNumber: "1.0.0",
				Disabled:      false,
			},
			{
				Slug:          "mod-both-disabled",
				Name:          "Mod Both Disabled",
				VersionNumber: "1.0.0",
				Disabled:      true,
			},
		},
	}

	rightLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "mod-disabled-locally",
				Name:          "Mod Disabled Locally",
				VersionNumber: "1.0.0",
				Disabled:      false,
			},
			{
				Slug:          "mod-enabled-locally",
				Name:          "Mod Enabled Locally",
				VersionNumber: "1.0.0",
				Disabled:      true,
			},
			{
				Slug:          "mod-both-disabled",
				Name:          "Mod Both Disabled",
				VersionNumber: "1.0.0",
				Disabled:      true,
			},
		},
	}

	res := CompareLockfiles(leftLock, rightLock)

	if res.Mismatches != 2 {
		t.Errorf("expected 2 mismatches from status discrepancy, got %d", res.Mismatches)
	}
	if res.Synchronized != 1 {
		t.Errorf("expected 1 synchronized (both disabled), got %d", res.Synchronized)
	}

	entryMap := make(map[string]ModDiffEntry)
	for _, e := range res.Entries {
		entryMap[e.Slug] = e
	}

	// 1. Disabled locally, enabled remotely
	e1 := entryMap["mod-disabled-locally"]
	if e1.Category != DiffMismatch || e1.Status != "[MISMATCH]" || !strings.Contains(e1.Notes, "disabled locally, enabled remotely") {
		t.Errorf("unexpected entry for mod-disabled-locally: %+v", e1)
	}
	if !e1.LocalDisabled || e1.RemoteDisabled {
		t.Errorf("expected LocalDisabled=true, RemoteDisabled=false, got %+v", e1)
	}

	// 2. Enabled locally, disabled remotely
	e2 := entryMap["mod-enabled-locally"]
	if e2.Category != DiffMismatch || e2.Status != "[MISMATCH]" || !strings.Contains(e2.Notes, "enabled locally, disabled remotely") {
		t.Errorf("unexpected entry for mod-enabled-locally: %+v", e2)
	}
	if e2.LocalDisabled || !e2.RemoteDisabled {
		t.Errorf("expected LocalDisabled=false, RemoteDisabled=true, got %+v", e2)
	}

	// 3. Both disabled
	e3 := entryMap["mod-both-disabled"]
	if e3.Category != DiffOK || e3.Status != "[OK]" || !strings.Contains(e3.Notes, "Synchronized") {
		t.Errorf("unexpected entry for mod-both-disabled: %+v", e3)
	}
}

func TestDiffEngine_SHA512ChecksumComparison(t *testing.T) {
	leftLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "mod-hash-mismatch",
				Name:          "Mod Hash Mismatch",
				VersionNumber: "1.0.0",
				SHA512:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			{
				Slug:          "mod-hash-match",
				Name:          "Mod Hash Match",
				VersionNumber: "1.0.0",
				SHA512:        "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
		},
	}

	rightLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "mod-hash-mismatch",
				Name:          "Mod Hash Mismatch",
				VersionNumber: "1.0.0",
				SHA512:        "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			},
			{
				Slug:          "mod-hash-match",
				Name:          "Mod Hash Match",
				VersionNumber: "1.0.0",
				SHA512:        "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
		},
	}

	res := CompareLockfiles(leftLock, rightLock)

	if res.Mismatches != 1 {
		t.Errorf("expected 1 mismatch from hash discrepancy, got %d", res.Mismatches)
	}
	if res.Synchronized != 1 {
		t.Errorf("expected 1 synchronized from matching hash, got %d", res.Synchronized)
	}

	entryMap := make(map[string]ModDiffEntry)
	for _, e := range res.Entries {
		entryMap[e.Slug] = e
	}

	mismatch := entryMap["mod-hash-mismatch"]
	if mismatch.Category != DiffMismatch || mismatch.Status != "[MISMATCH]" || !strings.Contains(mismatch.Notes, "Checksum mismatch") {
		t.Errorf("unexpected entry for mod-hash-mismatch: %+v", mismatch)
	}

	match := entryMap["mod-hash-match"]
	if match.Category != DiffOK || match.Status != "[OK]" || !strings.Contains(match.Notes, "Synchronized") {
		t.Errorf("unexpected entry for mod-hash-match: %+v", match)
	}
}

func TestDiffEngine_SideNormalizationVariants(t *testing.T) {
	leftLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{Slug: "mod-client-opt", Name: "Client Opt", Side: "Client (Server Opt.)"},
			{Slug: "mod-client-only-opt", Name: "Client Only Opt", Side: "Client Only (Opt.)"},
		},
	}
	rightLock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{Slug: "mod-server-opt", Name: "Server Opt", Side: "Server (Client Opt.)"},
			{Slug: "mod-server-only-opt", Name: "Server Only Opt", Side: "Server Only (Opt.)"},
		},
	}

	res := CompareLockfiles(leftLock, rightLock)

	if res.ClientOnly != 2 {
		t.Errorf("expected 2 client-only mods recognized from variants, got %d (missing: %d)", res.ClientOnly, res.Missing)
	}
	if res.ServerOnly != 2 {
		t.Errorf("expected 2 server-only mods recognized from variants, got %d (missing: %d)", res.ServerOnly, res.Missing)
	}
	if res.Missing != 0 {
		t.Errorf("expected 0 missing, got %d: %+v", res.Missing, res.Entries)
	}
}
