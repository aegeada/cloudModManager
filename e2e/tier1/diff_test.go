package tier1

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"cmm/internal/sync"
)

func TestDiff_RemoteServer(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()

	// 1. Setup Remote Server Daemon
	serverDir := t.TempDir()
	serverPort := getFreePort(t)
	token := "diff-server-token"

	serverLock := `[[mods]]
name = "Sodium"
slug = "sodium"
version = "0.5.8"
side = "both"

[[mods]]
name = "Lithium"
slug = "lithium"
version = "0.11.2"
side = "both"
`
	_ = os.WriteFile(filepath.Join(serverDir, "cmm.toml"), []byte("side = \"server\"\n"), 0644)
	_ = os.WriteFile(filepath.Join(serverDir, "cmm.lock"), []byte(serverLock), 0644)

	_, cleanup := startServeDaemon(t, serverDir, serverPort, token, ms.URL())
	defer cleanup()

	// 2. Setup Client Project
	ctx := harness.NewTestContext(t, ms.URL())
	clientLock := `[[mods]]
name = "Sodium"
slug = "sodium"
version = "0.5.8"
side = "both"

[[mods]]
name = "Lithium"
slug = "lithium"
version = "0.12.0"
side = "both"
`
	ctx.WriteFile("cmm.lock", clientLock)

	// 3. Run cmm diff against remote URL
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", serverPort)
	res := ctx.Run("diff", "--url", serverURL, "--token", token)
	res.AssertSuccess()

	// 4. Assert Output Table
	res.AssertStdoutContains("STATUS", "NAME", "SLUG", "LOCAL", "REMOTE")
	res.AssertStdoutContains("[OK]", "sodium", "0.5.8")
	res.AssertStdoutContains("[MISMATCH]", "lithium", "0.12.0", "0.11.2")
	res.AssertStdoutContains("Summary:")
}

func TestDiff_TwoLockfiles(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	packA := `[[mods]]
name = "Sodium"
slug = "sodium"
version = "0.5.8"
side = "both"

[[mods]]
name = "Iris"
slug = "iris"
version = "1.7.0"
side = "client"
`
	packB := `[[mods]]
name = "Sodium"
slug = "sodium"
version = "0.5.8"
side = "both"

[[mods]]
name = "Iris"
slug = "iris"
version = "1.7.2"
side = "client"
`
	ctx.WriteFile("packA.lock", packA)
	ctx.WriteFile("packB.lock", packB)

	res := ctx.Run("diff", "packA.lock", "packB.lock")
	res.AssertSuccess()
	res.AssertStdoutContains("[OK]", "sodium", "0.5.8")
	res.AssertStdoutContains("[MISMATCH]", "iris", "1.7.0", "1.7.2")
}

func TestDiff_JsonOutput(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	packA := `[[mods]]
name = "Sodium"
slug = "sodium"
version = "0.5.8"
side = "both"

[[mods]]
name = "Lithium"
slug = "lithium"
version = "0.12.0"
side = "both"
`
	packB := `[[mods]]
name = "Sodium"
slug = "sodium"
version = "0.5.8"
side = "both"

[[mods]]
name = "Lithium"
slug = "lithium"
version = "0.11.0"
side = "both"
`
	ctx.WriteFile("a.lock", packA)
	ctx.WriteFile("b.lock", packB)

	res := ctx.Run("diff", "--json", "a.lock", "b.lock")
	res.AssertSuccess()

	var diffResult sync.DiffResult
	if err := json.Unmarshal([]byte(res.Stdout), &diffResult); err != nil {
		t.Fatalf("failed to parse diff JSON output: %v\nOutput: %s", err, res.Stdout)
	}

	if diffResult.Total != 2 {
		t.Errorf("expected 2 total mods, got %d", diffResult.Total)
	}
	if diffResult.Synchronized != 1 {
		t.Errorf("expected 1 synchronized mod, got %d", diffResult.Synchronized)
	}
	if diffResult.Mismatches != 1 {
		t.Errorf("expected 1 mismatch, got %d", diffResult.Mismatches)
	}
	if len(diffResult.Entries) != 2 {
		t.Fatalf("expected 2 entries in JSON, got %d", len(diffResult.Entries))
	}

	foundSodium := false
	foundLithium := false
	for _, entry := range diffResult.Entries {
		if entry.Slug == "sodium" {
			foundSodium = true
			if entry.Status != "[OK]" || entry.Category != sync.DiffOK {
				t.Errorf("expected [OK] for sodium, got %s / %s", entry.Status, entry.Category)
			}
		}
		if entry.Slug == "lithium" {
			foundLithium = true
			if entry.Status != "[MISMATCH]" || entry.Category != sync.DiffMismatch {
				t.Errorf("expected [MISMATCH] for lithium, got %s / %s", entry.Status, entry.Category)
			}
			if entry.LocalVersion != "0.12.0" || entry.RemoteVersion != "0.11.0" {
				t.Errorf("unexpected versions for lithium: local=%s remote=%s", entry.LocalVersion, entry.RemoteVersion)
			}
		}
	}
	if !foundSodium || !foundLithium {
		t.Errorf("missing expected entries in JSON: sodium=%v lithium=%v", foundSodium, foundLithium)
	}
}

func TestDiff_Categories(t *testing.T) {
	ms := mockserver.New()
	defer ms.Close()
	ctx := harness.NewTestContext(t, ms.URL())

	// Client has:
	// 1. sodium 0.5.8 (both) -> matches server 0.5.8 -> [OK]
	// 2. lithium 0.12.0 (both) -> matches server 0.11.0 -> [MISMATCH]
	// 3. iris 1.7.0 (client) -> client only side="client" -> [CLIENT]
	// 4. voicechat 1.0.0 (both) -> missing on server -> [MISSING]
	clientLock := `[[mods]]
name = "Sodium"
slug = "sodium"
version = "0.5.8"
side = "both"

[[mods]]
name = "Lithium"
slug = "lithium"
version = "0.12.0"
side = "both"

[[mods]]
name = "Iris"
slug = "iris"
version = "1.7.0"
side = "client"

[[mods]]
name = "Simple Voice Chat"
slug = "simple-voice-chat"
version = "1.0.0"
side = "both"
`

	// Server has:
	// 1. sodium 0.5.8 (both)
	// 2. lithium 0.11.0 (both)
	// 5. chunky 1.3.0 (server) -> server only side="server" -> [SERVER]
	// 6. dynmap 3.0.0 (both) -> missing on client -> [MISSING]
	serverLock := `[[mods]]
name = "Sodium"
slug = "sodium"
version = "0.5.8"
side = "both"

[[mods]]
name = "Lithium"
slug = "lithium"
version = "0.11.0"
side = "both"

[[mods]]
name = "Chunky"
slug = "chunky"
version = "1.3.0"
side = "server"

[[mods]]
name = "Dynmap"
slug = "dynmap"
version = "3.0.0"
side = "both"
`

	ctx.WriteFile("client.lock", clientLock)
	ctx.WriteFile("server.lock", serverLock)

	// 1. Verify Table Format
	resTable := ctx.Run("diff", "client.lock", "server.lock")
	resTable.AssertSuccess()
	resTable.AssertStdoutContains(
		"[OK]",
		"[MISMATCH]",
		"[CLIENT]",
		"[SERVER]",
		"[MISSING]",
	)
	resTable.AssertStdoutContains("1 synchronized | 1 mismatches | 1 client-only | 1 server-only | 2 missing")

	// 2. Verify JSON Format
	resJSON := ctx.Run("diff", "--json", "client.lock", "server.lock")
	resJSON.AssertSuccess()

	var diffResult sync.DiffResult
	if err := json.Unmarshal([]byte(resJSON.Stdout), &diffResult); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if diffResult.Total != 6 {
		t.Errorf("expected 6 total mods, got %d", diffResult.Total)
	}
	if diffResult.Synchronized != 1 {
		t.Errorf("expected 1 synchronized, got %d", diffResult.Synchronized)
	}
	if diffResult.Mismatches != 1 {
		t.Errorf("expected 1 mismatch, got %d", diffResult.Mismatches)
	}
	if diffResult.ClientOnly != 1 {
		t.Errorf("expected 1 client-only, got %d", diffResult.ClientOnly)
	}
	if diffResult.ServerOnly != 1 {
		t.Errorf("expected 1 server-only, got %d", diffResult.ServerOnly)
	}
	if diffResult.Missing != 2 {
		t.Errorf("expected 2 missing, got %d", diffResult.Missing)
	}
}
