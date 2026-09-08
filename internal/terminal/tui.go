package terminal

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type CheckState string

const (
	CheckWrong      CheckState = "wrong"
	CheckConverging CheckState = "converging"
	CheckSuccess    CheckState = "success"

	observeInterval = 5 * time.Second
)

// ScenarioView keeps presentation callbacks independent from Kubernetes and
// curriculum implementations.
type ScenarioView struct {
	ScenarioID          string
	Title               string
	Module              string
	Description         string
	Objective           string
	Namespace           string
	Resource            string
	Experience          string
	Hints               []string
	InitialHintLevel    int
	Completion          string
	Debrief             string
	Kubeconfig          string
	ToolBinDir          string
	ObserveWhileRunning bool
	ObserveDelay        time.Duration
	HasNext             bool
	NextTitle           string

	Check    func(context.Context) (CheckState, string, error)
	UseHint  func(level int) error
	Complete func() error
	Shell    *ShellRunner
	Lab      LabShell
}

type SessionResult struct {
	Continue bool
}

func RunScenarioView(ctx context.Context, scenario ScenarioView) (SessionResult, error) {
	if scenario.Shell == nil {
		scenario.Shell = NewShellRunner()
	}
	if scenario.Lab == nil {
		scenario.Lab = newPtyLab(scenario.Shell)
	}
	defer func() { _ = scenario.Lab.Close() }()

	hintLevel := scenario.InitialHintLevel
	if hintLevel < 0 || hintLevel > len(scenario.Hints) {
		hintLevel = 0
	}
	status := "Lab Shell is attached. Press F2 to validate cluster state."
	if scenario.ObserveWhileRunning && scenario.ObserveDelay > 0 {
		status = waitingStatus(scenario.ObserveDelay)
	}
	model := scenarioViewModel{
		ctx:        ctx,
		scenario:   scenario,
		lab:        scenario.Lab,
		hintLevel:  hintLevel,
		status:     status,
		storyBeats: splitStoryBeats(scenario.Description),
	}
	program := tea.NewProgram(model, tea.WithAltScreen())
	final, err := program.Run()
	if err != nil {
		return SessionResult{}, fmt.Errorf("run scenario view: %w", err)
	}
	if finished, ok := final.(scenarioViewModel); ok {
		return SessionResult{Continue: finished.continueNext}, nil
	}
	return SessionResult{}, nil
}

type checkResultMsg struct {
	state   CheckState
	message string
	err     error
}

type hintSavedMsg struct{ err error }
type completionSavedMsg struct{ err error }

type scenarioViewModel struct {
	ctx             context.Context
	scenario        ScenarioView
	lab             LabShell
	width           int
	height          int
	status          string
	diagnostic      string
	state           CheckState
	hintLevel       int
	storyBeats      []string
	checking        bool
	shellError      error
	zoomed          bool
	prefix          bool
	now             func() time.Time
	observeDeadline time.Time
	advancePrompt   bool
	continueNext    bool
}

func (m scenarioViewModel) Init() tea.Cmd {
	return m.startLabCmd()
}

func (m scenarioViewModel) startLabCmd() tea.Cmd {
	if m.lab == nil {
		return m.observeCmd()
	}
	session := ShellSession{
		Kubeconfig: m.scenario.Kubeconfig,
		Namespace:  m.scenario.Namespace,
		ScenarioID: m.scenario.ScenarioID,
		Objective:  m.scenario.Objective,
		ToolBinDir: m.scenario.ToolBinDir,
	}
	return func() tea.Msg {
		layout := computeSplitLayout(m.width, m.height, m.zoomed, m.footerHeight())
		inner := layout.Shell.Inner()
		if err := m.lab.Start(m.ctx, session, inner.Width, inner.Height); err != nil {
			return labErrorMsg{err: err}
		}
		return labReadyMsg{}
	}
}

func (m scenarioViewModel) observeCmd() tea.Cmd {
	if !m.scenario.ObserveWhileRunning {
		return nil
	}
	return tea.Tick(observeInterval, func(time.Time) tea.Msg {
		return observeTickMsg{}
	})
}

func (m scenarioViewModel) waitCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return observeWaitMsg{}
	})
}

func (m scenarioViewModel) beginObservation() (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{m.observeCmd()}
	if !m.checking && m.state != CheckSuccess {
		m.checking = true
		cmds = append(cmds, m.checkCmd())
	}
	return m, tea.Batch(cmds...)
}

func (m scenarioViewModel) clock() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

func waitingStatus(delay time.Duration) string {
	seconds := int(delay.Round(time.Second).Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return fmt.Sprintf("Automatic validation starts in %ds. Press F2 to check now.", seconds)
}

type observeWaitMsg struct{}

func (m scenarioViewModel) listenLab() tea.Cmd {
	if m.lab == nil {
		return nil
	}
	return m.lab.Listen()
}

func (m scenarioViewModel) applyLayout() {
	if m.lab == nil {
		return
	}
	inner := computeSplitLayout(m.width, m.height, m.zoomed, m.footerHeight()).Shell.Inner()
	_ = m.lab.Resize(inner.Width, inner.Height)
}

func (m scenarioViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.applyLayout()
	case tea.KeyMsg:
		action, reserved := reservedActionFor(msg, m.prefix, m.advancePrompt)
		if reserved {
			m.prefix = false
			switch action {
			case actionPrefix:
				m.prefix = true
			case actionHint:
				return m.useHint()
			case actionCheck:
				return m.requestCheck()
			case actionZoom:
				m.zoomed = !m.zoomed
				m.applyLayout()
			case actionContinue:
				if m.advancePrompt && m.scenario.HasNext {
					m.continueNext = true
					return m, tea.Quit
				}
			case actionQuit:
				return m, tea.Quit
			}
			return m, nil
		}
		if m.lab != nil {
			if err := m.lab.HandleKey(msg); err != nil {
				m.shellError = err
			}
		}
		return m, nil
	case labReadyMsg:
		cmds := []tea.Cmd{m.listenLab()}
		if m.scenario.ObserveWhileRunning && m.scenario.ObserveDelay > 0 {
			m.observeDeadline = m.clock().Add(m.scenario.ObserveDelay)
			m.status = waitingStatus(m.scenario.ObserveDelay)
			cmds = append(cmds, m.waitCmd())
		} else {
			cmds = append(cmds, m.observeCmd())
		}
		return m, tea.Batch(cmds...)
	case labOutputMsg:
		return m, m.listenLab()
	case labExitMsg:
		m.shellError = msg.err
		return m, m.listenLab()
	case labErrorMsg:
		m.shellError = msg.err
		return m, m.listenLab()
	case observeWaitMsg:
		if m.state == CheckSuccess {
			return m, nil
		}
		remaining := m.observeDeadline.Sub(m.clock())
		if remaining > 0 {
			m.status = waitingStatus(remaining)
			return m, m.waitCmd()
		}
		return m.beginObservation()
	case observeTickMsg:
		cmds := []tea.Cmd{m.observeCmd()}
		if !m.checking && m.state != CheckSuccess {
			m.checking = true
			cmds = append(cmds, m.checkCmd())
		}
		return m, tea.Batch(cmds...)
	case checkResultMsg:
		m.checking = false
		if msg.err != nil {
			m.status = msg.err.Error()
			m.diagnostic = ""
			m.state = CheckWrong
			return m, nil
		}
		m.state = msg.state
		m.diagnostic = msg.message
		switch msg.state {
		case CheckSuccess:
			m.status = "Target state verified."
		case CheckConverging:
			m.status = "Kubernetes is still reconciling."
		case CheckWrong:
			m.status = "The target state has not been reached."
		}
		if msg.state == CheckSuccess && m.scenario.Complete != nil {
			return m, func() tea.Msg { return completionSavedMsg{err: m.scenario.Complete()} }
		}
		if msg.state == CheckSuccess {
			return m.enableAdvancePrompt(), nil
		}
	case hintSavedMsg:
		if msg.err != nil {
			m.status = "Could not save hint use: " + msg.err.Error()
		}
	case completionSavedMsg:
		if msg.err != nil {
			m.status = "Solved, but progress could not be saved: " + msg.err.Error()
			return m, nil
		}
		return m.enableAdvancePrompt(), nil
	}
	return m, nil
}

func (m scenarioViewModel) useHint() (tea.Model, tea.Cmd) {
	if m.hintLevel >= len(m.scenario.Hints) {
		return m, nil
	}
	m.hintLevel++
	if m.scenario.UseHint == nil {
		return m, nil
	}
	level := m.hintLevel
	return m, func() tea.Msg { return hintSavedMsg{err: m.scenario.UseHint(level)} }
}

func (m scenarioViewModel) enableAdvancePrompt() scenarioViewModel {
	m.advancePrompt = true
	if m.scenario.HasNext {
		title := m.scenario.NextTitle
		if title == "" {
			title = "the next scenario"
		}
		m.status = fmt.Sprintf("Continue to %s?", title)
	} else {
		m.status = "No further scenarios on this track. Press F10 when you are done."
	}
	return m
}

func (m scenarioViewModel) requestCheck() (tea.Model, tea.Cmd) {
	if m.checking {
		return m, nil
	}
	m.checking = true
	return m, m.checkCmd()
}

func (m scenarioViewModel) checkCmd() tea.Cmd {
	return func() tea.Msg {
		if m.scenario.Check == nil {
			return checkResultMsg{state: CheckWrong, message: "Validator is unavailable."}
		}
		state, message, err := m.scenario.Check(m.ctx)
		return checkResultMsg{state: state, message: message, err: err}
	}
}

func (m scenarioViewModel) currentHint() string {
	if m.hintLevel <= 0 || m.hintLevel > len(m.scenario.Hints) {
		return ""
	}
	return strings.TrimSpace(m.scenario.Hints[m.hintLevel-1])
}

func (m scenarioViewModel) footerHeight() int {
	if m.advancePrompt || m.currentHint() != "" {
		return hintFooterLines
	}
	return defaultFooterLines
}

func (m scenarioViewModel) View() string {
	width, height := m.width, m.height
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}
	layout := computeSplitLayout(width, height, m.zoomed, m.footerHeight())

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	label := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))

	header := title.Render("KubePrep · Kubernetes Scenario Runner") + "\n" +
		muted.Render(fmt.Sprintf("SCENARIO %s · MODULE %s · TRACK %s", m.scenario.ScenarioID, m.scenario.Module, m.scenario.Experience))
	header = lipgloss.NewStyle().Width(layout.Header.Width).MaxHeight(layout.Header.Height).Render(header)

	keys := "[?] hint  [F2] check  [F11] zoom  [F10] quit"
	if m.advancePrompt && m.scenario.HasNext {
		keys = "[y] next scenario  [n] stay done  [F10] quit"
	} else if m.advancePrompt {
		keys = "[F10] quit"
	} else if m.prefix {
		keys = "Prefix: [h] hint  [c] check  [z] zoom  [q] quit"
	}
	footerText := muted.Render(keys)
	if hint := m.currentHint(); hint != "" {
		footerText = label.Render(fmt.Sprintf("HINT %d", m.hintLevel)) + "  " + hint + "\n" + muted.Render(keys)
	}
	footer := lipgloss.NewStyle().Width(layout.Footer.Width).MaxHeight(layout.Footer.Height).Render(footerText)

	shell := m.shellPane(layout.Shell)
	if m.zoomed {
		return lipgloss.JoinVertical(lipgloss.Left, header, shell, footer)
	}
	scenario := m.scenarioPane(layout.Scenario)
	if layout.SideBySide {
		return lipgloss.JoinVertical(lipgloss.Left, header, lipgloss.JoinHorizontal(lipgloss.Top, scenario, shell), footer)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, scenario, shell, footer)
}

func (m scenarioViewModel) scenarioPane(size pane) string {
	label := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	inner := size.Inner()

	var body strings.Builder
	body.WriteString(label.Render(m.scenario.Title) + "\n")
	if m.state == CheckSuccess {
		body.WriteString(label.Render("SCENARIO COMPLETED") + "\n\n")
		if text := strings.TrimSpace(m.scenario.Completion); text != "" {
			body.WriteString(text + "\n\n")
		}
		if text := strings.TrimSpace(m.scenario.Debrief); text != "" {
			body.WriteString(label.Render("EXPLANATION") + "\n")
			body.WriteString(text + "\n")
		}
		if m.advancePrompt && m.scenario.HasNext {
			title := m.scenario.NextTitle
			if title == "" {
				title = "the next scenario"
			}
			body.WriteString("\n" + label.Render("CONTINUE") + "\n")
			body.WriteString("Continue to " + title + "?\nPress y to continue, or F10 when you are done training.\n")
		} else if m.advancePrompt {
			body.WriteString("\n" + label.Render("TRAINING COMPLETE") + "\n")
			body.WriteString("No further scenarios remain on this track. Press F10 to leave.\n")
		}
	} else {
		if desc := strings.Join(m.storyBeats, "\n\n"); desc != "" {
			body.WriteString(desc + "\n\n")
		}
		body.WriteString(label.Render("OBJECTIVE") + "\n")
		body.WriteString(m.scenario.Objective + "\n\n")
		body.WriteString(label.Render("VALIDATION") + "\n")
		if m.scenario.Resource != "" {
			body.WriteString(m.scenario.Resource + "\n")
		}
		if m.scenario.Namespace != "" {
			body.WriteString(m.scenario.Namespace + "\n")
		}
		body.WriteString(m.status)
		if m.diagnostic != "" {
			body.WriteString("\n" + muted.Render(m.diagnostic))
		}
		if m.checking {
			body.WriteString("\n" + muted.Render("Checking cluster state…"))
		}
		if m.hintLevel > 0 && m.hintLevel <= len(m.scenario.Hints) {
			body.WriteString("\n\n" + label.Render(fmt.Sprintf("HINT %d", m.hintLevel)) + "\n")
			body.WriteString(m.scenario.Hints[m.hintLevel-1])
		}
	}
	if m.shellError != nil {
		body.WriteString("\n\n" + muted.Render(m.shellError.Error()))
	}

	if inner.Width < 1 {
		inner.Width = 1
	}
	if inner.Height < 1 {
		inner.Height = 1
	}
	content := lipgloss.NewStyle().Width(inner.Width).Height(inner.Height).MaxWidth(inner.Width).MaxHeight(inner.Height).Render(body.String())
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Width(inner.Width).Height(inner.Height).Render(content)
}

func (m scenarioViewModel) shellPane(size pane) string {
	label := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110"))
	inner := size.Inner()
	if inner.Width < 1 {
		inner.Width = 1
	}
	if inner.Height < 1 {
		inner.Height = 1
	}
	title := "LAB SHELL"
	if m.zoomed {
		title = "LAB SHELL · zoomed"
	}
	body := "Starting Lab Shell…"
	if m.lab != nil {
		body = m.lab.View()
	}
	bodyHeight := inner.Height - 1
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	body = lipgloss.NewStyle().Width(inner.Width).Height(bodyHeight).MaxWidth(inner.Width).MaxHeight(bodyHeight).Render(body)
	content := label.Render(title) + "\n" + body
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Width(inner.Width).Height(inner.Height).Render(content)
}

func splitStoryBeats(story string) []string {
	parts := strings.Split(strings.TrimSpace(story), "\n\n")
	beats := make([]string, 0, len(parts))
	for _, part := range parts {
		if beat := strings.TrimSpace(part); beat != "" {
			beats = append(beats, beat)
		}
	}
	if len(beats) == 0 {
		return []string{""}
	}
	return beats
}
