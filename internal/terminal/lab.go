package terminal

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

// LabShell is a real PTY session rendered inside the scenario view.
type LabShell interface {
	Start(ctx context.Context, session ShellSession, cols, rows int) error
	Resize(cols, rows int) error
	HandleKey(msg tea.KeyMsg) error
	View() string
	Listen() tea.Cmd
	Close() error
}

type labOutputMsg struct{}

type labExitMsg struct {
	err error
}

type labErrorMsg struct {
	err error
}

type labReadyMsg struct{}

type observeTickMsg struct{}

type ptyLab struct {
	runner *ShellRunner

	mu      sync.Mutex
	ctx     context.Context
	session ShellSession
	command *exec.Cmd
	file    *os.File
	emu     vt10x.Terminal
	events  chan tea.Msg
	cols    int
	rows    int
	closed  bool
	once    sync.Once
}

func newPtyLab(runner *ShellRunner) *ptyLab {
	if runner == nil {
		runner = NewShellRunner()
	}
	return &ptyLab{
		runner: runner,
		events: make(chan tea.Msg, 16),
		cols:   80,
		rows:   24,
	}
}

func (p *ptyLab) Start(ctx context.Context, session ShellSession, cols, rows int) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return io.ErrClosedPipe
	}
	p.ctx = ctx
	p.session = session
	if cols > 0 {
		p.cols = cols
	}
	if rows > 0 {
		p.rows = rows
	}
	p.mu.Unlock()
	return p.spawn()
}

func (p *ptyLab) spawn() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return io.ErrClosedPipe
	}
	if p.file != nil {
		return nil
	}

	command, err := p.runner.Command(p.ctx, p.session)
	if err != nil {
		return err
	}
	size := &pty.Winsize{Cols: uint16(p.cols), Rows: uint16(p.rows)}
	file, err := pty.StartWithSize(command, size)
	if err != nil {
		return fmt.Errorf("start lab shell: %w", err)
	}
	p.command = command
	p.file = file
	p.emu = vt10x.New(vt10x.WithSize(p.cols, p.rows))

	go p.readLoop(file)
	go p.waitLoop(command)
	return nil
}

func (p *ptyLab) readLoop(file *os.File) {
	buf := make([]byte, 8192)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			p.mu.Lock()
			if p.emu != nil {
				_, _ = p.emu.Write(buf[:n])
			}
			p.mu.Unlock()
			p.send(labOutputMsg{})
		}
		if err != nil {
			return
		}
	}
}

func (p *ptyLab) waitLoop(command *exec.Cmd) {
	err := command.Wait()
	p.mu.Lock()
	if p.file != nil {
		_ = p.file.Close()
		p.file = nil
	}
	p.command = nil
	p.emu = nil
	closed := p.closed
	session := p.session
	ctx := p.ctx
	cols, rows := p.cols, p.rows
	p.mu.Unlock()
	if closed {
		p.send(labExitMsg{err: err})
		return
	}
	if restartErr := p.Start(ctx, session, cols, rows); restartErr != nil {
		p.send(labExitMsg{err: restartErr})
		return
	}
	p.send(labOutputMsg{})
}

func (p *ptyLab) send(msg tea.Msg) {
	select {
	case p.events <- msg:
	default:
	}
}

func (p *ptyLab) Resize(cols, rows int) error {
	if cols < 1 || rows < 1 {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cols = cols
	p.rows = rows
	if p.emu != nil {
		p.emu.Resize(cols, rows)
	}
	if p.file != nil {
		if err := pty.Setsize(p.file, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)}); err != nil {
			return fmt.Errorf("resize lab shell: %w", err)
		}
	}
	return nil
}

func (p *ptyLab) HandleKey(msg tea.KeyMsg) error {
	payload := encodeKey(msg)
	if len(payload) == 0 {
		return nil
	}
	p.mu.Lock()
	file := p.file
	p.mu.Unlock()
	if file == nil {
		return nil
	}
	_, err := file.Write(payload)
	return err
}

func (p *ptyLab) View() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.emu == nil {
		return "Starting Lab Shell…"
	}
	return renderEmulator(p.emu, p.cols, p.rows)
}

func (p *ptyLab) Listen() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-p.events
		if !ok {
			return labExitMsg{err: io.EOF}
		}
		return msg
	}
}

func (p *ptyLab) Close() error {
	var err error
	p.once.Do(func() {
		p.mu.Lock()
		p.closed = true
		file := p.file
		command := p.command
		p.file = nil
		p.command = nil
		p.mu.Unlock()
		if file != nil {
			_ = file.Close()
		}
		if command != nil && command.Process != nil {
			err = command.Process.Kill()
		}
	})
	return err
}

func renderEmulator(term vt10x.Terminal, cols, rows int) string {
	if term == nil || cols < 1 || rows < 1 {
		return ""
	}
	raw := term.String()
	lines := strings.Split(raw, "\n")
	if len(lines) > rows {
		lines = lines[:rows]
	}
	for i, line := range lines {
		if lipgloss.Width(line) > cols {
			lines[i] = trimToWidth(line, cols)
		}
	}
	for len(lines) < rows {
		lines = append(lines, "")
	}
	if term.CursorVisible() {
		cursor := term.Cursor()
		if cursor.Y >= 0 && cursor.Y < len(lines) {
			lines[cursor.Y] = overlayCursor(lines[cursor.Y], cursor.X, cols)
		}
	}
	return strings.Join(lines, "\n")
}

func trimToWidth(line string, cols int) string {
	var width int
	var b strings.Builder
	for _, r := range line {
		w := 1
		if r == '\t' {
			w = 4
		}
		if width+w > cols {
			break
		}
		b.WriteRune(r)
		width += w
	}
	return b.String()
}

func overlayCursor(line string, x, cols int) string {
	runes := []rune(line)
	for len(runes) < cols {
		runes = append(runes, ' ')
	}
	if x < 0 || x >= len(runes) {
		return line
	}
	cell := string(runes[x])
	if cell == "" {
		cell = " "
	}
	styled := lipgloss.NewStyle().Reverse(true).Render(cell)
	return string(runes[:x]) + styled + string(runes[x+1:])
}
