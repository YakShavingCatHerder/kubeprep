package terminal

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestComputeSplitLayoutUsesSideBySideWhenWide(t *testing.T) {
	layout := computeSplitLayout(120, 40, false, defaultFooterLines)
	if !layout.SideBySide {
		t.Fatal("expected a side-by-side layout at 120 columns")
	}
	if layout.Scenario.Width+layout.Shell.Width != 120 {
		t.Fatalf("pane widths = %d + %d, want 120", layout.Scenario.Width, layout.Shell.Width)
	}
	if layout.Shell.Inner().Width < 40 || layout.Shell.Inner().Height < 10 {
		t.Fatalf("shell inner pane too small: %+v", layout.Shell.Inner())
	}
}

func TestComputeSplitLayoutStacksOnEightyColumns(t *testing.T) {
	layout := computeSplitLayout(80, 24, false, defaultFooterLines)
	if layout.SideBySide {
		t.Fatal("80 columns should stack the scenario above the shell")
	}
	if layout.Scenario.Height+layout.Shell.Height != 24-headerLines-defaultFooterLines {
		t.Fatalf("stacked body heights = %d + %d", layout.Scenario.Height, layout.Shell.Height)
	}
}

func TestComputeSplitLayoutZoomGivesShellTheBody(t *testing.T) {
	layout := computeSplitLayout(120, 40, true, defaultFooterLines)
	if layout.Scenario.Width != 0 || layout.Scenario.Height != 0 {
		t.Fatalf("zoomed layout still has a scenario pane: %+v", layout.Scenario)
	}
	if layout.Shell.Width != 120 {
		t.Fatalf("zoomed shell width = %d, want 120", layout.Shell.Width)
	}
}

func TestReservedKeysAreNotForwardedToTheShell(t *testing.T) {
	tests := []struct {
		msg    tea.KeyMsg
		prefix bool
		action reservedAction
	}{
		{msg: tea.KeyMsg{Type: tea.KeyF1}, action: actionHint},
		{msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}, action: actionHint},
		{msg: tea.KeyMsg{Type: tea.KeyF2}, action: actionCheck},
		{msg: tea.KeyMsg{Type: tea.KeyF10}, action: actionQuit},
		{msg: tea.KeyMsg{Type: tea.KeyF11}, action: actionZoom},
		{msg: tea.KeyMsg{Type: tea.KeyCtrlG}, action: actionPrefix},
		{msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}, prefix: true, action: actionHint},
		{msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}, prefix: true, action: actionCheck},
	}
	for _, tt := range tests {
		action, reserved := reservedActionFor(tt.msg, tt.prefix, false)
		if !reserved || action != tt.action {
			t.Fatalf("key %v prefix %v: action=%q reserved=%v", tt.msg, tt.prefix, action, reserved)
		}
	}
	action, reserved := reservedActionFor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}, false, true)
	if !reserved || action != actionContinue {
		t.Fatal("y should continue when the next-scenario prompt is active")
	}
	action, reserved = reservedActionFor(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}, false, false)
	if reserved {
		t.Fatal("y should be typed into the lab shell before completion")
	}
}

func TestEncodeKeySendsEnterAndCtrlCToThePTY(t *testing.T) {
	if got := encodeKey(tea.KeyMsg{Type: tea.KeyEnter}); string(got) != "\r" {
		t.Fatalf("enter = %q", got)
	}
	if got := encodeKey(tea.KeyMsg{Type: tea.KeyCtrlC}); string(got) != "\x03" {
		t.Fatalf("ctrl-c = %q", got)
	}
	if got := encodeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("kubectl")}); string(got) != "kubectl" {
		t.Fatalf("runes = %q", got)
	}
}

func TestEncodeKeyIgnoresFunctionKeys(t *testing.T) {
	if got := encodeKey(tea.KeyMsg{Type: tea.KeyF2}); len(got) != 0 {
		t.Fatalf("F2 should not be typed into the PTY, got %q", got)
	}
}

func TestMemoryLabRecordsForwardedKeys(t *testing.T) {
	lab := &memoryLab{view: "$ "}
	model := scenarioViewModel{lab: lab}
	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if cmd != nil {
		t.Fatal("forwarded keys should not start a TUI command")
	}
	got := next.(scenarioViewModel)
	if strings.Join(lab.typed, "") != "k" {
		t.Fatalf("lab received %q", lab.typed)
	}
	if got.hintLevel != 0 {
		t.Fatal("letter k should not activate a hint")
	}
}
