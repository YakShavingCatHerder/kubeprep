package terminal

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type reservedAction string

const (
	actionNone     reservedAction = ""
	actionPrefix   reservedAction = "prefix"
	actionHint     reservedAction = "hint"
	actionCheck    reservedAction = "check"
	actionZoom     reservedAction = "zoom"
	actionQuit     reservedAction = "quit"
	actionContinue reservedAction = "continue"
)

func reservedActionFor(msg tea.KeyMsg, prefix bool, prompting bool) (reservedAction, bool) {
	if prompting {
		switch strings.ToLower(msg.String()) {
		case "y", "enter":
			return actionContinue, true
		case "n":
			return actionQuit, true
		}
	}
	switch strings.ToLower(msg.String()) {
	case "f1", "?", "alt+h":
		return actionHint, true
	case "f2", "alt+c":
		return actionCheck, true
	case "f10", "alt+q":
		return actionQuit, true
	case "f11", "alt+z":
		return actionZoom, true
	case "ctrl+g":
		return actionPrefix, true
	}
	switch msg.Type {
	case tea.KeyF1:
		return actionHint, true
	case tea.KeyF2:
		return actionCheck, true
	case tea.KeyF10:
		return actionQuit, true
	case tea.KeyF11:
		return actionZoom, true
	case tea.KeyCtrlG:
		return actionPrefix, true
	}
	if !prefix {
		return actionNone, false
	}
	switch msg.String() {
	case "h", "?":
		return actionHint, true
	case "c":
		return actionCheck, true
	case "z":
		return actionZoom, true
	case "q":
		return actionQuit, true
	default:
		return actionNone, true
	}
}

func encodeKey(msg tea.KeyMsg) []byte {
	switch msg.Type {
	case tea.KeyRunes:
		return []byte(string(msg.Runes))
	case tea.KeyEnter:
		return []byte{'\r'}
	case tea.KeyTab:
		return []byte{'\t'}
	case tea.KeySpace:
		return []byte{' '}
	case tea.KeyBackspace:
		return []byte{0x7f}
	case tea.KeyDelete:
		return []byte("\x1b[3~")
	case tea.KeyEsc:
		return []byte{0x1b}
	case tea.KeyUp:
		return []byte("\x1b[A")
	case tea.KeyDown:
		return []byte("\x1b[B")
	case tea.KeyRight:
		return []byte("\x1b[C")
	case tea.KeyLeft:
		return []byte("\x1b[D")
	case tea.KeyHome:
		return []byte("\x1b[H")
	case tea.KeyEnd:
		return []byte("\x1b[F")
	case tea.KeyPgUp:
		return []byte("\x1b[5~")
	case tea.KeyPgDown:
		return []byte("\x1b[6~")
	case tea.KeyCtrlC:
		return []byte{0x03}
	case tea.KeyCtrlD:
		return []byte{0x04}
	case tea.KeyCtrlZ:
		return []byte{0x1a}
	case tea.KeyCtrlL:
		return []byte{0x0c}
	case tea.KeyCtrlA:
		return []byte{0x01}
	case tea.KeyCtrlE:
		return []byte{0x05}
	case tea.KeyCtrlK:
		return []byte{0x0b}
	case tea.KeyCtrlU:
		return []byte{0x15}
	case tea.KeyCtrlW:
		return []byte{0x17}
	case tea.KeyCtrlR:
		return []byte{0x12}
	case tea.KeyCtrlP:
		return []byte{0x10}
	case tea.KeyCtrlN:
		return []byte{0x0e}
	default:
		if msg.Type > 0 && msg.Type < 27 {
			return []byte{byte(msg.Type)}
		}
		if s := msg.String(); len(s) == 1 {
			return []byte(s)
		}
		return nil
	}
}
