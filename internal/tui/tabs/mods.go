package tabs

import (
	"fmt"
	"strings"

	"cmm/internal/config"
	"cmm/internal/mod"
	"cmm/internal/tui/components"
	"cmm/internal/tui/styles"
	"cmm/internal/tui/tea"
)

// ModsModel manages Tab 1: Installed Mods.
type ModsModel struct {
	ConfigPath          string
	LockPath            string
	ModManager          *mod.Manager
	Mods                []mod.ModStatus
	FilteredMods        []mod.ModStatus
	Table               components.Table
	FilterInput         components.TextInput
	IsFiltering         bool
	Loading             bool
	DeleteModalVisible  bool
	ModToDelete         *mod.ModStatus
	DetailsModalVisible bool
	SelectedMod         *mod.ModStatus
	StatusMessage       string
	StatusIsError       bool
	Width               int
	Height              int
}

// NewModsModel initializes Tab 1.
func NewModsModel(cfgPath, lockPath string, mgr *mod.Manager) ModsModel {
	cols := []components.Column{
		{Title: "STATUS", Width: 10},
		{Title: "NAME", Width: 26},
		{Title: "SLUG", Width: 20},
		{Title: "VERSION", Width: 16},
		{Title: "SIDE", Width: 8},
	}

	filter := components.NewTextInput()
	filter.Prompt = "Filter: "
	filter.Placeholder = "Type to filter mods..."
	filter.CharLimit = 64

	return ModsModel{
		ConfigPath:  cfgPath,
		LockPath:    lockPath,
		ModManager:  mgr,
		Table:       components.NewTable(cols, 15),
		FilterInput: filter,
		Width:       80,
		Height:      20,
	}
}

// Init loads installed mods from cmm.lock.
func (m ModsModel) Init() tea.Cmd {
	return m.ReloadModsCmd()
}

// ReloadModsCmd returns a command to reload mods.
func (m ModsModel) ReloadModsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.ModManager == nil {
			return ModsLoadedMsg{Err: fmt.Errorf("mod manager not initialized")}
		}
		mods, err := m.ModManager.List()
		return ModsLoadedMsg{Mods: mods, Err: err}
	}
}

// ModsLoadedMsg carries the loaded mods slice.
type ModsLoadedMsg struct {
	Mods []mod.ModStatus
	Err  error
}

// PinToggledMsg notifies of pin toggle result.
type PinToggledMsg struct {
	Slug   string
	Pinned bool
	Err    error
}

// EnableToggledMsg notifies of enable/disable result.
type EnableToggledMsg struct {
	Slug     string
	Disabled bool
	Err      error
}

// ModDeletedResultMsg notifies of mod deletion result.
type ModDeletedResultMsg struct {
	Slug string
	Err  error
}

// UpdateCheckResultMsg carries update check results.
type UpdateCheckResultMsg struct {
	Candidates []mod.UpdateCandidate
	Updated    bool
	Err        error
}

// SetSize updates dimensions.
func (m *ModsModel) SetSize(width, height int) {
	m.Width = width
	m.Height = height
	tableHeight := height - 6
	if tableHeight < 5 {
		tableHeight = 5
	}
	m.Table.SetHeight(tableHeight)
	m.Table.SetWidth(width)

	// Adjust column widths dynamically
	slugWidth := 18
	verWidth := 14
	sideWidth := 8
	statusWidth := 10
	nameWidth := width - (slugWidth + verWidth + sideWidth + statusWidth + 10)
	if nameWidth < 15 {
		nameWidth = 15
	}

	m.Table.SetColumns([]components.Column{
		{Title: "STATUS", Width: statusWidth},
		{Title: "NAME", Width: nameWidth},
		{Title: "SLUG", Width: slugWidth},
		{Title: "VERSION", Width: verWidth},
		{Title: "SIDE", Width: sideWidth},
	})
}

// Update handles state transitions for Tab 1.
func (m ModsModel) Update(msg tea.Msg) (ModsModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case ModsLoadedMsg:
		m.Loading = false
		if msg.Err != nil {
			m.StatusMessage = fmt.Sprintf("Error loading mods: %v", msg.Err)
			m.StatusIsError = true
		} else {
			m.Mods = msg.Mods
			m.ApplyFilter()
		}

	case PinToggledMsg:
		if msg.Err != nil {
			m.StatusMessage = fmt.Sprintf("Pin error: %v", msg.Err)
			m.StatusIsError = true
		} else {
			status := "pinned"
			if !msg.Pinned {
				status = "unpinned"
			}
			m.StatusMessage = fmt.Sprintf("Mod '%s' successfully %s", msg.Slug, status)
			m.StatusIsError = false
			return m, m.ReloadModsCmd()
		}

	case EnableToggledMsg:
		if msg.Err != nil {
			m.StatusMessage = fmt.Sprintf("Toggle error: %v", msg.Err)
			m.StatusIsError = true
		} else {
			status := "enabled"
			if msg.Disabled {
				status = "disabled"
			}
			m.StatusMessage = fmt.Sprintf("Mod '%s' successfully %s", msg.Slug, status)
			m.StatusIsError = false
			return m, m.ReloadModsCmd()
		}

	case ModDeletedResultMsg:
		m.DeleteModalVisible = false
		m.ModToDelete = nil
		if msg.Err != nil {
			m.StatusMessage = fmt.Sprintf("Delete error: %v", msg.Err)
			m.StatusIsError = true
		} else {
			m.StatusMessage = fmt.Sprintf("Deleted mod '%s'", msg.Slug)
			m.StatusIsError = false
			return m, m.ReloadModsCmd()
		}

	case UpdateCheckResultMsg:
		if msg.Err != nil {
			m.StatusMessage = fmt.Sprintf("Update error: %v", msg.Err)
			m.StatusIsError = true
		} else if len(msg.Candidates) == 0 {
			m.StatusMessage = "All mods are up to date!"
			m.StatusIsError = false
		} else {
			m.StatusMessage = fmt.Sprintf("Applied updates for %d mod(s)", len(msg.Candidates))
			m.StatusIsError = false
			return m, m.ReloadModsCmd()
		}

	case tea.KeyMsg:
		// If Delete Modal is visible
		if m.DeleteModalVisible {
			switch msg.String() {
			case "y", "Y", "enter":
				if m.ModToDelete != nil && m.ModManager != nil {
					slug := m.ModToDelete.Slug
					return m, func() tea.Msg {
						_, err := m.ModManager.Remove(slug, false)
						return ModDeletedResultMsg{Slug: slug, Err: err}
					}
				}
				m.DeleteModalVisible = false
			case "n", "N", "esc", "q":
				m.DeleteModalVisible = false
				m.ModToDelete = nil
				return m, nil
			}
			return m, nil
		}

		// If Details Modal is visible
		if m.DetailsModalVisible {
			if msg.Type == tea.KeyEsc || msg.Type == tea.KeyEnter || msg.String() == "q" {
				m.DetailsModalVisible = false
				m.SelectedMod = nil
				return m, nil
			}
			return m, nil
		}

		// If Filter Input is active
		if m.IsFiltering {
			if msg.Type == tea.KeyEsc {
				m.IsFiltering = false
				m.FilterInput.Blur()
				m.FilterInput.Reset()
				m.ApplyFilter()
				return m, nil
			}
			if msg.Type == tea.KeyEnter {
				m.IsFiltering = false
				m.FilterInput.Blur()
				return m, nil
			}

			var cmd tea.Cmd
			m.FilterInput, cmd = m.FilterInput.Update(msg)
			m.ApplyFilter()
			return m, cmd
		}

		// Normal List Navigation & Actions
		switch msg.String() {
		case "/":
			m.IsFiltering = true
			m.FilterInput.Focus()
			return m, nil

		case "e", " ":
			// Toggle enable/disable for selected mod
			curr := m.CurrentSelectedMod()
			if curr != nil && m.ModManager != nil {
				slug := curr.Slug
				wasDisabled := curr.Disabled
				return m, func() tea.Msg {
					var err error
					if wasDisabled {
						_, err = m.ModManager.EnableMod(slug)
					} else {
						_, err = m.ModManager.DisableMod(slug, true)
					}
					return EnableToggledMsg{Slug: slug, Disabled: !wasDisabled, Err: err}
				}
			}

		case "p":
			// Toggle pin for selected mod
			curr := m.CurrentSelectedMod()
			if curr != nil && m.ModManager != nil {
				slug := curr.Slug
				newPinned := !curr.Pinned
				return m, func() tea.Msg {
					var err error
					if newPinned {
						err = m.ModManager.Pin(slug, curr.Version)
					} else {
						_, err = m.ModManager.Unpin(slug)
					}
					return PinToggledMsg{Slug: slug, Pinned: newPinned, Err: err}
				}
			}

		case "d", "x":
			// Open delete confirmation modal
			curr := m.CurrentSelectedMod()
			if curr != nil {
				m.DeleteModalVisible = true
				m.ModToDelete = curr
				return m, nil
			}

		case "u":
			// Check / apply updates
			curr := m.CurrentSelectedMod()
			if m.ModManager != nil {
				slug := ""
				if curr != nil {
					slug = curr.Slug
				}
				m.StatusMessage = "Checking for updates..."
				m.StatusIsError = false
				return m, func() tea.Msg {
					candidates, _, err := m.ModManager.CheckUpdates(slug, false)
					if err != nil {
						return UpdateCheckResultMsg{Err: err}
					}
					if len(candidates) > 0 {
						applyErr := m.ModManager.ApplyUpdates(candidates)
						return UpdateCheckResultMsg{Candidates: candidates, Updated: true, Err: applyErr}
					}
					return UpdateCheckResultMsg{Candidates: candidates, Updated: false}
				}
			}

		case "enter":
			// View detailed mod metadata
			curr := m.CurrentSelectedMod()
			if curr != nil {
				m.DetailsModalVisible = true
				m.SelectedMod = curr
				return m, nil
			}

		default:
			// Pass to Table
			var cmd tea.Cmd
			m.Table, cmd = m.Table.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// CurrentSelectedMod returns the ModStatus for the currently selected table row.
func (m ModsModel) CurrentSelectedMod() *mod.ModStatus {
	idx := m.Table.Cursor()
	if idx >= 0 && idx < len(m.FilteredMods) {
		return &m.FilteredMods[idx]
	}
	return nil
}

// ApplyFilter filters m.Mods according to FilterInput value and updates Table rows.
func (m *ModsModel) ApplyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.FilterInput.Value()))
	var filtered []mod.ModStatus

	for _, modItem := range m.Mods {
		if query == "" ||
			strings.Contains(strings.ToLower(modItem.Name), query) ||
			strings.Contains(strings.ToLower(modItem.Slug), query) {
			filtered = append(filtered, modItem)
		}
	}

	m.FilteredMods = filtered

	// Build rows for Table
	var rows [][]string
	for _, modItem := range filtered {
		statusBadge := "[OK]"
		if modItem.Disabled {
			statusBadge = "[DISABLED]"
		} else if modItem.Pinned {
			statusBadge = "[PIN]"
		} else if modItem.UpdateAvailable {
			statusBadge = "[UPDATE]"
		}

		sideStr := config.NormalizeSide(modItem.Side)

		rows = append(rows, []string{
			statusBadge,
			modItem.Name,
			modItem.Slug,
			modItem.Version,
			sideStr,
		})
	}

	m.Table.SetRows(rows)
}

// View renders Tab 1.
func (m ModsModel) View() string {
	// If delete confirmation modal is open
	if m.DeleteModalVisible && m.ModToDelete != nil {
		content := fmt.Sprintf(
			"Are you sure you want to delete '%s' (%s)?\n\nThis will remove the file from mods/ and update cmm.lock.",
			m.ModToDelete.Name, m.ModToDelete.Slug,
		)
		return components.RenderModal("Delete Mod Confirmation", content, "[y] Confirm  •  [n/Esc] Cancel", 58, 12, m.Width, m.Height)
	}

	// If details modal is open
	if m.DetailsModalVisible && m.SelectedMod != nil {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Name:        %s\n", styles.New().Bold(true).Foreground(styles.ColorWhite).Render(m.SelectedMod.Name)))
		sb.WriteString(fmt.Sprintf("Slug:        %s\n", styles.New().Foreground(styles.ColorCyan).Render(m.SelectedMod.Slug)))
		sb.WriteString(fmt.Sprintf("Version:     %s\n", m.SelectedMod.Version))
		stateStr := "Active"
		if m.SelectedMod.Disabled {
			stateStr = "Disabled (.jar.disabled)"
		}
		sb.WriteString(fmt.Sprintf("State:       %s\n", stateStr))
		sb.WriteString(fmt.Sprintf("Environment: %s\n", m.SelectedMod.Side))
		sb.WriteString(fmt.Sprintf("Pinned:      %t\n", m.SelectedMod.Pinned))
		sb.WriteString(fmt.Sprintf("Update:      %t\n", m.SelectedMod.UpdateAvailable))
		if m.SelectedMod.LatestVersion != "" {
			sb.WriteString(fmt.Sprintf("Latest Ver:  %s\n", styles.New().Bold(true).Foreground(styles.ColorGreen).Render(m.SelectedMod.LatestVersion)))
		}

		// Read lockfile entry for hashes, client/server details and download URL
		if lock, err := config.LoadLockfile(m.LockPath); err == nil {
			if entry := lock.GetMod(m.SelectedMod.Slug); entry != nil {
				if entry.ClientSide != "" || entry.ServerSide != "" {
					sb.WriteString(fmt.Sprintf("Client Side: %s\n", entry.ClientSide))
					sb.WriteString(fmt.Sprintf("Server Side: %s\n", entry.ServerSide))
				}
				if entry.FileName != "" {
					sb.WriteString(fmt.Sprintf("File Name:   %s\n", entry.FileName))
				}
				if entry.SHA512 != "" {
					shaShort := entry.SHA512
					if len(shaShort) > 32 {
						shaShort = shaShort[:32] + "…"
					}
					sb.WriteString(fmt.Sprintf("SHA-512:     %s\n", shaShort))
				}
				if entry.DownloadURL != "" {
					urlShort := entry.DownloadURL
					if len(urlShort) > 40 {
						urlShort = urlShort[:40] + "…"
					}
					sb.WriteString(fmt.Sprintf("Source URL:  %s\n", urlShort))
				}
			}
		}

		return components.RenderModal("Mod Details", strings.TrimSpace(sb.String()), "Press Esc or Enter to close", 66, 19, m.Width, m.Height)
	}

	var sb strings.Builder

	// Top control bar (Filter / Count)
	countStr := fmt.Sprintf("(%d mods installed)", len(m.Mods))
	if m.IsFiltering || m.FilterInput.Value() != "" {
		sb.WriteString(m.FilterInput.View())
		sb.WriteString("  ")
		sb.WriteString(styles.TableRowDim.Render(countStr))
	} else {
		filterHint := styles.HelpDescStyle.Render("Press [/] to filter  •  [e/Space] Toggle  •  [p] Pin  •  [d] Delete  •  [u] Update  •  [Enter] Details")
		sb.WriteString(filterHint)
		sb.WriteString("  ")
		sb.WriteString(styles.TableRowDim.Render(countStr))
	}
	sb.WriteString("\n\n")

	// Table
	sb.WriteString(m.Table.View())

	// Status Message
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
