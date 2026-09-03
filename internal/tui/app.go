package tui

import (
	"fmt"
	"strings"

	"cmm/internal/config"
	"cmm/internal/mod"
	"cmm/internal/modrinth"
	"cmm/internal/tui/components"
	"cmm/internal/tui/styles"
	"cmm/internal/tui/tabs"
	"cmm/internal/tui/tea"
)

// AppModel is the root Model for the CMM Terminal UI.
type AppModel struct {
	ConfigPath  string
	LockPath    string
	Config      *config.Config
	Lockfile    *config.Lockfile
	Client      *modrinth.Client
	ModManager  *mod.Manager

	ActiveTab   TabID
	Width       int
	Height      int

	HelpVisible bool
	StatusMsg   string
	StatusIsErr bool

	// Sub-tab Models
	ModsTab   tabs.ModsModel
	SearchTab tabs.SearchModel
	ConfigTab tabs.ConfigModel
	SyncTab   tabs.SyncModel
}

// NewApp constructs the main AppModel.
func NewApp(cfgPath, lockPath string) (AppModel, error) {
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		return AppModel{}, fmt.Errorf("failed to load %s: %w", cfgPath, err)
	}

	lock, _ := config.LoadLockfile(lockPath)
	if lock == nil {
		lock = &config.Lockfile{Mods: []config.LockfileMod{}}
	}

	client, err := modrinth.NewClient("cmm-tui/1.0.0")
	if err != nil {
		client = &modrinth.Client{}
	}

	mgr := mod.NewManager(client, cfgPath, lockPath)

	modsTab := tabs.NewModsModel(cfgPath, lockPath, mgr)
	searchTab := tabs.NewSearchModel(client, mgr, cfg)
	configTab := tabs.NewConfigModel(cfgPath, cfg)
	syncTab := tabs.NewSyncModel(client, cfgPath, lockPath, cfg)

	return AppModel{
		ConfigPath:  cfgPath,
		LockPath:    lockPath,
		Config:      cfg,
		Lockfile:    lock,
		Client:      client,
		ModManager:  mgr,
		ActiveTab:   TabMods,
		Width:       80,
		Height:      24,
		ModsTab:     modsTab,
		SearchTab:   searchTab,
		ConfigTab:   configTab,
		SyncTab:     syncTab,
	}, nil
}

// Init initializes the root app and sub-tabs.
func (a AppModel) Init() tea.Cmd {
	return tea.Batch(
		a.ModsTab.Init(),
		a.SearchTab.Init(),
		a.ConfigTab.Init(),
		a.SyncTab.Init(),
	)
}

// Update handles top-level routing, global keybindings, and passes messages to active tab.
func (a AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.Width = msg.Width
		a.Height = msg.Height
		contentHeight := msg.Height - 6
		if contentHeight < 8 {
			contentHeight = 8
		}

		subSizeMsg := tea.WindowSizeMsg{Width: msg.Width, Height: contentHeight}
		a.ModsTab.SetSize(msg.Width, contentHeight)
		a.SearchTab.SetSize(msg.Width, contentHeight)
		a.ConfigTab.SetSize(msg.Width, contentHeight)
		a.SyncTab.SetSize(msg.Width, contentHeight)
		_ = subSizeMsg

	case ReloadModsMsg:
		cmd := a.ModsTab.ReloadModsCmd()
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case StatusMsg:
		a.StatusMsg = msg.Message
		a.StatusIsErr = msg.IsError

	case SwitchTabMsg:
		a.ActiveTab = msg.Tab

	case tabs.InstallResultMsg:
		// When a mod is installed in search tab, notify mods tab to reload
		if msg.Err == nil {
			cmd := a.ModsTab.ReloadModsCmd()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case tabs.ConfigSavedMsg:
		// When config is saved, refresh search tab config facets
		if msg.Err == nil {
			if cfg, err := config.LoadConfig(a.ConfigPath); err == nil {
				a.Config = cfg
				a.SearchTab.Config = cfg
				a.SyncTab.Config = cfg
			}
		}

	case tea.KeyMsg:
		// Help Modal toggle
		if msg.String() == "?" || msg.Type == tea.KeyF1 {
			// Only toggle if not currently typing in a text input (or if modal already open)
			if a.HelpVisible {
				a.HelpVisible = false
				return a, nil
			}
			if a.ActiveTab != TabSearch && a.ActiveTab != TabConfig && !a.ModsTab.IsFiltering {
				a.HelpVisible = true
				return a, nil
			}
		}

		// Escape closes modals
		if msg.Type == tea.KeyEsc {
			if a.HelpVisible {
				a.HelpVisible = false
				return a, nil
			}
		}

		// Clean quit keybindings (when not typing text in active filter/input)
		if msg.Type == tea.KeyCtrlC || (!a.isTypingInput() && msg.String() == "q") {
			return a, func() tea.Msg { return tea.Quit() }
		}

		// Global Tab navigation when not typing in input
		if !a.isTypingInput() {
			switch msg.String() {
			case "1":
				a.ActiveTab = TabMods
				return a, nil
			case "2":
				a.ActiveTab = TabSearch
				return a, nil
			case "3":
				a.ActiveTab = TabConfig
				return a, nil
			case "4":
				a.ActiveTab = TabSync
				return a, nil
			case "l":
				a.ActiveTab = (a.ActiveTab + 1) % 4
				return a, nil
			case "h":
				if a.ActiveTab == 0 {
					a.ActiveTab = 3
				} else {
					a.ActiveTab--
				}
				return a, nil
			}
		}

		// Tab / Shift+Tab navigation
		if msg.Type == tea.KeyTab && !a.isSubTabHandlingTab() {
			a.ActiveTab = (a.ActiveTab + 1) % 4
			return a, nil
		}
		if msg.Type == tea.KeyShiftTab && !a.isSubTabHandlingTab() {
			if a.ActiveTab == 0 {
				a.ActiveTab = 3
			} else {
				a.ActiveTab--
			}
			return a, nil
		}
	}

	// Route message to active tab
	switch a.ActiveTab {
	case TabMods:
		var cmd tea.Cmd
		a.ModsTab, cmd = a.ModsTab.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case TabSearch:
		var cmd tea.Cmd
		a.SearchTab, cmd = a.SearchTab.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case TabConfig:
		var cmd tea.Cmd
		a.ConfigTab, cmd = a.ConfigTab.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case TabSync:
		var cmd tea.Cmd
		a.SyncTab, cmd = a.SyncTab.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return a, tea.Batch(cmds...)
}

func (a *AppModel) isTypingInput() bool {
	if a.ActiveTab == TabMods && a.ModsTab.IsFiltering {
		return true
	}
	if a.ActiveTab == TabSearch && !a.SearchTab.VersionModalVisible {
		return true
	}
	if a.ActiveTab == TabConfig && a.ConfigTab.ActiveField < tabs.FieldSaveButton {
		return true
	}
	if a.ActiveTab == TabSync {
		return a.SyncTab.IsTypingInput()
	}
	return false
}

func (a *AppModel) isSubTabHandlingTab() bool {
	if a.ActiveTab == TabConfig {
		return true
	}
	if a.ActiveTab == TabSync {
		return a.SyncTab.IsHandlingTab()
	}
	return false
}

// View renders the complete UI shell, header, tabs, active view, footer, and modals.
func (a AppModel) View() string {
	// If Help Modal is open
	if a.HelpVisible {
		return components.RenderHelpModal(a.Width, a.Height)
	}

	var sb strings.Builder

	// Header Bar
	headerTitle := styles.HeaderTitleStyle.Render(" Cloud Mod Manager (cmm) ")
	profileName := "Default"
	mcVer := "1.21.1"
	loader := "fabric"
	if a.Config != nil {
		if a.Config.Profile.Name != "" {
			profileName = a.Config.Profile.Name
		}
		if a.Config.Profile.MinecraftVersion != "" {
			mcVer = a.Config.Profile.MinecraftVersion
		}
		if a.Config.Profile.Loader != "" {
			loader = a.Config.Profile.Loader
		}
	}
	headerInfo := fmt.Sprintf(" Profile: %s | MC: %s | Loader: %s", profileName, mcVer, loader)
	headerInfoStyled := styles.HeaderSubStyle.Render(headerInfo)
	sb.WriteString(headerTitle + headerInfoStyled)
	sb.WriteString("\n\n")

	// Tab Navigation Bar
	tabTitles := []string{"1. Installed Mods", "2. Modrinth Search", "3. Config Editor", "4. Sync Dashboard"}
	var renderedTabs []string
	for i, t := range tabTitles {
		if TabID(i) == a.ActiveTab {
			renderedTabs = append(renderedTabs, styles.ActiveTabStyle.Render(" "+t+" "))
		} else {
			renderedTabs = append(renderedTabs, styles.InactiveTabStyle.Render(" "+t+" "))
		}
	}
	sb.WriteString(strings.Join(renderedTabs, "  "))
	sb.WriteString("\n")
	sb.WriteString(styles.TabDivider.Render(strings.Repeat("─", a.Width)))
	sb.WriteString("\n\n")

	// Main Tab Content View
	switch a.ActiveTab {
	case TabMods:
		sb.WriteString(a.ModsTab.View())
	case TabSearch:
		sb.WriteString(a.SearchTab.View())
	case TabConfig:
		sb.WriteString(a.ConfigTab.View())
	case TabSync:
		sb.WriteString(a.SyncTab.View())
	}

	// Footer Status Bar
	sb.WriteString("\n\n")
	sb.WriteString(styles.TabDivider.Render(strings.Repeat("─", a.Width)))
	sb.WriteString("\n")
	footerKeys := fmt.Sprintf(
		" %s %s  •  %s %s  •  %s %s  •  %s %s",
		styles.StatusBarKey.Render("[Tab/1-4]"),
		styles.StatusBarDesc.Render("Switch Tab"),
		styles.StatusBarKey.Render("[?]"),
		styles.StatusBarDesc.Render("Help"),
		styles.StatusBarKey.Render("[Esc]"),
		styles.StatusBarDesc.Render("Dismiss"),
		styles.StatusBarKey.Render("[q]"),
		styles.StatusBarDesc.Render("Quit"),
	)
	sb.WriteString(footerKeys)

	return sb.String()
}
