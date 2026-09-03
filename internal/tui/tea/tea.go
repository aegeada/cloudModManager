package tea

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Msg represents an action or event in the Elm architecture.
type Msg interface{}

// Cmd is an IO operation that produces a Msg when completed.
type Cmd func() Msg

// Model is the application state container.
type Model interface {
	Init() Cmd
	Update(msg Msg) (Model, Cmd)
	View() string
}

// WindowSizeMsg is sent when the terminal window changes dimensions.
type WindowSizeMsg struct {
	Width  int
	Height int
}

// QuitMsg requests the program to terminate.
type QuitMsg struct{}

// Quit is a command that terminates the Bubbletea program loop.
func Quit() Msg {
	return QuitMsg{}
}

// Batch executes multiple commands in parallel.
func Batch(cmds ...Cmd) Cmd {
	var validCmds []Cmd
	for _, c := range cmds {
		if c != nil {
			validCmds = append(validCmds, c)
		}
	}
	if len(validCmds) == 0 {
		return nil
	}
	return func() Msg {
		return batchMsg(validCmds)
	}
}

type batchMsg []Cmd

// Sequence runs commands sequentially.
func Sequence(cmds ...Cmd) Cmd {
	var validCmds []Cmd
	for _, c := range cmds {
		if c != nil {
			validCmds = append(validCmds, c)
		}
	}
	if len(validCmds) == 0 {
		return nil
	}
	return func() Msg {
		return sequenceMsg(validCmds)
	}
}

type sequenceMsg []Cmd

// Tick runs a timer that sends a message after duration d.
func Tick(d time.Duration, fn func(t time.Time) Msg) Cmd {
	return func() Msg {
		time.Sleep(d)
		if fn != nil {
			return fn(time.Now())
		}
		return nil
	}
}

// ProgramOption configures the Program.
type ProgramOption func(*Program)

// WithInput configures the input reader.
func WithInput(r io.Reader) ProgramOption {
	return func(p *Program) {
		p.input = r
	}
}

// WithOutput configures the output writer.
func WithOutput(w io.Writer) ProgramOption {
	return func(p *Program) {
		p.output = w
	}
}

// WithoutRenderer disables terminal rendering (used in headless testing).
func WithoutRenderer() ProgramOption {
	return func(p *Program) {
		p.headless = true
	}
}

// WithAltScreen enables alternate screen buffer.
func WithAltScreen() ProgramOption {
	return func(p *Program) {
		p.altScreen = true
	}
}

// Program runs an Elm architecture Model event loop.
type Program struct {
	initialModel Model
	model        Model
	input        io.Reader
	output       io.Writer
	msgs         chan Msg
	errs         chan error
	ctx          context.Context
	cancel       context.CancelFunc
	headless     bool
	altScreen    bool
	mu           sync.Mutex
	running      bool
	width        int
	height       int
}

// NewProgram constructs a new Program.
func NewProgram(m Model, opts ...ProgramOption) *Program {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Program{
		initialModel: m,
		model:        m,
		input:        os.Stdin,
		output:       os.Stdout,
		msgs:         make(chan Msg, 128),
		errs:         make(chan error, 1),
		ctx:          ctx,
		cancel:       cancel,
		width:        80,
		height:       24,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Send injects a message into the program's event loop.
func (p *Program) Send(msg Msg) {
	if msg == nil {
		return
	}
	select {
	case <-p.ctx.Done():
	case p.msgs <- msg:
	}
}

// Run executes the application event loop.
func (p *Program) Run() (Model, error) {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return p.model, fmt.Errorf("program is already running")
	}
	p.running = true
	p.mu.Unlock()

	defer func() {
		p.cancel()
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()

	// Query initial terminal dimensions if not headless
	if !p.headless {
		if w, h, err := GetTerminalSize(); err == nil && w > 0 && h > 0 {
			p.width = w
			p.height = h
		}
	}

	// Initialize terminal screen
	if !p.headless && p.altScreen {
		fmt.Fprint(p.output, "\033[?1049h\033[H")
		defer fmt.Fprint(p.output, "\033[?1049l\033[?25h")
	}

	// Initialize model
	initCmd := p.initialModel.Init()
	if initCmd != nil {
		go p.handleCmd(initCmd)
	}

	// Send initial WindowSizeMsg
	p.msgs <- WindowSizeMsg{Width: p.width, Height: p.height}

	// Start input reader goroutine
	go p.readInputs()

	// Initial render
	if !p.headless {
		p.render(p.model.View())
	}

	// Main event loop
	for {
		select {
		case <-p.ctx.Done():
			return p.model, nil
		case msg := <-p.msgs:
			if msg == nil {
				continue
			}

			switch m := msg.(type) {
			case QuitMsg:
				return p.model, nil
			case batchMsg:
				for _, cmd := range m {
					if cmd != nil {
						go p.handleCmd(cmd)
					}
				}
				continue
			case sequenceMsg:
				go func(cmds []Cmd) {
					for _, cmd := range cmds {
						if cmd != nil {
							res := cmd()
							if res != nil {
								p.Send(res)
							}
						}
					}
				}(m)
				continue
			case WindowSizeMsg:
				p.width = m.Width
				p.height = m.Height
			}

			newModel, cmd := p.model.Update(msg)
			p.model = newModel

			if cmd != nil {
				go p.handleCmd(cmd)
			}

			if !p.headless {
				p.render(p.model.View())
			}
		}
	}
}

func (p *Program) handleCmd(cmd Cmd) {
	if cmd == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			// Prevent crashes in background commands
		}
	}()
	msg := cmd()
	if msg != nil {
		p.Send(msg)
	}
}

func (p *Program) render(view string) {
	if p.headless {
		return
	}
	// Clear screen to home and write view
	var sb strings.Builder
	sb.WriteString("\033[H\033[2J")
	sb.WriteString(view)
	fmt.Fprint(p.output, sb.String())
}

// Release resources
func (p *Program) Kill() {
	p.cancel()
}
