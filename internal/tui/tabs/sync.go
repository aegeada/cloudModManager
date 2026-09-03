package tabs

import (
	"fmt"
	"strings"

	"cmm/internal/config"
	"cmm/internal/launcher"
	"cmm/internal/modrinth"
	"cmm/internal/sync"
	"cmm/internal/tui/components"
	"cmm/internal/tui/styles"
	"cmm/internal/tui/tea"
)

// SyncEngineType identifies one of the 7 sync workflows.
type SyncEngineType int

const (
	EngineLocal SyncEngineType = iota
	EngineModpack
	EngineGitHub
	EngineRemote
	EnginePush
	EngineDiff
	EngineLauncher
	EngineCount
)

func (e SyncEngineType) Title() string {
	switch e {
	case EngineLocal:
		return "1. Local Scan"
	case EngineModpack:
		return "2. Modpack Sync"
	case EngineGitHub:
		return "3. GitHub Sync"
	case EngineRemote:
		return "4. Remote Pull"
	case EnginePush:
		return "5. Remote Push"
	case EngineDiff:
		return "6. Diff Inspector"
	case EngineLauncher:
		return "7. Launcher Sync"
	default:
		return "Unknown"
	}
}

// SyncModel manages Tab 4: Sync, Push, Diff & Launcher Tools Dashboard.
type SyncModel struct {
	Client         *modrinth.Client
	ConfigPath     string
	LockPath       string
	Config         *config.Config
	SelectedEngine SyncEngineType

	// Engine specific inputs (1-4)
	ModpackSlug  components.TextInput
	ModpackPath  components.TextInput
	GitHubRepo   components.TextInput
	GitHubBranch components.TextInput
	GitHubToken  components.TextInput
	RemoteURL    components.TextInput
	RemoteToken  components.TextInput

	// 5. Remote Push inputs & options
	PushRemoteURL     components.TextInput
	PushRemoteToken   components.TextInput
	PushIncludeConfig bool
	PushDryRun        bool

	// 6. Diff Inspector inputs & table
	DiffRemoteURL   components.TextInput
	DiffRemoteToken components.TextInput
	DiffCompareFile components.TextInput
	DiffTable       components.Table
	DiffResult      *sync.DiffResult

	// 7. Launcher Sync table & options
	LauncherTable     components.Table
	LauncherInstances []launcher.Instance
	LauncherForce     bool
	LauncherDryRun    bool

	ActiveInputIdx int
	LogsViewport   components.Viewport
	IsSyncing      bool
	Spinner        components.Spinner
	LastResult     *sync.SyncResult
	StatusMessage  string
	StatusIsError  bool
	Width          int
	Height         int
}

// NewSyncModel initializes Tab 4.
func NewSyncModel(client *modrinth.Client, cfgPath, lockPath string, cfg *config.Config) SyncModel {
	vp := components.NewViewport(80, 10)
	vp.SetContent("Ready to synchronize. Select a workflow above and press [Enter] or [x] to run.")

	diffCols := []components.Column{
		{Title: "STATUS", Width: 12},
		{Title: "NAME", Width: 22},
		{Title: "SLUG", Width: 18},
		{Title: "LOCAL", Width: 12},
		{Title: "REMOTE", Width: 12},
		{Title: "NOTES", Width: 22},
	}
	diffTable := components.NewTable(diffCols, 10)

	launcherCols := []components.Column{
		{Title: "LAUNCHER", Width: 14},
		{Title: "NAME", Width: 20},
		{Title: "MC VER", Width: 10},
		{Title: "LOADER", Width: 10},
		{Title: "LOADER VER", Width: 12},
		{Title: "PATH", Width: 24},
	}
	launcherTable := components.NewTable(launcherCols, 10)

	m := SyncModel{
		Client:            client,
		ConfigPath:        cfgPath,
		LockPath:          lockPath,
		Config:            cfg,
		SelectedEngine:    EngineLocal,
		LogsViewport:      vp,
		DiffTable:         diffTable,
		LauncherTable:     launcherTable,
		PushIncludeConfig: true,
		PushDryRun:        false,
		LauncherForce:     false,
		LauncherDryRun:    false,
		Spinner:           components.NewSpinner(),
		Width:             80,
		Height:            20,
	}

	// 2. Modpack inputs
	m.ModpackSlug = components.NewTextInput()
	m.ModpackSlug.Placeholder = "Modpack slug (e.g. fabulously-optimized)"
	m.ModpackPath = components.NewTextInput()
	m.ModpackPath.Placeholder = "Path to local .mrpack archive (optional)"

	// 3. GitHub inputs
	m.GitHubRepo = components.NewTextInput()
	m.GitHubRepo.Placeholder = "owner/repository (e.g. myorg/modpack)"
	m.GitHubBranch = components.NewTextInput()
	m.GitHubBranch.SetValue("main")
	m.GitHubToken = components.NewTextInput()
	m.GitHubToken.Placeholder = "GitHub personal access token (optional for public repos)"

	// 4. Remote Pull inputs
	m.RemoteURL = components.NewTextInput()
	m.RemoteURL.SetValue("http://localhost:8080")
	m.RemoteToken = components.NewTextInput()
	m.RemoteToken.Placeholder = "Server sync bearer token (if required)"

	// 5. Remote Push inputs
	m.PushRemoteURL = components.NewTextInput()
	m.PushRemoteURL.SetValue("http://localhost:8080")
	m.PushRemoteToken = components.NewTextInput()
	m.PushRemoteToken.Placeholder = "Server sync bearer token (required for push)"

	// 6. Diff inputs
	m.DiffRemoteURL = components.NewTextInput()
	m.DiffRemoteURL.SetValue("http://localhost:8080")
	m.DiffRemoteToken = components.NewTextInput()
	m.DiffRemoteToken.Placeholder = "Bearer token (optional)"
	m.DiffCompareFile = components.NewTextInput()
	m.DiffCompareFile.Placeholder = "Path to 2nd cmm.lock for local vs local diff (optional)"

	m.updateInputFocus()
	return m
}

func (m *SyncModel) updateInputFocus() {
	m.ModpackSlug.Blur()
	m.ModpackPath.Blur()
	m.GitHubRepo.Blur()
	m.GitHubBranch.Blur()
	m.GitHubToken.Blur()
	m.RemoteURL.Blur()
	m.RemoteToken.Blur()
	m.PushRemoteURL.Blur()
	m.PushRemoteToken.Blur()
	m.DiffRemoteURL.Blur()
	m.DiffRemoteToken.Blur()
	m.DiffCompareFile.Blur()
	m.DiffTable.Blur()
	m.LauncherTable.Blur()

	switch m.SelectedEngine {
	case EngineModpack:
		if m.ActiveInputIdx == 0 {
			m.ModpackSlug.Focus()
		} else {
			m.ModpackPath.Focus()
		}
	case EngineGitHub:
		if m.ActiveInputIdx == 0 {
			m.GitHubRepo.Focus()
		} else if m.ActiveInputIdx == 1 {
			m.GitHubBranch.Focus()
		} else {
			m.GitHubToken.Focus()
		}
	case EngineRemote:
		if m.ActiveInputIdx == 0 {
			m.RemoteURL.Focus()
		} else {
			m.RemoteToken.Focus()
		}
	case EnginePush:
		if m.ActiveInputIdx == 0 {
			m.PushRemoteURL.Focus()
		} else if m.ActiveInputIdx == 1 {
			m.PushRemoteToken.Focus()
		}
	case EngineDiff:
		if m.ActiveInputIdx == 0 {
			m.DiffRemoteURL.Focus()
		} else if m.ActiveInputIdx == 1 {
			m.DiffRemoteToken.Focus()
		} else if m.ActiveInputIdx == 2 {
			m.DiffCompareFile.Focus()
		} else if m.ActiveInputIdx == 4 {
			m.DiffTable.Focus()
		}
	case EngineLauncher:
		m.LauncherTable.Focus()
	}
}

// IsTypingInput reports if the user is actively typing in a text field.
func (m SyncModel) IsTypingInput() bool {
	switch m.SelectedEngine {
	case EngineLocal:
		return false
	case EngineModpack:
		return true
	case EngineGitHub:
		return true
	case EngineRemote:
		return true
	case EnginePush:
		return m.ActiveInputIdx == 0 || m.ActiveInputIdx == 1
	case EngineDiff:
		return m.ActiveInputIdx == 0 || m.ActiveInputIdx == 1 || m.ActiveInputIdx == 2
	case EngineLauncher:
		return false
	default:
		return false
	}
}

// IsHandlingTab reports if Tab/Shift+Tab is consumed by this tab.
func (m SyncModel) IsHandlingTab() bool {
	switch m.SelectedEngine {
	case EngineLocal:
		return false
	case EngineModpack, EngineGitHub, EngineRemote, EnginePush, EngineDiff:
		return true
	case EngineLauncher:
		return false
	default:
		return false
	}
}

// Init initializes Tab 4 and starts background launcher detection.
func (m SyncModel) Init() tea.Cmd {
	return m.DetectLaunchersCmd()
}

// SetSize updates dimensions.
func (m *SyncModel) SetSize(width, height int) {
	m.Width = width
	m.Height = height

	vpHeight := height - 13
	if vpHeight < 5 {
		vpHeight = 5
	}
	m.LogsViewport.Width = width
	m.LogsViewport.Height = vpHeight

	// Diff table sizing
	diffTableHeight := height - 16
	if diffTableHeight < 4 {
		diffTableHeight = 4
	}
	m.DiffTable.SetHeight(diffTableHeight)
	m.DiffTable.SetWidth(width)

	statusW := 12
	slugW := 16
	locVerW := 12
	remVerW := 12
	notesW := 20
	nameW := width - (statusW + slugW + locVerW + remVerW + notesW + 12)
	if nameW < 14 {
		nameW = 14
	}
	m.DiffTable.SetColumns([]components.Column{
		{Title: "STATUS", Width: statusW},
		{Title: "NAME", Width: nameW},
		{Title: "SLUG", Width: slugW},
		{Title: "LOCAL", Width: locVerW},
		{Title: "REMOTE", Width: remVerW},
		{Title: "NOTES", Width: notesW},
	})

	// Launcher table sizing
	launcherTableHeight := height - 15
	if launcherTableHeight < 4 {
		launcherTableHeight = 4
	}
	m.LauncherTable.SetHeight(launcherTableHeight)
	m.LauncherTable.SetWidth(width)

	launcherW := 14
	mcVerW := 10
	loaderW := 10
	loaderVerW := 12
	instNameW := 20
	pathW := width - (launcherW + mcVerW + loaderW + loaderVerW + instNameW + 12)
	if pathW < 18 {
		pathW = 18
	}
	m.LauncherTable.SetColumns([]components.Column{
		{Title: "LAUNCHER", Width: launcherW},
		{Title: "NAME", Width: instNameW},
		{Title: "MC VER", Width: mcVerW},
		{Title: "LOADER", Width: loaderW},
		{Title: "LOADER VER", Width: loaderVerW},
		{Title: "PATH", Width: pathW},
	})
}

// SyncCompleteMsg carries sync results for Engines 1-4.
type SyncCompleteMsg struct {
	Engine SyncEngineType
	Result *sync.SyncResult
	Err    error
}

// PushCompleteMsg carries results for Engine 5 (Remote Push).
type PushCompleteMsg struct {
	Result *sync.PushResult
	Err    error
}

// DiffCompleteMsg carries results for Engine 6 (Diff Inspector).
type DiffCompleteMsg struct {
	Result *sync.DiffResult
	Err    error
}

// LaunchersDetectedMsg carries results for launcher auto-detection.
type LaunchersDetectedMsg struct {
	Instances []launcher.Instance
	Err       error
}

// LauncherSyncCompleteMsg carries results for launcher instance synchronization.
type LauncherSyncCompleteMsg struct {
	Result *launcher.LauncherSyncResult
	Err    error
}

// DetectLaunchersCmd initiates launcher instance auto-detection.
func (m SyncModel) DetectLaunchersCmd() tea.Cmd {
	return func() tea.Msg {
		detector := launcher.NewDetector(launcher.DetectorOptions{})
		instances, err := detector.DetectAll()
		return LaunchersDetectedMsg{Instances: instances, Err: err}
	}
}

// SelectedLauncherInstance returns the currently highlighted launcher instance in the table.
func (m *SyncModel) SelectedLauncherInstance() *launcher.Instance {
	if len(m.LauncherInstances) == 0 {
		return nil
	}
	cursor := m.LauncherTable.Cursor()
	if cursor >= 0 && cursor < len(m.LauncherInstances) {
		return &m.LauncherInstances[cursor]
	}
	return nil
}

// ExecutePushCmd initiates remote push synchronization.
func (m SyncModel) ExecutePushCmd() tea.Cmd {
	return func() tea.Msg {
		configDir := "config"
		if m.Config != nil && m.Config.Paths.ConfigDir != "" {
			configDir = m.Config.Paths.ConfigDir
		}
		syncer := sync.NewPushSynchronizer(m.ConfigPath, m.LockPath, configDir)
		url := strings.TrimSpace(m.PushRemoteURL.Value())
		token := strings.TrimSpace(m.PushRemoteToken.Value())
		res, err := syncer.Push(sync.PushOptions{
			URL:           url,
			Token:         token,
			IncludeConfig: m.PushIncludeConfig,
			DryRun:        m.PushDryRun,
			ConfigPath:    m.ConfigPath,
			LockPath:      m.LockPath,
			ConfigDir:     configDir,
		})
		return PushCompleteMsg{Result: res, Err: err}
	}
}

// ExecuteDiffCmd initiates lockfile / remote comparison.
func (m SyncModel) ExecuteDiffCmd() tea.Cmd {
	return func() tea.Msg {
		engine := sync.NewDiffEngine()
		compareFile := strings.TrimSpace(m.DiffCompareFile.Value())
		if compareFile != "" {
			res, err := engine.CompareFiles(m.LockPath, compareFile)
			return DiffCompleteMsg{Result: res, Err: err}
		}
		url := strings.TrimSpace(m.DiffRemoteURL.Value())
		token := strings.TrimSpace(m.DiffRemoteToken.Value())
		res, err := engine.CompareRemote(m.LockPath, url, token)
		return DiffCompleteMsg{Result: res, Err: err}
	}
}

// ExecuteLauncherSyncCmd initiates direct modpack sync into launcher instance.
func (m SyncModel) ExecuteLauncherSyncCmd(inst launcher.Instance, force, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		syncer := launcher.NewSyncer(m.Client, launcher.DetectorOptions{})
		opts := launcher.LauncherSyncOptions{
			InstanceName: inst.Name,
			LauncherType: inst.Launcher,
			ConfigPath:   m.ConfigPath,
			LockPath:     m.LockPath,
			Force:        force,
			DryRun:       dryRun,
		}
		res, err := syncer.SyncInstance(opts)
		return LauncherSyncCompleteMsg{Result: res, Err: err}
	}
}

// ExecuteSyncCmd initiates synchronization based on selected engine.
func (m SyncModel) ExecuteSyncCmd() tea.Cmd {
	switch m.SelectedEngine {
	case EngineLocal:
		return func() tea.Msg {
			syncer := sync.NewLocalSynchronizer(m.Client, m.ConfigPath, m.LockPath)
			modsDir := "mods"
			if m.Config != nil && m.Config.Paths.ModsDir != "" {
				modsDir = m.Config.Paths.ModsDir
			}
			res, err := syncer.Sync(sync.LocalSyncOptions{Path: modsDir})
			return SyncCompleteMsg{Engine: EngineLocal, Result: res, Err: err}
		}

	case EngineModpack:
		return func() tea.Msg {
			syncer := sync.NewModrinthSynchronizer(m.Client, m.ConfigPath, m.LockPath)
			slug := strings.TrimSpace(m.ModpackSlug.Value())
			filePath := strings.TrimSpace(m.ModpackPath.Value())
			res, err := syncer.Sync(sync.ModrinthSyncOptions{Slug: slug, FilePath: filePath})
			return SyncCompleteMsg{Engine: EngineModpack, Result: res, Err: err}
		}

	case EngineGitHub:
		return func() tea.Msg {
			syncer := sync.NewGitHubSynchronizer(m.Client, m.ConfigPath, m.LockPath)
			repo := strings.TrimSpace(m.GitHubRepo.Value())
			branch := strings.TrimSpace(m.GitHubBranch.Value())
			token := strings.TrimSpace(m.GitHubToken.Value())
			res, err := syncer.Sync(sync.GitHubSyncOptions{Repo: repo, Branch: branch, Token: token})
			return SyncCompleteMsg{Engine: EngineGitHub, Result: res, Err: err}
		}

	case EngineRemote:
		return func() tea.Msg {
			syncer := sync.NewRemoteSynchronizer(m.Client, m.ConfigPath, m.LockPath)
			url := strings.TrimSpace(m.RemoteURL.Value())
			token := strings.TrimSpace(m.RemoteToken.Value())
			res, err := syncer.Sync(sync.RemoteSyncOptions{URL: url, Token: token})
			return SyncCompleteMsg{Engine: EngineRemote, Result: res, Err: err}
		}

	case EnginePush:
		return m.ExecutePushCmd()

	case EngineDiff:
		return m.ExecuteDiffCmd()

	case EngineLauncher:
		inst := m.SelectedLauncherInstance()
		if inst == nil {
			return func() tea.Msg {
				return LauncherSyncCompleteMsg{Err: fmt.Errorf("no launcher instance selected")}
			}
		}
		return m.ExecuteLauncherSyncCmd(*inst, m.LauncherForce, m.LauncherDryRun)

	default:
		return func() tea.Msg {
			return SyncCompleteMsg{Err: fmt.Errorf("unknown sync engine")}
		}
	}
}

func (m *SyncModel) populateDiffTable(res *sync.DiffResult) {
	if res == nil {
		m.DiffTable.SetRows([][]string{})
		return
	}
	var rows [][]string
	for _, entry := range res.Entries {
		var statusBadge string
		switch entry.Category {
		case sync.DiffOK:
			statusBadge = styles.BadgeOk.Render("[OK]")
		case sync.DiffMismatch:
			statusBadge = styles.BadgeUpdate.Render("[MISMATCH]")
		case sync.DiffClient:
			statusBadge = styles.BadgeClient.Render("[CLIENT]")
		case sync.DiffServer:
			statusBadge = styles.BadgeServer.Render("[SERVER]")
		case sync.DiffMissing:
			statusBadge = styles.StatusError.Render("[MISSING]")
		default:
			statusBadge = entry.Status
		}

		locVer := entry.LocalVersion
		if locVer == "" {
			locVer = "-"
		}
		remVer := entry.RemoteVersion
		if remVer == "" {
			remVer = "-"
		}
		notes := entry.Notes

		rows = append(rows, []string{
			statusBadge,
			entry.Name,
			entry.Slug,
			locVer,
			remVer,
			notes,
		})
	}
	m.DiffTable.SetRows(rows)
}

func (m *SyncModel) populateLauncherTable(instances []launcher.Instance) {
	var rows [][]string
	for _, inst := range instances {
		launcherName := inst.LauncherName
		if launcherName == "" {
			launcherName = string(inst.Launcher)
		}
		mcVer := inst.MinecraftVersion
		if mcVer == "" {
			mcVer = "-"
		}
		loader := inst.Loader
		if loader == "" {
			loader = "vanilla"
		}
		loaderVer := inst.LoaderVersion
		if loaderVer == "" {
			loaderVer = "-"
		}
		path := inst.InstanceDir

		rows = append(rows, []string{
			launcherName,
			inst.Name,
			mcVer,
			loader,
			loaderVer,
			path,
		})
	}
	m.LauncherTable.SetRows(rows)
}

// Update handles state transitions for Tab 4.
func (m SyncModel) Update(msg tea.Msg) (SyncModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case components.SpinnerTickMsg:
		if m.IsSyncing {
			var cmd tea.Cmd
			m.Spinner, cmd = m.Spinner.Update(msg)
			return m, cmd
		}

	case LaunchersDetectedMsg:
		if msg.Err != nil {
			m.StatusMessage = fmt.Sprintf("Launcher detection warning: %v", msg.Err)
			m.StatusIsError = true
		} else {
			m.LauncherInstances = msg.Instances
			m.populateLauncherTable(msg.Instances)
			if len(msg.Instances) == 0 {
				m.StatusMessage = "No Minecraft launcher instances detected on local system."
				m.StatusIsError = false
			} else {
				m.StatusMessage = fmt.Sprintf("Detected %d launcher instance(s)", len(msg.Instances))
				m.StatusIsError = false
			}
		}

	case SyncCompleteMsg:
		m.IsSyncing = false
		if msg.Err != nil {
			m.StatusMessage = fmt.Sprintf("Sync failed: %v", msg.Err)
			m.StatusIsError = true
			m.LogsViewport.Append(styles.StatusError.Render(fmt.Sprintf("[%s] ERROR: %v", msg.Engine.Title(), msg.Err)))
		} else {
			m.LastResult = msg.Result
			m.StatusMessage = "Sync completed successfully"
			m.StatusIsError = false

			var logSb strings.Builder
			logSb.WriteString(styles.StatusSuccess.Render(fmt.Sprintf("=== Sync Complete: %s ===", msg.Engine.Title())))
			logSb.WriteString("\n")
			if msg.Result != nil {
				if msg.Result.Message != "" {
					logSb.WriteString("Message: " + msg.Result.Message + "\n")
				}
				logSb.WriteString(fmt.Sprintf("  • Added Mods:    %d\n", len(msg.Result.AddedMods)))
				for _, mod := range msg.Result.AddedMods {
					logSb.WriteString(fmt.Sprintf("    + %s\n", mod))
				}
				logSb.WriteString(fmt.Sprintf("  • Updated Mods:  %d\n", len(msg.Result.UpdatedMods)))
				for _, mod := range msg.Result.UpdatedMods {
					logSb.WriteString(fmt.Sprintf("    ~ %s\n", mod))
				}
				logSb.WriteString(fmt.Sprintf("  • Removed Mods:  %d\n", len(msg.Result.RemovedMods)))
				for _, mod := range msg.Result.RemovedMods {
					logSb.WriteString(fmt.Sprintf("    - %s\n", mod))
				}
				if len(msg.Result.UnknownJars) > 0 {
					logSb.WriteString(fmt.Sprintf("  • Unknown JARs:  %d\n", len(msg.Result.UnknownJars)))
					for _, jar := range msg.Result.UnknownJars {
						logSb.WriteString(fmt.Sprintf("    ? %s\n", jar))
					}
				}
				if msg.Result.UpToDate {
					logSb.WriteString("Status: Up to date (no changes needed)\n")
				}
			}
			m.LogsViewport.Append(logSb.String())
		}

	case PushCompleteMsg:
		m.IsSyncing = false
		if msg.Err != nil {
			m.StatusMessage = fmt.Sprintf("Push failed: %v", msg.Err)
			m.StatusIsError = true
			m.LogsViewport.Append(styles.StatusError.Render(fmt.Sprintf("[5. Remote Push] ERROR: %v", msg.Err)))
		} else {
			m.StatusMessage = "Remote push completed successfully"
			m.StatusIsError = false

			var logSb strings.Builder
			logSb.WriteString(styles.StatusSuccess.Render("=== Remote Push Execution Complete ==="))
			logSb.WriteString("\n")
			if msg.Result != nil {
				if msg.Result.Message != "" {
					logSb.WriteString("Server Response: " + msg.Result.Message + "\n")
				}
				logSb.WriteString(fmt.Sprintf("  • Added Mods:      %d\n", len(msg.Result.AddedMods)))
				for _, mod := range msg.Result.AddedMods {
					logSb.WriteString(fmt.Sprintf("    + %s\n", mod))
				}
				logSb.WriteString(fmt.Sprintf("  • Updated Mods:    %d\n", len(msg.Result.UpdatedMods)))
				for _, mod := range msg.Result.UpdatedMods {
					logSb.WriteString(fmt.Sprintf("    ~ %s\n", mod))
				}
				logSb.WriteString(fmt.Sprintf("  • Pruned Mods:     %d\n", len(msg.Result.PrunedMods)))
				for _, mod := range msg.Result.PrunedMods {
					logSb.WriteString(fmt.Sprintf("    - %s\n", mod))
				}
				logSb.WriteString(fmt.Sprintf("  • Configs Updated: %d\n", msg.Result.ConfigsUpdated))
			}
			m.LogsViewport.Append(logSb.String())
		}

	case DiffCompleteMsg:
		m.IsSyncing = false
		if msg.Err != nil {
			m.StatusMessage = fmt.Sprintf("Diff failed: %v", msg.Err)
			m.StatusIsError = true
			m.LogsViewport.Append(styles.StatusError.Render(fmt.Sprintf("[6. Diff Inspector] ERROR: %v", msg.Err)))
		} else {
			m.DiffResult = msg.Result
			m.populateDiffTable(msg.Result)
			m.StatusMessage = fmt.Sprintf("Diff complete: %d mods compared (%d OK, %d mismatch, %d client-only, %d server-only, %d missing)",
				msg.Result.Total, msg.Result.Synchronized, msg.Result.Mismatches, msg.Result.ClientOnly, msg.Result.ServerOnly, msg.Result.Missing)
			m.StatusIsError = false

			var logSb strings.Builder
			logSb.WriteString(styles.StatusSuccess.Render(fmt.Sprintf("=== Diff Comparison: %s ===", msg.Result.Target)))
			logSb.WriteString("\n")
			logSb.WriteString(fmt.Sprintf("  • Total Mods:     %d\n", msg.Result.Total))
			logSb.WriteString(fmt.Sprintf("  • Synchronized:   %d\n", msg.Result.Synchronized))
			logSb.WriteString(fmt.Sprintf("  • Mismatches:     %d\n", msg.Result.Mismatches))
			logSb.WriteString(fmt.Sprintf("  • Client-Only:    %d\n", msg.Result.ClientOnly))
			logSb.WriteString(fmt.Sprintf("  • Server-Only:    %d\n", msg.Result.ServerOnly))
			logSb.WriteString(fmt.Sprintf("  • Missing:        %d\n", msg.Result.Missing))
			m.LogsViewport.Append(logSb.String())

			// Focus diff table so user can scroll immediately
			m.ActiveInputIdx = 4
			m.updateInputFocus()
		}

	case LauncherSyncCompleteMsg:
		m.IsSyncing = false
		if msg.Err != nil {
			m.StatusMessage = fmt.Sprintf("Launcher sync failed: %v", msg.Err)
			m.StatusIsError = true
			m.LogsViewport.Append(styles.StatusError.Render(fmt.Sprintf("[7. Launcher Sync] ERROR: %v", msg.Err)))
		} else {
			m.StatusMessage = fmt.Sprintf("Modpack synced to instance '%s' successfully", msg.Result.Instance.Name)
			m.StatusIsError = false

			var logSb strings.Builder
			logSb.WriteString(styles.StatusSuccess.Render(fmt.Sprintf("=== Launcher Sync: %s (%s) ===", msg.Result.Instance.Name, msg.Result.Instance.LauncherName)))
			logSb.WriteString("\n")
			logSb.WriteString(fmt.Sprintf("Target Path: %s\n", msg.Result.Instance.InstanceDir))
			if msg.Result.Message != "" {
				logSb.WriteString("Message: " + msg.Result.Message + "\n")
			}
			logSb.WriteString(fmt.Sprintf("  • Added Mods:    %d\n", len(msg.Result.AddedMods)))
			for _, mod := range msg.Result.AddedMods {
				logSb.WriteString(fmt.Sprintf("    + %s\n", mod))
			}
			logSb.WriteString(fmt.Sprintf("  • Updated Mods:  %d\n", len(msg.Result.UpdatedMods)))
			for _, mod := range msg.Result.UpdatedMods {
				logSb.WriteString(fmt.Sprintf("    ~ %s\n", mod))
			}
			logSb.WriteString(fmt.Sprintf("  • Removed Mods:  %d\n", len(msg.Result.RemovedMods)))
			for _, mod := range msg.Result.RemovedMods {
				logSb.WriteString(fmt.Sprintf("    - %s\n", mod))
			}
			if len(msg.Result.SkippedMods) > 0 {
				logSb.WriteString(fmt.Sprintf("  • Skipped Server Mods: %d\n", len(msg.Result.SkippedMods)))
				for _, mod := range msg.Result.SkippedMods {
					logSb.WriteString(fmt.Sprintf("    * %s (server only)\n", mod))
				}
			}
			if msg.Result.UpToDate {
				logSb.WriteString("Status: Instance is already up to date.\n")
			}
			m.LogsViewport.Append(logSb.String())
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "1":
			m.SelectedEngine = EngineLocal
			m.ActiveInputIdx = 0
			m.updateInputFocus()
			return m, nil
		case "2":
			m.SelectedEngine = EngineModpack
			m.ActiveInputIdx = 0
			m.updateInputFocus()
			return m, nil
		case "3":
			m.SelectedEngine = EngineGitHub
			m.ActiveInputIdx = 0
			m.updateInputFocus()
			return m, nil
		case "4":
			m.SelectedEngine = EngineRemote
			m.ActiveInputIdx = 0
			m.updateInputFocus()
			return m, nil
		case "5":
			m.SelectedEngine = EnginePush
			m.ActiveInputIdx = 0
			m.updateInputFocus()
			return m, nil
		case "6":
			m.SelectedEngine = EngineDiff
			m.ActiveInputIdx = 0
			m.updateInputFocus()
			return m, nil
		case "7":
			m.SelectedEngine = EngineLauncher
			m.ActiveInputIdx = 0
			m.updateInputFocus()
			if len(m.LauncherInstances) == 0 {
				return m, m.DetectLaunchersCmd()
			}
			return m, nil
		}

		// Engine 7: Launcher specific shortcuts
		if m.SelectedEngine == EngineLauncher {
			switch msg.String() {
			case "r":
				m.StatusMessage = "Rescanning Minecraft launcher instances..."
				m.StatusIsError = false
				return m, m.DetectLaunchersCmd()
			case "f":
				m.LauncherForce = !m.LauncherForce
				return m, nil
			case "d":
				m.LauncherDryRun = !m.LauncherDryRun
				return m, nil
			case "s", "enter", "x":
				if !m.IsSyncing {
					inst := m.SelectedLauncherInstance()
					if inst == nil {
						m.StatusMessage = "No launcher instance selected"
						m.StatusIsError = true
						return m, nil
					}
					m.IsSyncing = true
					m.StatusMessage = fmt.Sprintf("Synchronizing modpack to '%s'...", inst.Name)
					m.StatusIsError = false
					m.LogsViewport.Append(fmt.Sprintf("Starting sync to launcher instance '%s' (%s)...", inst.Name, inst.InstanceDir))
					return m, tea.Batch(m.ExecuteLauncherSyncCmd(*inst, m.LauncherForce, m.LauncherDryRun), m.Spinner.Tick())
				}
			default:
				// Pass to LauncherTable
				var tblCmd tea.Cmd
				m.LauncherTable, tblCmd = m.LauncherTable.Update(msg)
				if tblCmd != nil {
					cmds = append(cmds, tblCmd)
				}
			}
			return m, tea.Batch(cmds...)
		}

		// Global execution triggers (x, p, d)
		switch msg.String() {
		case "x":
			if !m.IsSyncing {
				m.IsSyncing = true
				m.StatusMessage = fmt.Sprintf("Running %s...", m.SelectedEngine.Title())
				m.StatusIsError = false
				m.LogsViewport.Append(fmt.Sprintf("Starting %s...", m.SelectedEngine.Title()))
				return m, tea.Batch(m.ExecuteSyncCmd(), m.Spinner.Tick())
			}

		case "p":
			if m.SelectedEngine == EnginePush && !m.IsSyncing && m.ActiveInputIdx >= 2 {
				m.IsSyncing = true
				m.StatusMessage = "Pushing modpack to remote server..."
				m.StatusIsError = false
				m.LogsViewport.Append("Starting Remote Push...")
				return m, tea.Batch(m.ExecutePushCmd(), m.Spinner.Tick())
			}

		case "d":
			if m.SelectedEngine == EngineDiff && !m.IsSyncing && m.ActiveInputIdx >= 3 {
				m.IsSyncing = true
				m.StatusMessage = "Comparing local modpack with remote..."
				m.StatusIsError = false
				m.LogsViewport.Append("Starting Diff Audit...")
				return m, tea.Batch(m.ExecuteDiffCmd(), m.Spinner.Tick())
			}

		case " ":
			// Checkbox toggle handler
			if m.SelectedEngine == EnginePush {
				if m.ActiveInputIdx == 2 {
					m.PushIncludeConfig = !m.PushIncludeConfig
					return m, nil
				} else if m.ActiveInputIdx == 3 {
					m.PushDryRun = !m.PushDryRun
					return m, nil
				}
			}

		case "enter":
			switch m.SelectedEngine {
			case EngineLocal:
				if !m.IsSyncing {
					m.IsSyncing = true
					m.StatusMessage = fmt.Sprintf("Running %s...", m.SelectedEngine.Title())
					m.StatusIsError = false
					m.LogsViewport.Append(fmt.Sprintf("Starting %s...", m.SelectedEngine.Title()))
					return m, tea.Batch(m.ExecuteSyncCmd(), m.Spinner.Tick())
				}

			case EngineModpack, EngineRemote:
				if m.ActiveInputIdx >= 1 {
					if !m.IsSyncing {
						m.IsSyncing = true
						m.StatusMessage = fmt.Sprintf("Running %s...", m.SelectedEngine.Title())
						m.StatusIsError = false
						m.LogsViewport.Append(fmt.Sprintf("Starting %s...", m.SelectedEngine.Title()))
						return m, tea.Batch(m.ExecuteSyncCmd(), m.Spinner.Tick())
					}
				} else {
					m.ActiveInputIdx++
					m.updateInputFocus()
					return m, nil
				}

			case EngineGitHub:
				if m.ActiveInputIdx >= 2 {
					if !m.IsSyncing {
						m.IsSyncing = true
						m.StatusMessage = fmt.Sprintf("Running %s...", m.SelectedEngine.Title())
						m.StatusIsError = false
						m.LogsViewport.Append(fmt.Sprintf("Starting %s...", m.SelectedEngine.Title()))
						return m, tea.Batch(m.ExecuteSyncCmd(), m.Spinner.Tick())
					}
				} else {
					m.ActiveInputIdx++
					m.updateInputFocus()
					return m, nil
				}

			case EnginePush:
				if m.ActiveInputIdx == 2 {
					m.PushIncludeConfig = !m.PushIncludeConfig
					return m, nil
				} else if m.ActiveInputIdx == 3 {
					m.PushDryRun = !m.PushDryRun
					return m, nil
				} else if m.ActiveInputIdx >= 4 {
					if !m.IsSyncing {
						m.IsSyncing = true
						m.StatusMessage = "Pushing modpack to remote server..."
						m.StatusIsError = false
						m.LogsViewport.Append("Starting Remote Push...")
						return m, tea.Batch(m.ExecutePushCmd(), m.Spinner.Tick())
					}
				} else {
					m.ActiveInputIdx++
					m.updateInputFocus()
					return m, nil
				}

			case EngineDiff:
				if m.ActiveInputIdx == 3 {
					if !m.IsSyncing {
						m.IsSyncing = true
						m.StatusMessage = "Computing client-server difference audit..."
						m.StatusIsError = false
						m.LogsViewport.Append("Starting Diff Audit...")
						return m, tea.Batch(m.ExecuteDiffCmd(), m.Spinner.Tick())
					}
				} else if m.ActiveInputIdx < 3 {
					m.ActiveInputIdx++
					m.updateInputFocus()
					return m, nil
				}
			}

		case "tab":
			maxIdx := 0
			switch m.SelectedEngine {
			case EngineModpack, EngineRemote:
				maxIdx = 1
			case EngineGitHub:
				maxIdx = 2
			case EnginePush:
				maxIdx = 4
			case EngineDiff:
				maxIdx = 4
			}
			if maxIdx > 0 {
				m.ActiveInputIdx = (m.ActiveInputIdx + 1) % (maxIdx + 1)
				m.updateInputFocus()
				return m, nil
			}

		case "shift+tab":
			maxIdx := 0
			switch m.SelectedEngine {
			case EngineModpack, EngineRemote:
				maxIdx = 1
			case EngineGitHub:
				maxIdx = 2
			case EnginePush:
				maxIdx = 4
			case EngineDiff:
				maxIdx = 4
			}
			if maxIdx > 0 {
				m.ActiveInputIdx = (m.ActiveInputIdx - 1 + maxIdx + 1) % (maxIdx + 1)
				m.updateInputFocus()
				return m, nil
			}
		}

		// Update active text inputs
		switch m.SelectedEngine {
		case EngineModpack:
			if m.ActiveInputIdx == 0 {
				var cmd tea.Cmd
				m.ModpackSlug, cmd = m.ModpackSlug.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else {
				var cmd tea.Cmd
				m.ModpackPath, cmd = m.ModpackPath.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		case EngineGitHub:
			if m.ActiveInputIdx == 0 {
				var cmd tea.Cmd
				m.GitHubRepo, cmd = m.GitHubRepo.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else if m.ActiveInputIdx == 1 {
				var cmd tea.Cmd
				m.GitHubBranch, cmd = m.GitHubBranch.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else {
				var cmd tea.Cmd
				m.GitHubToken, cmd = m.GitHubToken.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		case EngineRemote:
			if m.ActiveInputIdx == 0 {
				var cmd tea.Cmd
				m.RemoteURL, cmd = m.RemoteURL.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else {
				var cmd tea.Cmd
				m.RemoteToken, cmd = m.RemoteToken.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		case EnginePush:
			if m.ActiveInputIdx == 0 {
				var cmd tea.Cmd
				m.PushRemoteURL, cmd = m.PushRemoteURL.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else if m.ActiveInputIdx == 1 {
				var cmd tea.Cmd
				m.PushRemoteToken, cmd = m.PushRemoteToken.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		case EngineDiff:
			if m.ActiveInputIdx == 0 {
				var cmd tea.Cmd
				m.DiffRemoteURL, cmd = m.DiffRemoteURL.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else if m.ActiveInputIdx == 1 {
				var cmd tea.Cmd
				m.DiffRemoteToken, cmd = m.DiffRemoteToken.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else if m.ActiveInputIdx == 2 {
				var cmd tea.Cmd
				m.DiffCompareFile, cmd = m.DiffCompareFile.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else if m.ActiveInputIdx == 4 {
				var cmd tea.Cmd
				m.DiffTable, cmd = m.DiffTable.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}

		// Scroll viewport
		var vpCmd tea.Cmd
		m.LogsViewport, vpCmd = m.LogsViewport.Update(msg)
		if vpCmd != nil {
			cmds = append(cmds, vpCmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders Tab 4.
func (m SyncModel) View() string {
	var sb strings.Builder

	// Top Engine selector buttons in two rows
	sb.WriteString(styles.FieldLabelStyle.Render("Select Sync Workflow:") + "\n")
	row1 := []SyncEngineType{EngineLocal, EngineModpack, EngineGitHub, EngineRemote}
	for _, eng := range row1 {
		title := eng.Title()
		if eng == m.SelectedEngine {
			sb.WriteString(styles.ButtonActive.Render(" " + title + " "))
		} else {
			sb.WriteString(styles.ButtonInactive.Render(" " + title + " "))
		}
		sb.WriteString("  ")
	}
	sb.WriteString("\n")
	row2 := []SyncEngineType{EnginePush, EngineDiff, EngineLauncher}
	for _, eng := range row2 {
		title := eng.Title()
		if eng == m.SelectedEngine {
			sb.WriteString(styles.ButtonActive.Render(" " + title + " "))
		} else {
			sb.WriteString(styles.ButtonInactive.Render(" " + title + " "))
		}
		sb.WriteString("  ")
	}
	sb.WriteString("\n\n")

	// Engine specific forms
	switch m.SelectedEngine {
	case EngineLocal:
		sb.WriteString(styles.HelpDescStyle.Render("Scans local JAR files in mods/ directory, computes hashes, and reconciles cmm.lock."))
		sb.WriteString("\n\n")
		sb.WriteString(styles.ButtonActive.Render(" Press [x] or [Enter] to Run Local Scan "))

	case EngineModpack:
		sb.WriteString(styles.FieldLabelStyle.Render("Modpack Slug:    "))
		sb.WriteString(m.ModpackSlug.View())
		sb.WriteString("\n")
		sb.WriteString(styles.FieldLabelStyle.Render("Or .mrpack Path: "))
		sb.WriteString(m.ModpackPath.View())
		sb.WriteString("\n\n")
		sb.WriteString(styles.ButtonActive.Render(" Press [x] or [Enter] to Sync Modpack "))

	case EngineGitHub:
		sb.WriteString(styles.FieldLabelStyle.Render("GitHub Repo: "))
		sb.WriteString(m.GitHubRepo.View())
		sb.WriteString("\n")
		sb.WriteString(styles.FieldLabelStyle.Render("Git Branch:  "))
		sb.WriteString(m.GitHubBranch.View())
		sb.WriteString("\n")
		sb.WriteString(styles.FieldLabelStyle.Render("Auth Token:  "))
		sb.WriteString(m.GitHubToken.View())
		sb.WriteString("\n\n")
		sb.WriteString(styles.ButtonActive.Render(" Press [x] or [Enter] to Sync GitHub "))

	case EngineRemote:
		sb.WriteString(styles.FieldLabelStyle.Render("Server URL:  "))
		sb.WriteString(m.RemoteURL.View())
		sb.WriteString("\n")
		sb.WriteString(styles.FieldLabelStyle.Render("Sync Token:  "))
		sb.WriteString(m.RemoteToken.View())
		sb.WriteString("\n\n")
		sb.WriteString(styles.ButtonActive.Render(" Press [x] or [Enter] to Pull from Remote Server "))

	case EnginePush:
		sb.WriteString(styles.FieldLabelStyle.Render("Server URL:     "))
		sb.WriteString(m.PushRemoteURL.View())
		sb.WriteString("\n")
		sb.WriteString(styles.FieldLabelStyle.Render("Sync Token:     "))
		sb.WriteString(m.PushRemoteToken.View())
		sb.WriteString("\n")

		includeCfgBox := "[ ]"
		if m.PushIncludeConfig {
			includeCfgBox = styles.BadgeUpdate.Render("[X]")
		}
		dryRunBox := "[ ]"
		if m.PushDryRun {
			dryRunBox = styles.BadgeUpdate.Render("[X]")
		}

		if m.ActiveInputIdx == 2 {
			sb.WriteString(styles.FieldFocusedLabel.Render("Include Config: ") + includeCfgBox + styles.New().Bold(true).Render(" Include server config/ directory (config.zip) [Space to toggle]"))
		} else {
			sb.WriteString(styles.FieldLabelStyle.Render("Include Config: ") + includeCfgBox + " Include server config/ directory (config.zip)")
		}
		sb.WriteString("\n")

		if m.ActiveInputIdx == 3 {
			sb.WriteString(styles.FieldFocusedLabel.Render("Dry Run Mode:   ") + dryRunBox + styles.New().Bold(true).Render(" Dry run mode (simulate only, no server changes) [Space to toggle]"))
		} else {
			sb.WriteString(styles.FieldLabelStyle.Render("Dry Run Mode:   ") + dryRunBox + " Dry run mode (simulate only, no server changes)")
		}
		sb.WriteString("\n\n")

		if m.ActiveInputIdx == 4 {
			sb.WriteString(styles.ButtonActive.Render(" Press [Enter], [p], or [x] to Push to Remote Server "))
		} else {
			sb.WriteString(styles.ButtonInactive.Render(" Press [Enter], [p], or [x] to Push to Remote Server "))
		}

	case EngineDiff:
		sb.WriteString(styles.FieldLabelStyle.Render("Remote URL:   "))
		sb.WriteString(m.DiffRemoteURL.View())
		sb.WriteString("   ")
		sb.WriteString(styles.FieldLabelStyle.Render("Token: "))
		sb.WriteString(m.DiffRemoteToken.View())
		sb.WriteString("\n")
		sb.WriteString(styles.FieldLabelStyle.Render("Or File:      "))
		sb.WriteString(m.DiffCompareFile.View())
		sb.WriteString("   ")
		if m.ActiveInputIdx == 3 {
			sb.WriteString(styles.ButtonActive.Render(" [Run Diff Audit] "))
		} else {
			sb.WriteString(styles.ButtonInactive.Render(" [Run Diff Audit] "))
		}
		sb.WriteString("\n\n")

		sb.WriteString(styles.TableHeaderStyle.Render("── Client vs Server Difference Audit ──"))
		sb.WriteString("\n")
		sb.WriteString(m.DiffTable.View())
		if m.DiffResult != nil {
			sb.WriteString("\n")
			summaryStr := fmt.Sprintf("Summary: %d total mods | %d synchronized | %d mismatches | %d client-only | %d server-only | %d missing",
				m.DiffResult.Total, m.DiffResult.Synchronized, m.DiffResult.Mismatches, m.DiffResult.ClientOnly, m.DiffResult.ServerOnly, m.DiffResult.Missing)
			sb.WriteString(styles.StatusBarDesc.Render(summaryStr))
		}

	case EngineLauncher:
		forceBox := "[ ]"
		if m.LauncherForce {
			forceBox = styles.BadgeUpdate.Render("[X]")
		}
		dryRunBox := "[ ]"
		if m.LauncherDryRun {
			dryRunBox = styles.BadgeUpdate.Render("[X]")
		}

		optStr := fmt.Sprintf("Options: %s Force [f]   %s Dry run [d]   •   Actions: [s/Enter] Sync   [r] Rescan Launchers",
			forceBox, dryRunBox)
		sb.WriteString(styles.HelpDescStyle.Render(optStr))
		sb.WriteString("\n\n")

		sb.WriteString(styles.TableHeaderStyle.Render("── Detected Minecraft Launcher Instances ──"))
		sb.WriteString("\n")
		sb.WriteString(m.LauncherTable.View())
	}

	if m.IsSyncing {
		sb.WriteString("  ")
		sb.WriteString(m.Spinner.View())
		sb.WriteString(" Synchronizing...")
	}

	if m.SelectedEngine != EngineDiff {
		sb.WriteString("\n\n")
		sb.WriteString(styles.TableHeaderStyle.Render("── Execution Output Log ──"))
		sb.WriteString("\n")
		sb.WriteString(m.LogsViewport.View())
	}

	// Status line
	if m.StatusMessage != "" {
		sb.WriteString("\n\n")
		if m.StatusIsError {
			sb.WriteString(styles.StatusError.Render(m.StatusMessage))
		} else {
			sb.WriteString(styles.StatusSuccess.Render(m.StatusMessage))
		}
	}

	return sb.String()
}

