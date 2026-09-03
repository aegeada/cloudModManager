package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLockfile_SaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cmm-lockfile-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "cmm.lock")

	lock := &Lockfile{
		Mods: []LockfileMod{
			{
				Slug:          "sodium",
				Name:          "Sodium",
				Source:        "modrinth",
				ProjectID:     "AANobbMI",
				VersionID:     "v123",
				VersionNumber: "0.5.8",
				FileName:      "sodium-fabric-0.5.8.jar",
				SHA512:        "abcd512",
				DownloadURL:   "https://cdn.modrinth.com/data/AANobbMI/versions/v123/sodium-fabric-0.5.8.jar",
				Side:          "client",
				Pinned:        false,
			},
		},
	}

	err = SaveLockfile(path, lock)
	if err != nil {
		t.Fatalf("SaveLockfile failed: %v", err)
	}

	loaded, err := LoadLockfile(path)
	if err != nil {
		t.Fatalf("LoadLockfile failed: %v", err)
	}

	if len(loaded.Mods) != 1 {
		t.Fatalf("expected 1 mod, got %d", len(loaded.Mods))
	}
	if loaded.Mods[0].Slug != "sodium" {
		t.Errorf("expected sodium, got %s", loaded.Mods[0].Slug)
	}

	// Test helper methods
	mod := loaded.GetMod("sodium")
	if mod == nil || mod.ProjectID != "AANobbMI" {
		t.Errorf("GetMod failed: %+v", mod)
	}

	loaded.SetPinned("sodium", true)
	if !loaded.IsPinned("sodium") {
		t.Errorf("expected sodium to be pinned")
	}

	loaded.AddOrUpdateMod(LockfileMod{
		Slug:      "lithium",
		Name:      "Lithium",
		ProjectID: "gvQqBUqZ",
	})
	if len(loaded.Mods) != 2 {
		t.Errorf("expected 2 mods after add, got %d", len(loaded.Mods))
	}

	removed := loaded.RemoveMod("sodium")
	if !removed || len(loaded.Mods) != 1 || loaded.Mods[0].Slug != "lithium" {
		t.Errorf("expected sodium removed, got %+v", loaded.Mods)
	}
}

func TestLockfile_VersionTagAndNameMatching(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cmm-lockfile-version-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "cmm.lock")
	content := `
[[mods]]
name = "sodium"
version = "0.5.0"
pinned = false
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadLockfile(path)
	if err != nil {
		t.Fatalf("LoadLockfile failed: %v", err)
	}

	if len(loaded.Mods) != 1 {
		t.Fatalf("expected 1 mod, got %d", len(loaded.Mods))
	}

	mod := loaded.GetMod("sodium")
	if mod == nil {
		t.Fatalf("GetMod by name failed")
	}
	if mod.GetVersion() != "0.5.0" {
		t.Errorf("expected version 0.5.0, got %s", mod.GetVersion())
	}

	if !loaded.SetPinned("sodium", true) {
		t.Errorf("SetPinned by name failed")
	}
	if !loaded.IsPinned("sodium") {
		t.Errorf("expected is pinned true")
	}

	if !loaded.RemoveMod("sodium") {
		t.Errorf("RemoveMod by name failed")
	}
	if len(loaded.Mods) != 0 {
		t.Errorf("expected 0 mods, got %d", len(loaded.Mods))
	}
}

func TestLockfile_MapOfTablesAndEmptyParsing(t *testing.T) {
	mapContent := `
[mods.sodium]
name = "Sodium"
version = "0.5.8"
pinned = true

[mods.lithium]
pinned = false
`
	lock, err := DecodeLockfile(mapContent)
	if err != nil {
		t.Fatalf("DecodeLockfile failed for map format: %v", err)
	}
	if len(lock.Mods) != 2 {
		t.Fatalf("expected 2 mods, got %d", len(lock.Mods))
	}
	sMod := lock.GetMod("sodium")
	if sMod == nil || !sMod.Pinned || sMod.GetVersion() != "0.5.8" {
		t.Errorf("unexpected sodium mod: %+v", sMod)
	}
	lMod := lock.GetMod("lithium")
	if lMod == nil || lMod.Pinned {
		t.Errorf("unexpected lithium mod: %+v", lMod)
	}

	emptyLock, err := DecodeLockfile("[mods]")
	if err != nil {
		t.Fatalf("DecodeLockfile failed for empty table: %v", err)
	}
	if len(emptyLock.Mods) != 0 {
		t.Errorf("expected 0 mods, got %d", len(emptyLock.Mods))
	}

	blankLock, err := DecodeLockfile("")
	if err != nil {
		t.Fatalf("DecodeLockfile failed for empty string: %v", err)
	}
	if len(blankLock.Mods) != 0 {
		t.Errorf("expected 0 mods, got %d", len(blankLock.Mods))
	}
}

func TestDetermineSide_And_NormalizeSide(t *testing.T) {
	tests := []struct {
		clientSide string
		serverSide string
		expected   string
	}{
		{"required", "required", "Client & Server"},
		{"optional", "required", "Server (Client Opt.)"},
		{"required", "optional", "Client (Server Opt.)"},
		{"optional", "optional", "Client & Server (Opt.)"},
		{"required", "unsupported", "Client Only"},
		{"optional", "unsupported", "Client Only (Opt.)"},
		{"unsupported", "required", "Server Only"},
		{"unsupported", "optional", "Server Only (Opt.)"},
		{"", "", "Client & Server"},
	}

	for _, tt := range tests {
		got := DetermineSide(tt.clientSide, tt.serverSide)
		if got != tt.expected {
			t.Errorf("DetermineSide(%q, %q) = %q, expected %q", tt.clientSide, tt.serverSide, got, tt.expected)
		}
	}

	normTests := []struct {
		input    string
		expected string
	}{
		{"required", "Client & Server"},
		{"optional", "Client & Server (Opt.)"},
		{"both", "Client & Server"},
		{"client", "Client Only"},
		{"client-only", "Client Only"},
		{"client only (opt.)", "Client Only (Opt.)"},
		{"server", "Server Only"},
		{"server-only", "Server Only"},
		{"server only (opt.)", "Server Only (Opt.)"},
		{"server (client opt.)", "Server (Client Opt.)"},
		{"client (server opt.)", "Client (Server Opt.)"},
		{"", "Client & Server"},
	}

	for _, nt := range normTests {
		got := NormalizeSide(nt.input)
		if got != nt.expected {
			t.Errorf("NormalizeSide(%q) = %q, expected %q", nt.input, got, nt.expected)
		}
	}
}

func TestExtractModSlug_DisabledAndComplexFilenames(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sodium.jar", "sodium"},
		{"sodium.jar.disabled", "sodium"},
		{"sodium-fabric-0.5.8.jar.disabled", "sodium"},
		{"fabric-api-0.92.0+1.21.jar.disabled", "fabric-api"},
		{"appleskin-mc1.20.1-fabric-2.5.1.jar.disabled", "appleskin"},
		{"appleskin.jar.disabled", "appleskin"},
		{"iris.jar.disabled", "iris"},
		{"iris.disabled", "iris"},
		{"Sodium-Fabric-0.5.8.JAR.DISABLED", "sodium"},
		{"mods/subdir/lithium-mc1.21-0.12.0.jar.disabled", "lithium"},
		{"forge-config-api-port-fabric-10.0.0.jar.disabled", "forge-config-api-port"},
		{"mod.jar.disabled.disabled", "mod"},
		{"", ""},
	}

	for _, tt := range tests {
		got := ExtractModSlug(tt.input)
		if got != tt.expected {
			t.Errorf("ExtractModSlug(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestLockfile_Disabled_TOMLAndJSONSerialization(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cmm-lockfile-disabled-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "cmm.lock")

	lock := &Lockfile{
		Mods: []LockfileMod{
			{
				Slug:     "sodium",
				Name:     "Sodium",
				FileName: "sodium-0.5.8.jar",
				Disabled: false,
			},
			{
				Slug:     "lithium",
				Name:     "Lithium",
				FileName: "lithium-0.11.0.jar.disabled",
				Disabled: true,
			},
		},
	}

	err = SaveLockfile(path, lock)
	if err != nil {
		t.Fatalf("SaveLockfile failed: %v", err)
	}

	rawBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	rawContent := string(rawBytes)

	// In TOML, Disabled: true should be written as `disabled = true`
	if !strings.Contains(rawContent, "disabled = true") {
		t.Errorf("expected TOML to contain 'disabled = true', got:\n%s", rawContent)
	}
	// In TOML, Disabled: false should be omitted (omitempty)
	if strings.Contains(rawContent, "disabled = false") {
		t.Errorf("expected TOML to omit 'disabled = false', got:\n%s", rawContent)
	}

	// Load back and verify roundtrip
	loaded, err := LoadLockfile(path)
	if err != nil {
		t.Fatalf("LoadLockfile failed: %v", err)
	}
	if len(loaded.Mods) != 2 {
		t.Fatalf("expected 2 mods, got %d", len(loaded.Mods))
	}
	if loaded.IsDisabled("sodium") {
		t.Errorf("expected sodium disabled == false")
	}
	if !loaded.IsDisabled("lithium") {
		t.Errorf("expected lithium disabled == true")
	}

	// Test JSON serialization omitempty
	jsonData, err := json.Marshal(lock.Mods[0])
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if strings.Contains(string(jsonData), `"disabled"`) {
		t.Errorf("expected JSON to omit disabled when false, got %s", string(jsonData))
	}

	jsonDataDisabled, err := json.Marshal(lock.Mods[1])
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !strings.Contains(string(jsonDataDisabled), `"disabled":true`) {
		t.Errorf("expected JSON to contain '\"disabled\":true', got %s", string(jsonDataDisabled))
	}
}

func TestLockfile_Disabled_MapAndArrayTOMLParsing(t *testing.T) {
	arrayContent := `
[[mods]]
slug = "sodium"
name = "Sodium"
disabled = true

[[mods]]
slug = "lithium"
name = "Lithium"
`
	arrLock, err := DecodeLockfile(arrayContent)
	if err != nil {
		t.Fatalf("DecodeLockfile array failed: %v", err)
	}
	if !arrLock.IsDisabled("sodium") {
		t.Errorf("expected sodium disabled in array format")
	}
	if arrLock.IsDisabled("lithium") {
		t.Errorf("expected lithium active in array format")
	}

	mapContent := `
[mods.sodium]
name = "Sodium"
disabled = true

[mods.lithium]
name = "Lithium"
`
	mapLock, err := DecodeLockfile(mapContent)
	if err != nil {
		t.Fatalf("DecodeLockfile map failed: %v", err)
	}
	if !mapLock.IsDisabled("sodium") {
		t.Errorf("expected sodium disabled in map format")
	}
	if mapLock.IsDisabled("lithium") {
		t.Errorf("expected lithium active in map format")
	}
}

func TestLockfile_IsDisabled_SetDisabled_HelperMethods(t *testing.T) {
	lock := &Lockfile{
		Mods: []LockfileMod{
			{
				Slug:      "sodium",
				Name:      "Sodium",
				ProjectID: "AANobbMI",
				FileName:  "sodium-fabric-0.5.8.jar",
				Disabled:  false,
			},
		},
	}

	// Query by slug, projectID, filename
	if lock.IsDisabled("sodium") {
		t.Errorf("expected sodium not disabled initially")
	}
	if lock.IsDisabled("AANobbMI") {
		t.Errorf("expected AANobbMI not disabled initially")
	}

	// SetDisabled
	if !lock.SetDisabled("sodium", true) {
		t.Errorf("SetDisabled failed")
	}
	if !lock.IsDisabled("sodium") {
		t.Errorf("expected sodium disabled = true")
	}
	if !lock.IsDisabled("sodium-fabric-0.5.8.jar.disabled") {
		t.Errorf("expected match with .jar.disabled filename")
	}

	// Set back to false
	if !lock.SetDisabled("AANobbMI", false) {
		t.Errorf("SetDisabled by projectID failed")
	}
	if lock.IsDisabled("sodium") {
		t.Errorf("expected sodium disabled = false")
	}

	// Non-existent mod
	if lock.IsDisabled("non-existent") {
		t.Errorf("expected false for non-existent mod")
	}
	if lock.SetDisabled("non-existent", true) {
		t.Errorf("expected false when setting non-existent mod")
	}

	// Nil receiver checks
	var nilLock *Lockfile
	if nilLock.IsDisabled("sodium") {
		t.Errorf("expected false for nil lockfile")
	}
	if nilLock.SetDisabled("sodium", true) {
		t.Errorf("expected false for nil lockfile SetDisabled")
	}
	if nilLock.GetMod("sodium") != nil {
		t.Errorf("expected nil for nil lockfile GetMod")
	}
	if nilLock.IsPinned("sodium") {
		t.Errorf("expected false for nil lockfile IsPinned")
	}
	if nilLock.SetPinned("sodium", true) {
		t.Errorf("expected false for nil lockfile SetPinned")
	}
	if nilLock.RemoveMod("sodium") {
		t.Errorf("expected false for nil lockfile RemoveMod")
	}
}

func TestLockfile_MatchMod_WithDisabledFileNames(t *testing.T) {
	mod := &LockfileMod{
		Slug:      "sodium",
		Name:      "Sodium",
		ProjectID: "AANobbMI",
		FileName:  "sodium-fabric-0.5.8.jar.disabled",
		Disabled:  true,
	}

	queries := []string{
		"sodium",
		"Sodium",
		"AANobbMI",
		"sodium-fabric-0.5.8.jar.disabled",
		"sodium-fabric-0.5.8.jar",
		"sodium-fabric-0.5.8",
	}

	for _, q := range queries {
		if !matchMod(mod, q) {
			t.Errorf("matchMod failed to match %q for mod with disabled filename %q", q, mod.FileName)
		}
	}
}

func TestLockfile_NumberedSlugDisambiguation(t *testing.T) {
	lock := &Lockfile{
		Mods: []LockfileMod{
			{Slug: "mod-01", Name: "Mod 01", ProjectID: "id-01", FileName: "mod-01-1.0.jar"},
			{Slug: "mod-02", Name: "Mod 02", ProjectID: "id-02", FileName: "mod-02-1.0.jar"},
			{Slug: "mod-03", Name: "Mod 03", ProjectID: "id-03", FileName: "mod-03-1.0.jar"},
			{Slug: "mod-10", Name: "Mod 10", ProjectID: "id-10", FileName: "mod-10-1.0.jar"},
			{Slug: "worldedit-cui-2", Name: "WorldEdit CUI 2", ProjectID: "we-2", FileName: "worldedit-cui-2.jar"},
			{Slug: "worldedit-cui-3", Name: "WorldEdit CUI 3", ProjectID: "we-3", FileName: "worldedit-cui-3.jar"},
			{Slug: "jei-1", Name: "JEI 1", ProjectID: "jei-1", FileName: "jei-1.jar"},
			{Slug: "jei-2", Name: "JEI 2", ProjectID: "jei-2", FileName: "jei-2.jar"},
		},
	}

	testCases := []struct {
		query        string
		expectedSlug string
	}{
		{"mod-01", "mod-01"},
		{"mod-02", "mod-02"},
		{"mod-03", "mod-03"},
		{"mod-10", "mod-10"},
		{"id-02", "mod-02"},
		{"Mod 03", "mod-03"},
		{"mod-02-1.0.jar", "mod-02"},
		{"mod-02-1.0.jar.disabled", "mod-02"},
		{"worldedit-cui-2", "worldedit-cui-2"},
		{"worldedit-cui-3", "worldedit-cui-3"},
		{"jei-1", "jei-1"},
		{"jei-2", "jei-2"},
	}

	for _, tc := range testCases {
		m := lock.GetMod(tc.query)
		if m == nil {
			t.Fatalf("GetMod(%q) returned nil, expected %s", tc.query, tc.expectedSlug)
		}
		if m.Slug != tc.expectedSlug {
			t.Errorf("GetMod(%q) = %q, expected %q (COLLISION DETECTED)", tc.query, m.Slug, tc.expectedSlug)
		}
	}
}

func TestLockfile_TwoPassLookupPriority(t *testing.T) {
	lock := &Lockfile{
		Mods: []LockfileMod{
			{
				Slug:      "sodium",
				Name:      "Sodium",
				ProjectID: "AANobbMI",
				FileName:  "sodium-fabric-0.5.8.jar",
			},
			{
				Slug:      "sodium-extra",
				Name:      "Sodium Extra",
				ProjectID: "extra-id",
				FileName:  "sodium-extra-fabric-0.5.1.jar",
			},
		},
	}

	// 1. Exact query for sodium-extra must NOT return sodium
	extraMod := lock.GetMod("sodium-extra")
	if extraMod == nil || extraMod.Slug != "sodium-extra" {
		t.Fatalf("expected sodium-extra, got %+v", extraMod)
	}

	// 2. Exact query for sodium must NOT return sodium-extra
	sodiumMod := lock.GetMod("sodium")
	if sodiumMod == nil || sodiumMod.Slug != "sodium" {
		t.Fatalf("expected sodium, got %+v", sodiumMod)
	}

	// 3. Negative queries must return nil
	negativeQueries := []string{"", "non-existent", "sodium-reloaded", "extra", "lithium"}
	for _, nq := range negativeQueries {
		if lock.GetMod(nq) != nil {
			t.Errorf("expected GetMod(%q) = nil, got %+v", nq, lock.GetMod(nq))
		}
	}
}

func TestExtractModSlug_DisabledWithoutJarExtension(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ferrite-core+fabric+mc1.21.disabled", "ferrite-core"},
		{"cloth-config-fabric+mc1.21.disabled", "cloth-config"},
		{"sodium-0.5.8.disabled", "sodium"},
		{"iris-mc1.20.1-1.6.4.disabled", "iris"},
		{"appleskin-fabric-mc1.20.4-2.5.1.disabled", "appleskin"},
		{"lithium-mc1.21-0.12.0.disabled", "lithium"},
		{"mod-1.21.1.disabled", "mod"},
		{"mod-v1.0.0.disabled", "mod"},
		{"fabric-api-0.100.0+1.21.disabled", "fabric-api"},
		{"forge-config-api-port-fabric-10.0.0.disabled", "forge-config-api-port"},
	}

	for _, tt := range tests {
		got := ExtractModSlug(tt.input)
		if got != tt.expected {
			t.Errorf("ExtractModSlug(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestLockfile_HelperMethods_NumberedSlugs(t *testing.T) {
	lock := &Lockfile{
		Mods: []LockfileMod{
			{Slug: "mod-01", Name: "Mod 01", FileName: "mod-01.jar", Disabled: false, Pinned: false},
			{Slug: "mod-02", Name: "Mod 02", FileName: "mod-02.jar", Disabled: false, Pinned: false},
		},
	}

	// 1. SetDisabled on mod-02 must NOT affect mod-01
	if !lock.SetDisabled("mod-02", true) {
		t.Fatalf("SetDisabled mod-02 failed")
	}
	if lock.IsDisabled("mod-01") {
		t.Errorf("mod-01 was erroneously marked disabled when disabling mod-02!")
	}
	if !lock.IsDisabled("mod-02") {
		t.Errorf("mod-02 was not marked disabled")
	}

	// 2. SetPinned on mod-02 must NOT affect mod-01
	if !lock.SetPinned("mod-02", true) {
		t.Fatalf("SetPinned mod-02 failed")
	}
	if lock.IsPinned("mod-01") {
		t.Errorf("mod-01 was erroneously pinned when pinning mod-02!")
	}
	if !lock.IsPinned("mod-02") {
		t.Errorf("mod-02 was not pinned")
	}

	// 3. AddOrUpdateMod updating mod-02 must NOT overwrite mod-01
	lock.AddOrUpdateMod(LockfileMod{
		Slug:     "mod-02",
		Name:     "Mod 02 Updated",
		FileName: "mod-02-2.0.jar",
		Disabled: true,
	})
	if len(lock.Mods) != 2 {
		t.Fatalf("expected 2 mods, got %d", len(lock.Mods))
	}
	if lock.Mods[0].Slug != "mod-01" || lock.Mods[0].Name != "Mod 01" {
		t.Errorf("mod-01 was overwritten during mod-02 update: %+v", lock.Mods[0])
	}
	if lock.Mods[1].Slug != "mod-02" || lock.Mods[1].Name != "Mod 02 Updated" {
		t.Errorf("mod-02 was not updated properly: %+v", lock.Mods[1])
	}

	// 4. RemoveMod on mod-02 must leave mod-01 intact
	if !lock.RemoveMod("mod-02") {
		t.Fatalf("RemoveMod mod-02 failed")
	}
	if len(lock.Mods) != 1 || lock.Mods[0].Slug != "mod-01" {
		t.Errorf("expected only mod-01 remaining, got %+v", lock.Mods)
	}
}

