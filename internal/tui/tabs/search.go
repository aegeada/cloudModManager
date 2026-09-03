package tabs

import (
	"fmt"
	"strings"
	"time"

	"cmm/internal/config"
	"cmm/internal/mod"
	"cmm/internal/modrinth"
	"cmm/internal/tui/components"
	"cmm/internal/tui/styles"
	"cmm/internal/tui/tea"
)

// SearchModel manages Tab 2: Modrinth Search & Install.
type SearchModel struct {
	Client              *modrinth.Client
	ModManager          *mod.Manager
	Config              *config.Config
	SearchInput         components.TextInput
	ResultsTable        components.Table
	Hits                []modrinth.SearchHit
	TotalHits           int
	Offset              int
	Limit               int
	DebounceSeq         int
	IsSearching         bool
	IsInstalling        bool
	InstallProgress     string
	Spinner             components.Spinner
	VersionModalVisible bool
	SelectedHit         *modrinth.SearchHit
	AvailableVersions   []modrinth.Version
	VersionTable        components.Table
	StatusMessage       string
	StatusIsError       bool
	Width               int
	Height              int
}

// NewSearchModel initializes Tab 2.
func NewSearchModel(client *modrinth.Client, mgr *mod.Manager, cfg *config.Config) SearchModel {
	input := components.NewTextInput()
	input.Prompt = "Search: "
	input.Placeholder = "Search Modrinth mods (e.g. sodium, lithium, iris)..."
	input.Focus()

	cols := []components.Column{
		{Title: "TITLE", Width: 24},
		{Title: "AUTHOR", Width: 16},
		{Title: "DOWNLOADS", Width: 12},
		{Title: "CATEGORIES", Width: 18},
		{Title: "DESCRIPTION", Width: 32},
	}

	vCols := []components.Column{
		{Title: "VERSION", Width: 18},
		{Title: "NAME", Width: 28},
		{Title: "GAME VERSIONS", Width: 18},
		{Title: "LOADERS", Width: 16},
	}

	return SearchModel{
		Client:        client,
		ModManager:    mgr,
		Config:        cfg,
		SearchInput:   input,
		ResultsTable:  components.NewTable(cols, 14),
		VersionTable:  components.NewTable(vCols, 8),
		Spinner:       components.NewSpinner(),
		Limit:         20,
		Offset:        0,
		Width:         80,
		Height:        20,
		StatusMessage: "Type a search query to discover and install mods",
	}
}

// Init initializes Tab 2.
func (m SearchModel) Init() tea.Cmd {
	return nil
}

// SetSize updates dimensions.
func (m *SearchModel) SetSize(width, height int) {
	m.Width = width
	m.Height = height
	tableHeight := height - 7
	if tableHeight < 5 {
		tableHeight = 5
	}
	m.ResultsTable.SetHeight(tableHeight)
	m.ResultsTable.SetWidth(width)

	// Adjust column widths
	authorW := 14
	dlW := 11
	catW := 16
	titleW := 22
	descW := width - (authorW + dlW + catW + titleW + 12)
	if descW < 15 {
		descW = 15
	}

	m.ResultsTable.SetColumns([]components.Column{
		{Title: "TITLE", Width: titleW},
		{Title: "AUTHOR", Width: authorW},
		{Title: "DOWNLOADS", Width: dlW},
		{Title: "CATEGORIES", Width: catW},
		{Title: "DESCRIPTION", Width: descW},
	})
}

// SearchDebounceMsg is sent after debounce duration.
type SearchDebounceMsg struct {
	Query string
	Seq   int
}

// SearchResultsMsg carries search response.
type SearchResultsMsg struct {
	Query     string
	Seq       int
	Hits      []modrinth.SearchHit
	TotalHits int
	Err       error
}

// VersionsLoadedMsg carries project versions.
type VersionsLoadedMsg struct {
	Hit      *modrinth.SearchHit
	Versions []modrinth.Version
	Err      error
}

// InstallResultMsg carries install result.
type InstallResultMsg struct {
	Slug    string
	Version string
	Result  *mod.AddResult
	Err     error
}

// PerformSearchCmd executes Modrinth API search.
func (m SearchModel) PerformSearchCmd(query string, seq int) tea.Cmd {
	return func() tea.Msg {
		if m.Client == nil || strings.TrimSpace(query) == "" {
			return SearchResultsMsg{Query: query, Seq: seq, Hits: nil, TotalHits: 0}
		}

		facets := [][]string{{"project_type:mod"}}
		if m.Config != nil {
			if m.Config.Profile.Loader != "" {
				facets = append(facets, []string{"categories:" + strings.ToLower(m.Config.Profile.Loader)})
			}
			if m.Config.Profile.MinecraftVersion != "" {
				facets = append(facets, []string{"versions:" + m.Config.Profile.MinecraftVersion})
			}
		}

		resp, err := m.Client.SearchWithOptions(query, facets, "relevance", m.Offset, m.Limit)
		if err != nil {
			return SearchResultsMsg{Query: query, Seq: seq, Err: err}
		}

		return SearchResultsMsg{
			Query:     query,
			Seq:       seq,
			Hits:      resp.Hits,
			TotalHits: resp.TotalHits,
		}
	}
}

// FetchVersionsCmd fetches compatible versions for a hit.
func (m SearchModel) FetchVersionsCmd(hit *modrinth.SearchHit) tea.Cmd {
	return func() tea.Msg {
		if m.Client == nil || hit == nil {
			return VersionsLoadedMsg{Hit: hit, Err: fmt.Errorf("client or hit missing")}
		}

		var loaders []string
		var gameVersions []string
		if m.Config != nil {
			if m.Config.Profile.Loader != "" {
				loaders = []string{strings.ToLower(m.Config.Profile.Loader)}
			}
			if m.Config.Profile.MinecraftVersion != "" {
				gameVersions = []string{m.Config.Profile.MinecraftVersion}
			}
		}

		versions, err := m.Client.GetProjectVersions(hit.Slug, loaders, gameVersions, nil)
		return VersionsLoadedMsg{Hit: hit, Versions: versions, Err: err}
	}
}

// InstallModCmd triggers recursive mod installation.
func (m SearchModel) InstallModCmd(slug, version string) tea.Cmd {
	return func() tea.Msg {
		if m.ModManager == nil {
			return InstallResultMsg{Slug: slug, Version: version, Err: fmt.Errorf("mod manager not initialized")}
		}

		res, err := m.ModManager.Add(slug, version)
		return InstallResultMsg{Slug: slug, Version: version, Result: res, Err: err}
	}
}

// Update handles state transitions for Tab 2.
func (m SearchModel) Update(msg tea.Msg) (SearchModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case components.SpinnerTickMsg:
		if m.IsInstalling || m.IsSearching {
			var cmd tea.Cmd
			m.Spinner, cmd = m.Spinner.Update(msg)
			return m, cmd
		}

	case SearchDebounceMsg:
		if msg.Seq == m.DebounceSeq {
			m.IsSearching = true
			return m, tea.Batch(m.PerformSearchCmd(msg.Query, msg.Seq), m.Spinner.Tick())
		}

	case SearchResultsMsg:
		if msg.Seq == m.DebounceSeq {
			m.IsSearching = false
			if msg.Err != nil {
				m.StatusMessage = fmt.Sprintf("Search error: %v", msg.Err)
				m.StatusIsError = true
			} else {
				m.Hits = msg.Hits
				m.TotalHits = msg.TotalHits
				m.buildResultsRows()
				m.StatusMessage = fmt.Sprintf("Found %d results for '%s'", msg.TotalHits, msg.Query)
				m.StatusIsError = false
			}
		}

	case VersionsLoadedMsg:
		if msg.Err != nil {
			m.StatusMessage = fmt.Sprintf("Error fetching versions: %v", msg.Err)
			m.StatusIsError = true
		} else if len(msg.Versions) == 0 {
			m.StatusMessage = fmt.Sprintf("No compatible versions found for '%s'", msg.Hit.Title)
			m.StatusIsError = true
		} else {
			m.SelectedHit = msg.Hit
			m.AvailableVersions = msg.Versions
			m.VersionModalVisible = true
			m.buildVersionRows()
		}

	case InstallResultMsg:
		m.IsInstalling = false
		m.VersionModalVisible = false
		if msg.Err != nil {
			m.StatusMessage = fmt.Sprintf("Install error: %v", msg.Err)
			m.StatusIsError = true
		} else {
			depCount := 0
			if msg.Result != nil {
				depCount = len(msg.Result.InstalledDeps)
			}
			m.StatusMessage = fmt.Sprintf("Successfully installed '%s' (%d dependencies resolved)", msg.Slug, depCount)
			m.StatusIsError = false
		}

	case tea.KeyMsg:
		// Version Picker Modal Navigation
		if m.VersionModalVisible {
			switch msg.String() {
			case "esc", "q":
				m.VersionModalVisible = false
				m.SelectedHit = nil
				return m, nil
			case "enter", "i":
				// Install selected version
				if len(m.AvailableVersions) > 0 && m.SelectedHit != nil {
					idx := m.VersionTable.Cursor()
					if idx >= 0 && idx < len(m.AvailableVersions) {
						v := m.AvailableVersions[idx]
						m.IsInstalling = true
						m.InstallProgress = fmt.Sprintf("Installing %s v%s...", m.SelectedHit.Title, v.VersionNumber)
						return m, tea.Batch(m.InstallModCmd(m.SelectedHit.Slug, v.VersionNumber), m.Spinner.Tick())
					}
				}
			default:
				var cmd tea.Cmd
				m.VersionTable, cmd = m.VersionTable.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				return m, tea.Batch(cmds...)
			}
			return m, nil
		}

		// When Results Table is focused or Arrow navigation
		switch msg.Type {
		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
			var cmd tea.Cmd
			m.ResultsTable, cmd = m.ResultsTable.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "enter":
			// Open Version Picker for selected search hit
			curr := m.CurrentSelectedHit()
			if curr != nil {
				m.StatusMessage = fmt.Sprintf("Fetching versions for %s...", curr.Title)
				m.StatusIsError = false
				return m, m.FetchVersionsCmd(curr)
			}

		case "i":
			// Fast install latest compatible version
			curr := m.CurrentSelectedHit()
			if curr != nil {
				m.IsInstalling = true
				m.InstallProgress = fmt.Sprintf("Installing latest %s...", curr.Title)
				return m, tea.Batch(m.InstallModCmd(curr.Slug, ""), m.Spinner.Tick())
			}

		default:
			// Text input typing with live debounce
			prevVal := m.SearchInput.Value()
			var cmd tea.Cmd
			m.SearchInput, cmd = m.SearchInput.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

			if m.SearchInput.Value() != prevVal {
				m.DebounceSeq++
				currSeq := m.DebounceSeq
				currQuery := m.SearchInput.Value()
				debounceCmd := tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
					return SearchDebounceMsg{Query: currQuery, Seq: currSeq}
				})
				cmds = append(cmds, debounceCmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *SearchModel) CurrentSelectedHit() *modrinth.SearchHit {
	if len(m.Hits) == 0 {
		return nil
	}
	idx := m.ResultsTable.Cursor()
	if idx >= 0 && idx < len(m.Hits) {
		return &m.Hits[idx]
	}
	return nil
}

func (m *SearchModel) buildResultsRows() {
	var rows [][]string
	for _, hit := range m.Hits {
		dlStr := fmt.Sprintf("%d", hit.Downloads)
		if hit.Downloads >= 1000000 {
			dlStr = fmt.Sprintf("%.1fM", float64(hit.Downloads)/1000000)
		} else if hit.Downloads >= 1000 {
			dlStr = fmt.Sprintf("%.1fk", float64(hit.Downloads)/1000)
		}

		cats := strings.Join(hit.Categories, ", ")
		desc := strings.ReplaceAll(hit.Description, "\n", " ")

		rows = append(rows, []string{
			hit.Title,
			hit.Author,
			dlStr,
			cats,
			desc,
		})
	}
	m.ResultsTable.SetRows(rows)
}

func (m *SearchModel) buildVersionRows() {
	var rows [][]string
	for _, v := range m.AvailableVersions {
		gv := strings.Join(v.GameVersions, ", ")
		ld := strings.Join(v.Loaders, ", ")
		rows = append(rows, []string{
			v.VersionNumber,
			v.Name,
			gv,
			ld,
		})
	}
	m.VersionTable.SetRows(rows)
}

// View renders Tab 2.
func (m SearchModel) View() string {
	// Version Picker Modal
	if m.VersionModalVisible && m.SelectedHit != nil {
		var content strings.Builder
		content.WriteString(fmt.Sprintf("Select compatible version for %s:\n\n", styles.New().Bold(true).Foreground(styles.ColorCyan).Render(m.SelectedHit.Title)))
		content.WriteString(m.VersionTable.View())
		return components.RenderModal("Version Picker", content.String(), "[Enter/i] Install Selected  •  [Esc] Cancel", 74, 16, m.Width, m.Height)
	}

	var sb strings.Builder

	// Top search input & status
	sb.WriteString(m.SearchInput.View())
	if m.IsSearching {
		sb.WriteString("  ")
		sb.WriteString(m.Spinner.View())
		sb.WriteString(" Searching Modrinth...")
	} else if m.IsInstalling {
		sb.WriteString("  ")
		sb.WriteString(m.Spinner.View())
		sb.WriteString(" " + m.InstallProgress)
	}
	sb.WriteString("\n\n")

	// Results Table
	sb.WriteString(m.ResultsTable.View())

	// Status Message
	sb.WriteString("\n\n")
	if m.StatusMessage != "" {
		if m.StatusIsError {
			sb.WriteString(styles.StatusError.Render(m.StatusMessage))
		} else {
			sb.WriteString(styles.StatusSuccess.Render(m.StatusMessage))
		}
	}

	return sb.String()
}
