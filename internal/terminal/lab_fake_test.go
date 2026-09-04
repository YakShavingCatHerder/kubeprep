package terminal

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

type memoryLab struct {
	view  string
	typed []string
	cols  int
	rows  int
}

func (m *memoryLab) Start(_ context.Context, _ ShellSession, cols, rows int) error {
	m.cols, m.rows = cols, rows
	return nil
}

func (m *memoryLab) Resize(cols, rows int) error {
	m.cols, m.rows = cols, rows
	return nil
}

func (m *memoryLab) HandleKey(msg tea.KeyMsg) error {
	if payload := encodeKey(msg); len(payload) > 0 {
		m.typed = append(m.typed, string(payload))
	}
	return nil
}

func (m *memoryLab) View() string {
	if m.view == "" {
		return "$ "
	}
	return m.view
}

func (m *memoryLab) Listen() tea.Cmd { return nil }

func (m *memoryLab) Close() error { return nil }
