package tabs

import (
	"fmt"
	"strings"

	"cmm/internal/config"
	"cmm/internal/tui/components"
	"cmm/internal/tui/styles"
	"cmm/internal/tui/tea"
)

type ConfigField int

const (
	FieldProfileName ConfigField = iota
	FieldMCVersion
	FieldLoader
	FieldLoaderVersion
	FieldSide
	FieldModsDir
	FieldConfigDir
	FieldModrinthToken
	FieldSaveButton
	FieldReloadButton
	FieldCount
)

// ConfigModel manages Tab 3: Configuration & Profile Editor.
type ConfigModel struct {
	ConfigPath    string
	Config        *config.Config
	ActiveField   ConfigField
	Inputs        [FieldCount]components.TextInput
	StatusMessage string
	StatusIsError bool
	Width         int
	Height        int
}

// NewConfigModel initializes Tab 3.
func NewConfigModel(cfgPath string, cfg *config.Config) ConfigModel {
	m := ConfigModel{
		ConfigPath:  cfgPath,
		Config:      cfg,
		ActiveField: FieldProfileName,
		Width:       80,
		Height:      20,
	}

	for i := 0; i < int(FieldCount); i++ {
		m.Inputs[i] = components.NewTextInput()
		m.Inputs[i].Prompt = ""
	}

	m.loadFieldsFromConfig()
	m.updateFocus()
	return m
}

func (m *ConfigModel) loadFieldsFromConfig() {
	if m.Config == nil {
		return
	}

	m.Inputs[FieldProfileName].SetValue(m.Config.Profile.Name)
	m.Inputs[FieldProfileName].Placeholder = "e.g. My Modpack"

	m.Inputs[FieldMCVersion].SetValue(m.Config.Profile.MinecraftVersion)
	m.Inputs[FieldMCVersion].Placeholder = "e.g. 1.21.1"

	m.Inputs[FieldLoader].SetValue(m.Config.Profile.Loader)
	m.Inputs[FieldLoader].Placeholder = "fabric / forge / neoforge / quilt"

	m.Inputs[FieldLoaderVersion].SetValue(m.Config.Profile.LoaderVersion)
	m.Inputs[FieldLoaderVersion].Placeholder = "e.g. 0.16.0 (optional)"

	side := m.Config.Profile.Side
	if side == "" {
		side = "both"
	}
	m.Inputs[FieldSide].SetValue(side)
	m.Inputs[FieldSide].Placeholder = "server / client / both"

	m.Inputs[FieldModsDir].SetValue(m.Config.Paths.ModsDir)
	m.Inputs[FieldModsDir].Placeholder = "e.g. mods"

	m.Inputs[FieldConfigDir].SetValue(m.Config.Paths.ConfigDir)
	m.Inputs[FieldConfigDir].Placeholder = "e.g. config"

	m.Inputs[FieldModrinthToken].SetValue(m.Config.Modrinth.Token)
	m.Inputs[FieldModrinthToken].Placeholder = "Optional Modrinth API Token"
}

func (m *ConfigModel) updateFocus() {
	for i := 0; i < int(FieldCount); i++ {
		if ConfigField(i) == m.ActiveField {
			m.Inputs[i].Focus()
		} else {
			m.Inputs[i].Blur()
		}
	}
}

// Init initializes Tab 3.
func (m ConfigModel) Init() tea.Cmd {
	return nil
}

// SetSize updates dimensions.
func (m *ConfigModel) SetSize(width, height int) {
	m.Width = width
	m.Height = height
}

// ConfigSavedMsg carries save result.
type ConfigSavedMsg struct {
	Err error
}

// ConfigReloadedMsg carries reload result.
type ConfigReloadedMsg struct {
	Config *config.Config
	Err    error
}

// SaveConfigCmd writes config to cmm.toml.
func (m ConfigModel) SaveConfigCmd() tea.Cmd {
	return func() tea.Msg {
		// Validate
		name := strings.TrimSpace(m.Inputs[FieldProfileName].Value())
		if name == "" {
			return ConfigSavedMsg{Err: fmt.Errorf("profile name cannot be empty")}
		}

		mc := strings.TrimSpace(m.Inputs[FieldMCVersion].Value())
		if mc == "" {
			return ConfigSavedMsg{Err: fmt.Errorf("minecraft version cannot be empty")}
		}

		loader := strings.ToLower(strings.TrimSpace(m.Inputs[FieldLoader].Value()))
		if loader != "fabric" && loader != "forge" && loader != "neoforge" && loader != "quilt" {
			return ConfigSavedMsg{Err: fmt.Errorf("unsupported loader '%s' (must be fabric, forge, neoforge, or quilt)", loader)}
		}

		side := strings.ToLower(strings.TrimSpace(m.Inputs[FieldSide].Value()))
		if side != "server" && side != "client" && side != "both" {
			return ConfigSavedMsg{Err: fmt.Errorf("invalid side '%s' (must be server, client, or both)", side)}
		}

		// Update config in-memory
		if m.Config == nil {
			m.Config = config.DefaultConfig()
		}

		m.Config.Profile.Name = name
		m.Config.Profile.MinecraftVersion = mc
		m.Config.Profile.Loader = loader
		m.Config.Profile.LoaderVersion = strings.TrimSpace(m.Inputs[FieldLoaderVersion].Value())
		m.Config.Profile.Side = side

		modsDir := strings.TrimSpace(m.Inputs[FieldModsDir].Value())
		if modsDir == "" {
			modsDir = "mods"
		}
		m.Config.Paths.ModsDir = modsDir

		cfgDir := strings.TrimSpace(m.Inputs[FieldConfigDir].Value())
		if cfgDir == "" {
			cfgDir = "config"
		}
		m.Config.Paths.ConfigDir = cfgDir
		m.Config.Modrinth.Token = strings.TrimSpace(m.Inputs[FieldModrinthToken].Value())

		err := config.SaveConfig(m.ConfigPath, m.Config)
		return ConfigSavedMsg{Err: err}
	}
}

// ReloadConfigCmd loads config from cmm.toml.
func (m ConfigModel) ReloadConfigCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.LoadConfig(m.ConfigPath)
		return ConfigReloadedMsg{Config: cfg, Err: err}
	}
}

// Update handles state transitions for Tab 3.
func (m ConfigModel) Update(msg tea.Msg) (ConfigModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case ConfigSavedMsg:
		if msg.Err != nil {
			m.StatusMessage = fmt.Sprintf("Error saving configuration: %v", msg.Err)
			m.StatusIsError = true
		} else {
			m.StatusMessage = "Configuration saved successfully to cmm.toml"
			m.StatusIsError = false
		}

	case ConfigReloadedMsg:
		if msg.Err != nil {
			m.StatusMessage = fmt.Sprintf("Error reloading configuration: %v", msg.Err)
			m.StatusIsError = true
		} else {
			m.Config = msg.Config
			m.loadFieldsFromConfig()
			m.StatusMessage = "Configuration reloaded from disk"
			m.StatusIsError = false
		}

	case tea.KeyMsg:
		// Save shortcut 's' when on button or global key
		if msg.String() == "s" && (m.ActiveField == FieldSaveButton || m.ActiveField == FieldReloadButton) {
			return m, m.SaveConfigCmd()
		}

		// Reload shortcut 'r' when on button or global key
		if msg.String() == "r" && (m.ActiveField == FieldSaveButton || m.ActiveField == FieldReloadButton) {
			return m, m.ReloadConfigCmd()
		}

		switch msg.Type {
		case tea.KeyUp:
			if m.ActiveField > 0 {
				m.ActiveField--
				m.updateFocus()
			}
			return m, nil

		case tea.KeyDown:
			if m.ActiveField < FieldCount-1 {
				m.ActiveField++
				m.updateFocus()
			}
			return m, nil

		case tea.KeyTab:
			m.ActiveField = (m.ActiveField + 1) % FieldCount
			m.updateFocus()
			return m, nil

		case tea.KeyShiftTab:
			if m.ActiveField == 0 {
				m.ActiveField = FieldCount - 1
			} else {
				m.ActiveField--
			}
			m.updateFocus()
			return m, nil

		case tea.KeyEnter:
			if m.ActiveField == FieldSaveButton {
				return m, m.SaveConfigCmd()
			} else if m.ActiveField == FieldReloadButton {
				return m, m.ReloadConfigCmd()
			} else {
				// Move to next field
				m.ActiveField = (m.ActiveField + 1) % FieldCount
				m.updateFocus()
				return m, nil
			}

		case tea.KeySpace:
			// Cycle Loader options on Space
			if m.ActiveField == FieldLoader {
				loaders := []string{"fabric", "forge", "neoforge", "quilt"}
				curr := strings.ToLower(m.Inputs[FieldLoader].Value())
				next := loaders[0]
				for i, l := range loaders {
					if l == curr {
						next = loaders[(i+1)%len(loaders)]
						break
					}
				}
				m.Inputs[FieldLoader].SetValue(next)
				return m, nil
			}

			// Cycle Side options on Space
			if m.ActiveField == FieldSide {
				sides := []string{"both", "server", "client"}
				curr := strings.ToLower(m.Inputs[FieldSide].Value())
				next := sides[0]
				for i, s := range sides {
					if s == curr {
						next = sides[(i+1)%len(sides)]
						break
					}
				}
				m.Inputs[FieldSide].SetValue(next)
				return m, nil
			}
		}

		// Text input typing for active field
		if m.ActiveField < FieldSaveButton {
			var cmd tea.Cmd
			m.Inputs[m.ActiveField], cmd = m.Inputs[m.ActiveField].Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders Tab 3.
func (m ConfigModel) View() string {
	var sb strings.Builder

	sb.WriteString(styles.HelpDescStyle.Render("Edit profile configuration for cmm.toml  •  [Tab/Arrows] Navigate  •  [Space] Cycle options"))
	sb.WriteString("\n\n")

	renderField := func(label string, field ConfigField, extraHint string) {
		focused := m.ActiveField == field
		lblStyle := styles.FieldLabelStyle
		if focused {
			lblStyle = styles.FieldFocusedLabel
		}

		prefix := "  "
		if focused {
			prefix = "▶ "
		}

		lbl := lblStyle.Render(fmt.Sprintf("%-22s", label))
		val := m.Inputs[field].View()
		sb.WriteString(fmt.Sprintf("%s%s %s", prefix, lbl, val))
		if extraHint != "" {
			sb.WriteString("  " + styles.TableRowDim.Render(extraHint))
		}
		sb.WriteString("\n")
	}

	renderField("Profile Name:", FieldProfileName, "")
	renderField("Minecraft Version:", FieldMCVersion, "")
	renderField("Mod Loader:", FieldLoader, "(Space to toggle: fabric, forge, neoforge, quilt)")
	renderField("Loader Version:", FieldLoaderVersion, "(Optional)")
	renderField("Environment Side:", FieldSide, "(Space to toggle: both, server, client)")
	renderField("Mods Directory:", FieldModsDir, "")
	renderField("Config Directory:", FieldConfigDir, "")
	renderField("Modrinth API Token:", FieldModrinthToken, "")

	sb.WriteString("\n")

	// Buttons
	savePrefix := "  "
	if m.ActiveField == FieldSaveButton {
		savePrefix = "▶ "
		sb.WriteString(savePrefix + styles.ButtonActive.Render(" Save Changes (s) "))
	} else {
		sb.WriteString(savePrefix + styles.ButtonInactive.Render(" Save Changes (s) "))
	}

	sb.WriteString("   ")

	reloadPrefix := ""
	if m.ActiveField == FieldReloadButton {
		reloadPrefix = "▶ "
		sb.WriteString(reloadPrefix + styles.ButtonActive.Render(" Reload Disk (r) "))
	} else {
		sb.WriteString(reloadPrefix + styles.ButtonInactive.Render(" Reload Disk (r) "))
	}

	sb.WriteString("\n\n")

	// Status Message
	if m.StatusMessage != "" {
		if m.StatusIsError {
			sb.WriteString(styles.StatusError.Render(m.StatusMessage))
		} else {
			sb.WriteString(styles.StatusSuccess.Render(m.StatusMessage))
		}
	}

	return sb.String()
}
