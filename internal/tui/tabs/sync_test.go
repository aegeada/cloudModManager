package tabs

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"cmm/internal/config"
	"cmm/internal/launcher"
	"cmm/internal/modrinth"
	"cmm/internal/sync"
	"cmm/internal/tui/styles"
	"cmm/internal/tui/tea"
)

func setupTestSyncTab(t *testing.T) (SyncModel, string, string) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cmm.toml")
	lockPath := filepath.Join(tmpDir, "cmm.lock")

	cfg := config.DefaultConfig()
	_ = config.SaveConfig(cfgPath, cfg)
	_ = config.SaveLockfile(lockPath, &config.Lockfile{Mods: []config.LockfileMod{}})

	client, _ := modrinth.NewClient("cmm-test/1.0.0")
	model := NewSyncModel(client, cfgPath, lockPath, cfg)
	return model, cfgPath, lockPath
}

func TestSyncTab_EngineSelection(t *testing.T) {
	m, _, _ := setupTestSyncTab(t)

	if m.SelectedEngine != EngineLocal {
		t.Errorf("Expected initial engine EngineLocal (0), got %v", m.SelectedEngine)
	}

	// Press '2' -> Modpack Sync
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if m.SelectedEngine != EngineModpack {
		t.Errorf("Expected engine EngineModpack (1), got %v", m.SelectedEngine)
	}

	// Press '3' -> GitHub Sync
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if m.SelectedEngine != EngineGitHub {
		t.Errorf("Expected engine EngineGitHub (2), got %v", m.SelectedEngine)
	}

	// Press '4' -> Remote Pull
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	if m.SelectedEngine != EngineRemote {
		t.Errorf("Expected engine EngineRemote (3), got %v", m.SelectedEngine)
	}

	// Press '5' -> Remote Push
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	if m.SelectedEngine != EnginePush {
		t.Errorf("Expected engine EnginePush (4), got %v", m.SelectedEngine)
	}

	// Press '6' -> Diff Inspector
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'6'}})
	if m.SelectedEngine != EngineDiff {
		t.Errorf("Expected engine EngineDiff (5), got %v", m.SelectedEngine)
	}

	// Press '7' -> Launcher Sync
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'7'}})
	if m.SelectedEngine != EngineLauncher {
		t.Errorf("Expected engine EngineLauncher (6), got %v", m.SelectedEngine)
	}

	// Check Titles
	for eng := EngineLocal; eng < EngineCount; eng++ {
		title := eng.Title()
		if title == "" || title == "Unknown" {
			t.Errorf("Expected non-empty title for engine %v, got %q", eng, title)
		}
	}
}

func TestSyncTab_TriggerAndStreamOutput(t *testing.T) {
	m, _, _ := setupTestSyncTab(t)

	// Simulate sync completion with delta results
	syncResult := &sync.SyncResult{
		AddedMods:   []string{"fabric-api", "sodium"},
		UpdatedMods: []string{"lithium"},
		RemovedMods: []string{"old-mod"},
		UnknownJars: []string{"custom-unmanaged.jar"},
		Message:     "Sync applied successfully",
	}

	m, _ = m.Update(SyncCompleteMsg{
		Engine: EngineLocal,
		Result: syncResult,
	})

	if m.IsSyncing {
		t.Errorf("Expected IsSyncing to be false after completion")
	}

	view := styles.StripANSI(m.View())
	if !strings.Contains(view, "Sync completed successfully") {
		t.Errorf("Expected sync complete status in view, got:\n%s", view)
	}
	if !strings.Contains(view, "fabric-api") || !strings.Contains(view, "sodium") {
		t.Errorf("Expected added mods in log view, got:\n%s", view)
	}
	if !strings.Contains(view, "custom-unmanaged.jar") {
		t.Errorf("Expected unknown jar in log view, got:\n%s", view)
	}
}

func TestSyncTab_RemotePush_Workflow(t *testing.T) {
	m, _, _ := setupTestSyncTab(t)

	// Switch to Push (5)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	if m.SelectedEngine != EnginePush {
		t.Fatalf("Expected EnginePush, got %v", m.SelectedEngine)
	}

	// Initial push inputs
	if m.PushRemoteURL.Value() != "http://localhost:8080" {
		t.Errorf("Expected default push URL, got %q", m.PushRemoteURL.Value())
	}
	if !m.PushIncludeConfig {
		t.Errorf("Expected PushIncludeConfig to default to true")
	}
	if m.PushDryRun {
		t.Errorf("Expected PushDryRun to default to false")
	}

	// Tab navigation: 0 -> 1 -> 2 (IncludeConfig) -> 3 (DryRun) -> 4 (Button)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.ActiveInputIdx != 1 {
		t.Errorf("Expected ActiveInputIdx 1, got %d", m.ActiveInputIdx)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.ActiveInputIdx != 2 {
		t.Errorf("Expected ActiveInputIdx 2, got %d", m.ActiveInputIdx)
	}

	// Space on IncludeConfig toggles it to false
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if m.PushIncludeConfig {
		t.Errorf("Expected PushIncludeConfig to be false after Space toggle")
	}

	// Move to DryRun (idx 3) and toggle with Space
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.ActiveInputIdx != 3 {
		t.Errorf("Expected ActiveInputIdx 3, got %d", m.ActiveInputIdx)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if !m.PushDryRun {
		t.Errorf("Expected PushDryRun to be true after Space toggle")
	}

	// Shift+Tab backwards
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.ActiveInputIdx != 2 {
		t.Errorf("Expected ActiveInputIdx 2 after Shift+Tab, got %d", m.ActiveInputIdx)
	}

	// Test PushCompleteMsg (Success)
	pushResult := &sync.PushResult{
		Success:        true,
		Message:        "Delta sync applied on remote server",
		AddedMods:      []string{"iris.jar", "sodium.jar"},
		UpdatedMods:    []string{"fabric-api.jar"},
		PrunedMods:     []string{"old-server-mod.jar"},
		ConfigsUpdated: 4,
	}

	m, _ = m.Update(PushCompleteMsg{
		Result: pushResult,
	})

	if m.IsSyncing {
		t.Errorf("Expected IsSyncing to be false")
	}
	if m.StatusIsError {
		t.Errorf("Expected StatusIsError false on successful push")
	}

	view := styles.StripANSI(m.View())
	if !strings.Contains(view, "Remote push completed successfully") {
		t.Errorf("Expected success status in view, got:\n%s", view)
	}
	if !strings.Contains(view, "iris.jar") || !strings.Contains(view, "Configs Updated: 4") {
		t.Errorf("Expected push details in view, got:\n%s", view)
	}

	// Test PushCompleteMsg (Error)
	m, _ = m.Update(PushCompleteMsg{
		Err: fmt.Errorf("remote daemon unreachable at http://localhost:8080"),
	})
	if !m.StatusIsError {
		t.Errorf("Expected StatusIsError true on push error")
	}
	viewErr := styles.StripANSI(m.View())
	if !strings.Contains(viewErr, "Push failed") {
		t.Errorf("Expected failure message in view, got:\n%s", viewErr)
	}
}

func TestSyncTab_DiffInspector_Workflow(t *testing.T) {
	m, _, _ := setupTestSyncTab(t)

	// Switch to Diff (6)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'6'}})
	if m.SelectedEngine != EngineDiff {
		t.Fatalf("Expected EngineDiff, got %v", m.SelectedEngine)
	}

	// Test Tab cycling in Diff view
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // 1: Token
	if m.ActiveInputIdx != 1 {
		t.Errorf("Expected ActiveInputIdx 1, got %d", m.ActiveInputIdx)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // 2: CompareFile
	if m.ActiveInputIdx != 2 {
		t.Errorf("Expected ActiveInputIdx 2, got %d", m.ActiveInputIdx)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // 3: Run button
	if m.ActiveInputIdx != 3 {
		t.Errorf("Expected ActiveInputIdx 3, got %d", m.ActiveInputIdx)
	}

	// Test DiffCompleteMsg with all 5 diff categories
	diffResult := &sync.DiffResult{
		Target:       "http://localhost:8080 vs cmm.lock",
		Total:        5,
		Synchronized: 1,
		Mismatches:   1,
		ClientOnly:   1,
		ServerOnly:   1,
		Missing:      1,
		Entries: []sync.ModDiffEntry{
			{
				Slug:          "fabric-api",
				Name:          "Fabric API",
				Category:      sync.DiffOK,
				Status:        "[OK]",
				LocalVersion:  "0.92.0",
				RemoteVersion: "0.92.0",
				Notes:         "Synchronized",
			},
			{
				Slug:          "sodium",
				Name:          "Sodium",
				Category:      sync.DiffMismatch,
				Status:        "[MISMATCH]",
				LocalVersion:  "0.5.8",
				RemoteVersion: "0.5.3",
				Notes:         "Version mismatch",
			},
			{
				Slug:         "iris",
				Name:         "Iris Shaders",
				Category:     sync.DiffClient,
				Status:       "[CLIENT]",
				LocalVersion: "1.7.0",
				Notes:        "Client-only mod",
			},
			{
				Slug:          "chunky",
				Name:          "Chunky",
				Category:      sync.DiffServer,
				Status:        "[SERVER]",
				RemoteVersion: "1.3.0",
				Notes:         "Server-only mod",
			},
			{
				Slug:     "lithium",
				Name:     "Lithium",
				Category: sync.DiffMissing,
				Status:   "[MISSING]",
				Notes:    "Missing on server",
			},
		},
	}

	m, _ = m.Update(DiffCompleteMsg{
		Result: diffResult,
	})

	if m.IsSyncing {
		t.Errorf("Expected IsSyncing to be false")
	}
	if m.DiffResult == nil || m.DiffResult.Total != 5 {
		t.Errorf("Expected DiffResult with 5 items, got %+v", m.DiffResult)
	}
	if m.ActiveInputIdx != 4 {
		t.Errorf("Expected DiffTable to be focused (idx 4), got %d", m.ActiveInputIdx)
	}

	view := styles.StripANSI(m.View())
	if !strings.Contains(view, "Client vs Server Difference Audit") {
		t.Errorf("Expected table header in view, got:\n%s", view)
	}
	if !strings.Contains(view, "Fabric API") || !strings.Contains(view, "Sodium") || !strings.Contains(view, "Iris Shaders") {
		t.Errorf("Expected mod names in view table, got:\n%s", view)
	}
	if !strings.Contains(view, "5 total mods") || !strings.Contains(view, "1 synchronized") {
		t.Errorf("Expected summary in view, got:\n%s", view)
	}

	// Test table navigation
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.DiffTable.Cursor() != 1 {
		t.Errorf("Expected table cursor 1, got %d", m.DiffTable.Cursor())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.DiffTable.Cursor() != 0 {
		t.Errorf("Expected table cursor 0, got %d", m.DiffTable.Cursor())
	}

	// Test DiffCompleteMsg (Error)
	m, _ = m.Update(DiffCompleteMsg{
		Err: fmt.Errorf("diff connection timeout"),
	})
	if !m.StatusIsError {
		t.Errorf("Expected StatusIsError true on diff failure")
	}
}

func TestSyncTab_LauncherTools_Workflow(t *testing.T) {
	m, _, _ := setupTestSyncTab(t)

	// Switch to Launcher (7)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'7'}})
	if m.SelectedEngine != EngineLauncher {
		t.Fatalf("Expected EngineLauncher, got %v", m.SelectedEngine)
	}

	// Simulate detected instances
	mockInstances := []launcher.Instance{
		{
			ID:               "prism-main",
			Name:             "Prism-Fabric-1.21.1",
			Launcher:         launcher.LauncherPrism,
			LauncherName:     "Prism Launcher",
			MinecraftVersion: "1.21.1",
			Loader:           "fabric",
			LoaderVersion:    "0.16.5",
			InstanceDir:      "/home/user/.local/share/PrismLauncher/instances/test",
			ModsDir:          "/home/user/.local/share/PrismLauncher/instances/test/minecraft/mods",
		},
		{
			ID:               "vanilla-main",
			Name:             "Vanilla-Profile",
			Launcher:         launcher.LauncherVanilla,
			LauncherName:     "Official Minecraft",
			MinecraftVersion: "1.21.1",
			Loader:           "vanilla",
			InstanceDir:      "/home/user/.minecraft",
			ModsDir:          "/home/user/.minecraft/mods",
		},
	}

	m, _ = m.Update(LaunchersDetectedMsg{
		Instances: mockInstances,
	})

	if len(m.LauncherInstances) != 2 {
		t.Fatalf("Expected 2 launcher instances, got %d", len(m.LauncherInstances))
	}
	if len(m.LauncherTable.Rows) != 2 {
		t.Fatalf("Expected 2 rows in LauncherTable, got %d", len(m.LauncherTable.Rows))
	}

	// Test selecting instance and options toggle
	if inst := m.SelectedLauncherInstance(); inst == nil || inst.Name != "Prism-Fabric-1.21.1" {
		t.Errorf("Expected selected instance Prism-Fabric-1.21.1, got %+v", inst)
	}

	// Toggle force ('f') and dry run ('d')
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if !m.LauncherForce {
		t.Errorf("Expected LauncherForce to be true after 'f'")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if !m.LauncherDryRun {
		t.Errorf("Expected LauncherDryRun to be true after 'd'")
	}

	// Test LauncherSyncCompleteMsg (Success)
	syncResult := &launcher.LauncherSyncResult{
		Instance:    mockInstances[0],
		AddedMods:   []string{"fabric-api.jar", "sodium.jar"},
		UpdatedMods: []string{"lithium.jar"},
		RemovedMods: []string{"old.jar"},
		SkippedMods: []string{"server-only-mod.jar"},
		UpToDate:    false,
		Message:     "Modpack synchronized successfully",
	}

	m, _ = m.Update(LauncherSyncCompleteMsg{
		Result: syncResult,
	})

	if m.IsSyncing {
		t.Errorf("Expected IsSyncing false after sync completion")
	}
	if m.StatusIsError {
		t.Errorf("Expected StatusIsError false")
	}

	view := styles.StripANSI(m.View())
	if !strings.Contains(view, "Prism-Fabric-1.21.1") {
		t.Errorf("Expected instance name in view, got:\n%s", view)
	}
	if !strings.Contains(view, "fabric-api.jar") {
		t.Errorf("Expected sync log in view, got:\n%s", view)
	}
	m.LogsViewport.GotoTop()
	topView := styles.StripANSI(m.LogsViewport.View())
	if !strings.Contains(topView, "Launcher Sync:") {
		t.Errorf("Expected Launcher Sync header in viewport, got:\n%s", topView)
	}

	// Test LauncherSyncCompleteMsg (Error)
	m, _ = m.Update(LauncherSyncCompleteMsg{
		Err: fmt.Errorf("incompatible minecraft version"),
	})
	if !m.StatusIsError {
		t.Errorf("Expected StatusIsError true on sync error")
	}
	viewErr := styles.StripANSI(m.View())
	if !strings.Contains(viewErr, "Launcher sync failed") {
		t.Errorf("Expected error message in view, got:\n%s", viewErr)
	}
}

func TestSyncTab_IsTypingAndHandlingTab(t *testing.T) {
	m, _, _ := setupTestSyncTab(t)

	// EngineLocal
	m.SelectedEngine = EngineLocal
	if m.IsTypingInput() {
		t.Errorf("EngineLocal should not be typing")
	}
	if m.IsHandlingTab() {
		t.Errorf("EngineLocal should not handle tab")
	}

	// EngineModpack
	m.SelectedEngine = EngineModpack
	if !m.IsTypingInput() {
		t.Errorf("EngineModpack should be typing")
	}
	if !m.IsHandlingTab() {
		t.Errorf("EngineModpack should handle tab")
	}

	// EnginePush
	m.SelectedEngine = EnginePush
	m.ActiveInputIdx = 0 // URL input
	if !m.IsTypingInput() {
		t.Errorf("EnginePush index 0 should be typing")
	}
	m.ActiveInputIdx = 2 // Checkbox
	if m.IsTypingInput() {
		t.Errorf("EnginePush index 2 should NOT be typing")
	}
	if !m.IsHandlingTab() {
		t.Errorf("EnginePush should handle tab")
	}

	// EngineDiff
	m.SelectedEngine = EngineDiff
	m.ActiveInputIdx = 0 // URL input
	if !m.IsTypingInput() {
		t.Errorf("EngineDiff index 0 should be typing")
	}
	m.ActiveInputIdx = 4 // Table
	if m.IsTypingInput() {
		t.Errorf("EngineDiff index 4 (table) should NOT be typing")
	}
	if !m.IsHandlingTab() {
		t.Errorf("EngineDiff should handle tab")
	}

	// EngineLauncher
	m.SelectedEngine = EngineLauncher
	if m.IsTypingInput() {
		t.Errorf("EngineLauncher should NOT be typing")
	}
	if m.IsHandlingTab() {
		t.Errorf("EngineLauncher should NOT handle tab")
	}
}

func TestSyncTab_WindowResizeAndNoPanic(t *testing.T) {
	m, _, _ := setupTestSyncTab(t)

	sizes := [][2]int{
		{120, 40},
		{80, 24},
		{50, 15},
		{30, 8},
	}

	for _, sz := range sizes {
		m, _ = m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})

		for eng := EngineLocal; eng < EngineCount; eng++ {
			m.SelectedEngine = eng
			view := m.View()
			if len(view) == 0 {
				t.Errorf("View should not be empty for size %dx%d, engine %v", sz[0], sz[1], eng)
			}
		}
	}
}

