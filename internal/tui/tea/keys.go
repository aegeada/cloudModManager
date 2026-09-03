package tea

import (
	"bufio"
	"io"
	"strings"
	"unicode/utf8"
)

// KeyType identifies the kind of key pressed.
type KeyType int

const (
	KeyRunes KeyType = iota
	KeyNull
	KeyBreak
	KeyEnter
	KeyBackspace
	KeyTab
	KeyShiftTab
	KeyEsc
	KeyLeft
	KeyRight
	KeyUp
	KeyDown
	KeyPgUp
	KeyPgDown
	KeyHome
	KeyEnd
	KeyDelete
	KeyInsert
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyCtrlC
	KeyCtrlD
	KeyCtrlZ
	KeyCtrlA
	KeyCtrlE
	KeyCtrlK
	KeyCtrlU
	KeyCtrlW
	KeySpace
)

// KeyMsg represents a key press event.
type KeyMsg struct {
	Type  KeyType
	Runes []rune
	Alt   bool
}

// String returns a canonical string representation of the KeyMsg.
func (k KeyMsg) String() string {
	switch k.Type {
	case KeyEnter:
		return "enter"
	case KeyBackspace:
		return "backspace"
	case KeyTab:
		return "tab"
	case KeyShiftTab:
		return "shift+tab"
	case KeyEsc:
		return "esc"
	case KeyLeft:
		return "left"
	case KeyRight:
		return "right"
	case KeyUp:
		return "up"
	case KeyDown:
		return "down"
	case KeyPgUp:
		return "pgup"
	case KeyPgDown:
		return "pgdown"
	case KeyHome:
		return "home"
	case KeyEnd:
		return "end"
	case KeyDelete:
		return "delete"
	case KeyInsert:
		return "insert"
	case KeyF1:
		return "f1"
	case KeyF2:
		return "f2"
	case KeyF3:
		return "f3"
	case KeyF4:
		return "f4"
	case KeyF5:
		return "f5"
	case KeyF6:
		return "f6"
	case KeyF7:
		return "f7"
	case KeyF8:
		return "f8"
	case KeyF9:
		return "f9"
	case KeyF10:
		return "f10"
	case KeyF11:
		return "f11"
	case KeyF12:
		return "f12"
	case KeyCtrlC:
		return "ctrl+c"
	case KeyCtrlD:
		return "ctrl+d"
	case KeyCtrlZ:
		return "ctrl+z"
	case KeyCtrlA:
		return "ctrl+a"
	case KeyCtrlE:
		return "ctrl+e"
	case KeyCtrlK:
		return "ctrl+k"
	case KeyCtrlU:
		return "ctrl+u"
	case KeyCtrlW:
		return "ctrl+w"
	case KeySpace:
		return " "
	case KeyRunes:
		return string(k.Runes)
	default:
		if len(k.Runes) > 0 {
			return string(k.Runes)
		}
		return ""
	}
}

// KeyMatches checks if a KeyMsg matches any of the given key strings.
func KeyMatches(msg KeyMsg, keys ...string) bool {
	str := msg.String()
	for _, k := range keys {
		if strings.EqualFold(str, k) {
			return true
		}
	}
	return false
}

// readInputs reads from the input reader and dispatches KeyMsg to the event loop.
func (p *Program) readInputs() {
	reader := bufio.NewReader(p.input)
	buf := make([]byte, 256)

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		n, err := reader.Read(buf)
		if n > 0 {
			p.parseAndSendKeys(buf[:n])
		}
		if err != nil {
			if err == io.EOF {
				// Clean EOF - if not already quitting, quit
				p.Send(QuitMsg{})
			}
			return
		}
	}
}

func (p *Program) parseAndSendKeys(b []byte) {
	i := 0
	for i < len(b) {
		// ANSI Escape sequences
		if b[i] == 0x1b { // ESC
			if i+1 >= len(b) {
				p.Send(KeyMsg{Type: KeyEsc})
				i++
				continue
			}

			if b[i+1] == '[' || b[i+1] == 'O' {
				// CSI sequence
				seq, consumed := parseCSISequence(b[i:])
				if consumed > 0 {
					p.Send(seq)
					i += consumed
					continue
				}
			}

			// Alt + key
			r, size := utf8.DecodeRune(b[i+1:])
			p.Send(KeyMsg{Type: KeyRunes, Runes: []rune{r}, Alt: true})
			i += 1 + size
			continue
		}

		// Control characters
		switch b[i] {
		case 0x00: // Ctrl+@
			p.Send(KeyMsg{Type: KeyNull})
		case 0x01: // Ctrl+A
			p.Send(KeyMsg{Type: KeyCtrlA})
		case 0x03: // Ctrl+C
			p.Send(KeyMsg{Type: KeyCtrlC})
		case 0x04: // Ctrl+D
			p.Send(KeyMsg{Type: KeyCtrlD})
		case 0x05: // Ctrl+E
			p.Send(KeyMsg{Type: KeyCtrlE})
		case 0x09: // Tab
			p.Send(KeyMsg{Type: KeyTab})
		case 0x0a, 0x0d: // Enter (\n or \r)
			p.Send(KeyMsg{Type: KeyEnter})
		case 0x0b: // Ctrl+K
			p.Send(KeyMsg{Type: KeyCtrlK})
		case 0x15: // Ctrl+U
			p.Send(KeyMsg{Type: KeyCtrlU})
		case 0x17: // Ctrl+W
			p.Send(KeyMsg{Type: KeyCtrlW})
		case 0x1a: // Ctrl+Z
			p.Send(KeyMsg{Type: KeyCtrlZ})
		case 0x20: // Space
			p.Send(KeyMsg{Type: KeySpace, Runes: []rune{' '}})
		case 0x7f, 0x08: // Backspace
			p.Send(KeyMsg{Type: KeyBackspace})
		default:
			// UTF-8 Rune
			r, size := utf8.DecodeRune(b[i:])
			if r != utf8.RuneError {
				p.Send(KeyMsg{Type: KeyRunes, Runes: []rune{r}})
				i += size
				continue
			}
			p.Send(KeyMsg{Type: KeyRunes, Runes: []rune{rune(b[i])}})
		}
		i++
	}
}

func parseCSISequence(b []byte) (KeyMsg, int) {
	if len(b) < 2 {
		return KeyMsg{Type: KeyEsc}, 1
	}

	if b[1] == 'O' {
		// SS3 sequence e.g. \x1bOP -> F1
		if len(b) >= 3 {
			switch b[2] {
			case 'P':
				return KeyMsg{Type: KeyF1}, 3
			case 'Q':
				return KeyMsg{Type: KeyF2}, 3
			case 'R':
				return KeyMsg{Type: KeyF3}, 3
			case 'S':
				return KeyMsg{Type: KeyF4}, 3
			}
		}
	}

	if b[1] == '[' {
		if len(b) >= 3 {
			switch b[2] {
			case 'A':
				return KeyMsg{Type: KeyUp}, 3
			case 'B':
				return KeyMsg{Type: KeyDown}, 3
			case 'C':
				return KeyMsg{Type: KeyRight}, 3
			case 'D':
				return KeyMsg{Type: KeyLeft}, 3
			case 'H':
				return KeyMsg{Type: KeyHome}, 3
			case 'F':
				return KeyMsg{Type: KeyEnd}, 3
			case 'Z':
				return KeyMsg{Type: KeyShiftTab}, 3
			}

			// Tilde sequences e.g. \x1b[3~ -> Delete, \x1b[5~ -> PgUp, \x1b[6~ -> PgDown
			if len(b) >= 4 && b[3] == '~' {
				switch b[2] {
				case '1', '7':
					return KeyMsg{Type: KeyHome}, 4
				case '2':
					return KeyMsg{Type: KeyInsert}, 4
				case '3':
					return KeyMsg{Type: KeyDelete}, 4
				case '4', '8':
					return KeyMsg{Type: KeyEnd}, 4
				case '5':
					return KeyMsg{Type: KeyPgUp}, 4
				case '6':
					return KeyMsg{Type: KeyPgDown}, 4
				}
			}

			// F1-F12 extended sequences e.g. \x1b[11~ -> F1, \x1b[12~ -> F2 ...
			if len(b) >= 5 && b[4] == '~' {
				code := string(b[2:4])
				switch code {
				case "11":
					return KeyMsg{Type: KeyF1}, 5
				case "12":
					return KeyMsg{Type: KeyF2}, 5
				case "13":
					return KeyMsg{Type: KeyF3}, 5
				case "14":
					return KeyMsg{Type: KeyF4}, 5
				case "15":
					return KeyMsg{Type: KeyF5}, 5
				case "17":
					return KeyMsg{Type: KeyF6}, 5
				case "18":
					return KeyMsg{Type: KeyF7}, 5
				case "19":
					return KeyMsg{Type: KeyF8}, 5
				case "20":
					return KeyMsg{Type: KeyF9}, 5
				case "21":
					return KeyMsg{Type: KeyF10}, 5
				case "23":
					return KeyMsg{Type: KeyF11}, 5
				case "24":
					return KeyMsg{Type: KeyF12}, 5
				}
			}
		}
	}

	return KeyMsg{Type: KeyEsc}, 1
}
