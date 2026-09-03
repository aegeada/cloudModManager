package styles

// Palette colors for CMM TUI
const (
	ColorPrimary   Color = "#7D56F4"
	ColorSecondary Color = "#00D787"
	ColorCyan      Color = "#00AFFF"
	ColorGreen     Color = "#00FF87"
	ColorYellow    Color = "#FFD700"
	ColorRed       Color = "#FF5F5F"
	ColorGray      Color = "#767676"
	ColorDarkGray  Color = "#3A3A3A"
	ColorLightGray Color = "#BCBCBC"
	ColorWhite     Color = "#FFFFFF"
	ColorBlack     Color = "#000000"
	ColorBgDark    Color = "#1E1E2E"
	ColorHighlight Color = "#5F5FAF"
)

var (
	// App Shell & Headers
	HeaderTitleStyle = New().Bold(true).Foreground(ColorWhite).Background(ColorPrimary).Padding(0, 1, 0, 1)
	HeaderSubStyle   = New().Foreground(ColorLightGray).Padding(0, 1, 0, 0)
	StatusBarKey     = New().Bold(true).Foreground(ColorCyan)
	StatusBarDesc    = New().Foreground(ColorLightGray)
	StatusSuccess    = New().Bold(true).Foreground(ColorGreen)
	StatusError      = New().Bold(true).Foreground(ColorRed)

	// Tab Bar
	ActiveTabStyle = New().
			Bold(true).
			Foreground(ColorWhite).
			Background(ColorPrimary).
			Padding(0, 2, 0, 2)

	InactiveTabStyle = New().
				Foreground(ColorLightGray).
				Background(ColorDarkGray).
				Padding(0, 2, 0, 2)

	TabDivider = New().Foreground(ColorDarkGray)

	// Status Badges
	BadgePin = New().
			Bold(true).
			Foreground(ColorYellow)

	BadgeUpdate = New().
			Bold(true).
			Foreground(ColorGreen)

	BadgeOk = New().
			Foreground(ColorCyan)

	BadgeServer = New().
			Foreground(ColorPrimary)

	BadgeClient = New().
			Foreground(ColorCyan)

	BadgeBoth = New().
			Foreground(ColorSecondary)

	// Tables & Lists
	TableHeaderStyle = New().
				Bold(true).
				Foreground(ColorWhite).
				Background(ColorDarkGray).
				Padding(0, 1, 0, 1)

	TableRowSelected = New().
				Bold(true).
				Foreground(ColorWhite).
				Background(ColorHighlight).
				Padding(0, 1, 0, 1)

	TableRowNormal = New().
			Foreground(ColorLightGray).
			Padding(0, 1, 0, 1)

	TableRowDim = New().
			Foreground(ColorGray).
			Padding(0, 1, 0, 1)

	// Form Inputs & Fields
	FieldLabelStyle = New().
			Bold(true).
			Foreground(ColorCyan)

	FieldFocusedLabel = New().
				Bold(true).
				Foreground(ColorYellow)

	InputValueStyle = New().
			Foreground(ColorWhite)

	InputPlaceholderStyle = New().
				Foreground(ColorGray)

	ButtonActive = New().
			Bold(true).
			Foreground(ColorBlack).
			Background(ColorSecondary).
			Padding(0, 2, 0, 2)

	ButtonInactive = New().
			Foreground(ColorWhite).
			Background(ColorDarkGray).
			Padding(0, 2, 0, 2)

	// Modals & Overlays
	ModalBoxStyle = New().
			Border(RoundedBorder).
			BorderForeground(ColorPrimary).
			Padding(1, 2, 1, 2)

	ModalTitleStyle = New().
			Bold(true).
			Foreground(ColorWhite).
			Background(ColorPrimary).
			Padding(0, 1, 0, 1)

	HelpKeyStyle = New().
			Bold(true).
			Foreground(ColorYellow)

	HelpDescStyle = New().
			Foreground(ColorLightGray)
)
