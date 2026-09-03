package components

import (
	"time"

	"cmm/internal/tui/styles"
	"cmm/internal/tui/tea"
)

// SpinnerTickMsg is dispatched to advance the spinner animation.
type SpinnerTickMsg struct {
	Time time.Time
	ID   int
}

// Spinner provides animated progress indicator.
type Spinner struct {
	frames []string
	frame  int
	id     int
}

// NewSpinner creates a new Braille spinner.
func NewSpinner() Spinner {
	return Spinner{
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		frame:  0,
	}
}

// Tick returns a command to advance spinner frame after 80ms.
func (s Spinner) Tick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return SpinnerTickMsg{Time: t, ID: s.id}
	})
}

// Update advances the frame when receiving SpinnerTickMsg.
func (s Spinner) Update(msg tea.Msg) (Spinner, tea.Cmd) {
	if _, ok := msg.(SpinnerTickMsg); ok {
		s.frame = (s.frame + 1) % len(s.frames)
		return s, s.Tick()
	}
	return s, nil
}

// View renders current spinner frame.
func (s Spinner) View() string {
	if len(s.frames) == 0 {
		return ""
	}
	frame := s.frames[s.frame]
	return styles.New().Bold(true).Foreground(styles.ColorCyan).Render(frame)
}
