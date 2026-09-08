package terminal

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestScenarioViewRevealsHintsOneAtATime(t *testing.T) {
	saved := 0
	model := scenarioViewModel{
		ctx: context.Background(),
		scenario: ScenarioView{
			Hints: []string{"one", "two", "three"},
			UseHint: func(level int) error {
				saved = level
				return nil
			},
		},
		lab: &memoryLab{},
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyF1})
	got := next.(scenarioViewModel)
	if got.hintLevel != 1 {
		t.Fatalf("hint level = %d, want 1", got.hintLevel)
	}
	if cmd == nil {
		t.Fatal("expected persistence command")
	}
	_ = cmd()
	if saved != 1 {
		t.Fatalf("saved hint level = %d, want 1", saved)
	}
}

func TestQuestionMarkRevealsHintInFooter(t *testing.T) {
	model := scenarioViewModel{
		width:  80,
		height: 24,
		scenario: ScenarioView{
			Title:       "Shell Orientation",
			Objective:   strings.Repeat("Inspect the cluster. ", 12),
			Hints:       []string{"The right pane is a real shell."},
			Description: strings.Repeat("The training surface is a split view.\n\n", 6),
		},
		storyBeats: []string{strings.Repeat("The training surface is a split view. ", 8)},
		lab:        &memoryLab{view: "$ "},
	}
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	got := next.(scenarioViewModel)
	if got.hintLevel != 1 {
		t.Fatalf("hint level = %d, want 1", got.hintLevel)
	}
	view := got.View()
	if !strings.Contains(view, "HINT 1") || !strings.Contains(view, "The right pane is a real shell.") {
		t.Fatalf("hint missing from view:\n%s", view)
	}
}

func TestF1StringAliasRevealsHint(t *testing.T) {
	model := scenarioViewModel{
		scenario: ScenarioView{Hints: []string{"use the lab shell"}},
		lab:      &memoryLab{},
	}
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyF1})
	if next.(scenarioViewModel).hintLevel != 1 {
		t.Fatal("F1 did not reveal a hint")
	}
}

func TestScenarioViewReportsValidationError(t *testing.T) {
	model := scenarioViewModel{
		ctx: context.Background(),
		scenario: ScenarioView{
			Check: func(context.Context) (CheckState, string, error) {
				return CheckWrong, "", errors.New("cluster unavailable")
			},
		},
		lab: &memoryLab{},
	}

	msg := model.checkCmd()().(checkResultMsg)
	next, _ := model.Update(msg)
	got := next.(scenarioViewModel)
	if got.status != "cluster unavailable" {
		t.Fatalf("status = %q", got.status)
	}
}

func TestScenarioViewRequiresExplicitCheck(t *testing.T) {
	called := false
	model := scenarioViewModel{
		ctx: context.Background(),
		scenario: ScenarioView{
			Check: func(context.Context) (CheckState, string, error) {
				called = true
				return CheckSuccess, "done", nil
			},
		},
		lab: &memoryLab{},
	}
	if command := model.Init(); command == nil {
		t.Fatal("Init() should start the lab shell")
	}
	if called {
		t.Fatal("validator ran before an explicit check")
	}
}

func TestObserveWhileRunningChecksOnTick(t *testing.T) {
	model := scenarioViewModel{
		ctx: context.Background(),
		scenario: ScenarioView{
			ObserveWhileRunning: true,
			Check: func(context.Context) (CheckState, string, error) {
				return CheckSuccess, "three nodes", nil
			},
		},
		lab: &memoryLab{},
	}
	next, command := model.Update(observeTickMsg{})
	got := next.(scenarioViewModel)
	if !got.checking || command == nil {
		t.Fatal("observe tick did not start a check")
	}
}

func TestObserveDelayWaitsBeforeAutomaticCheck(t *testing.T) {
	called := false
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	model := scenarioViewModel{
		ctx: context.Background(),
		now: func() time.Time { return now },
		scenario: ScenarioView{
			ObserveWhileRunning: true,
			ObserveDelay:        time.Minute,
			Check: func(context.Context) (CheckState, string, error) {
				called = true
				return CheckSuccess, "too soon", nil
			},
		},
		lab: &memoryLab{},
	}
	next, cmd := model.Update(labReadyMsg{})
	got := next.(scenarioViewModel)
	if cmd == nil {
		t.Fatal("expected wait command")
	}
	if got.observeDeadline != now.Add(time.Minute) {
		t.Fatalf("deadline = %s", got.observeDeadline)
	}
	if !strings.Contains(got.status, "60s") {
		t.Fatalf("status = %q", got.status)
	}

	next, _ = got.Update(observeWaitMsg{})
	got = next.(scenarioViewModel)
	if called || got.checking {
		t.Fatal("automatic check ran before the delay elapsed")
	}
	if !strings.Contains(got.status, "60s") && !strings.Contains(got.status, "59s") {
		t.Fatalf("countdown status = %q", got.status)
	}

	got.now = func() time.Time { return now.Add(time.Minute) }
	next, cmd = got.Update(observeWaitMsg{})
	got = next.(scenarioViewModel)
	if !got.checking || cmd == nil {
		t.Fatal("automatic check did not start after the delay")
	}
}

func TestOrientationAllowsManualCheckDuringDelay(t *testing.T) {
	model := scenarioViewModel{
		ctx: context.Background(),
		scenario: ScenarioView{
			ObserveWhileRunning: true,
			ObserveDelay:        time.Minute,
			Check: func(context.Context) (CheckState, string, error) {
				return CheckSuccess, "manual", nil
			},
		},
		lab:             &memoryLab{},
		observeDeadline: time.Now().Add(time.Minute),
	}
	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyF2})
	got := next.(scenarioViewModel)
	if !got.checking || cmd == nil {
		t.Fatal("F2 should validate during the delay")
	}
}

func TestScenarioViewKeepsShellVisibleAfterSuccess(t *testing.T) {
	model := scenarioViewModel{
		width:  120,
		height: 40,
		scenario: ScenarioView{
			ScenarioID: "pod-creation",
			Module:     "pods",
			Title:      "First API Object",
			HasNext:    true,
			NextTitle:  "First API Object",
			Completion: "Pod nginx exists in kubecrypt-beginner.",
			Debrief:    "The API server stored the Pod object.",
		},
		lab:           &memoryLab{view: "kubectl get nodes"},
		state:         CheckSuccess,
		advancePrompt: true,
	}
	view := model.View()
	for _, text := range []string{"SCENARIO COMPLETED", "LAB SHELL", "kubectl get nodes", "EXPLANATION", "CONTINUE", "First API Object"} {
		if !strings.Contains(view, text) {
			t.Fatalf("success view does not contain %q", text)
		}
	}
}

func TestScenarioViewFitsEightyColumns(t *testing.T) {
	model := scenarioViewModel{
		width:  80,
		height: 36,
		scenario: ScenarioView{
			ScenarioID: "test-scenario",
			Module:     "pods",
			Experience: "beginner",
			Title:      "Test Scenario",
			Objective:  "Inspect the cluster.",
			Namespace:  "kubecrypt-test",
			Resource:   "cluster nodes",
		},
		storyBeats: []string{"The cluster is already running."},
		status:     "Lab Shell is attached. Press F2 to validate cluster state.",
		lab:        &memoryLab{view: "$ "},
	}
	view := model.View()
	for lineNumber, line := range strings.Split(view, "\n") {
		if width := lipgloss.Width(line); width > 80 {
			t.Fatalf("line %d width = %d, want <= 80: %q", lineNumber+1, width, line)
		}
	}
	for _, text := range []string{"OBJECTIVE", "VALIDATION", "LAB SHELL", "F2", "hint"} {
		if !strings.Contains(view, text) {
			t.Fatalf("view does not contain %q", text)
		}
	}
}

func TestYContinuesToNextScenario(t *testing.T) {
	model := scenarioViewModel{
		scenario: ScenarioView{
			HasNext:   true,
			NextTitle: "Cluster Components",
		},
		lab:           &memoryLab{},
		advancePrompt: true,
		state:         CheckSuccess,
	}
	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	got := next.(scenarioViewModel)
	if !got.continueNext {
		t.Fatal("y did not request the next scenario")
	}
	if cmd == nil {
		t.Fatal("expected quit command after continuing")
	}
}

func TestNLeavesAfterCompletion(t *testing.T) {
	model := scenarioViewModel{
		scenario:      ScenarioView{HasNext: true, NextTitle: "Cluster Components"},
		lab:           &memoryLab{},
		advancePrompt: true,
		state:         CheckSuccess,
	}
	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got := next.(scenarioViewModel)
	if got.continueNext {
		t.Fatal("n should not continue to the next scenario")
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestCompletionPromptAppearsAfterSuccessfulCheck(t *testing.T) {
	model := scenarioViewModel{
		ctx:    context.Background(),
		width:  120,
		height: 40,
		scenario: ScenarioView{
			HasNext:   true,
			NextTitle: "Cluster Components",
			Complete:  func() error { return nil },
			Check: func(context.Context) (CheckState, string, error) {
				return CheckSuccess, "ok", nil
			},
		},
		lab: &memoryLab{},
	}
	next, cmd := model.Update(checkResultMsg{state: CheckSuccess, message: "ok"})
	got := next.(scenarioViewModel)
	if cmd == nil {
		t.Fatal("expected completion save command")
	}
	next, _ = got.Update(cmd().(completionSavedMsg))
	got = next.(scenarioViewModel)
	if !got.advancePrompt {
		t.Fatal("completion did not open the continue prompt")
	}
	if !strings.Contains(got.View(), "Continue to Cluster Components") {
		t.Fatalf("missing continue prompt:\n%s", got.View())
	}
}

func TestLetterQDoesNotQuit(t *testing.T) {
	lab := &memoryLab{}
	model := scenarioViewModel{lab: lab}
	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		t.Fatal("q should be typed into the lab shell, not quit the TUI")
	}
	if next.(scenarioViewModel).lab != lab {
		t.Fatal("model lost the lab shell")
	}
	if strings.Join(lab.typed, "") != "q" {
		t.Fatalf("lab received %q", lab.typed)
	}
}

func TestF10Quits(t *testing.T) {
	model := scenarioViewModel{lab: &memoryLab{}}
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyF10})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestF2StartsACheck(t *testing.T) {
	model := scenarioViewModel{
		ctx: context.Background(),
		scenario: ScenarioView{
			Check: func(context.Context) (CheckState, string, error) {
				return CheckSuccess, "ok", nil
			},
		},
		lab: &memoryLab{},
	}
	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyF2})
	got := next.(scenarioViewModel)
	if !got.checking || cmd == nil {
		t.Fatal("F2 did not start validation")
	}
}

func TestPtyLabEchoesACommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY smoke test in short mode")
	}
	lab := newPtyLab(&ShellRunner{
		LookupEnv:  func(string) (string, bool) { return "/bin/sh", true },
		Environ:    func() []string { return []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir()} },
		Executable: func() (string, error) { return "/bin/true", nil },
	})
	defer func() { _ = lab.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := lab.Start(ctx, ShellSession{ScenarioID: "pty-smoke"}, 80, 24); err != nil {
		t.Fatalf("start lab: %v", err)
	}
	if err := lab.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("echo kubecrypt-split")}); err != nil {
		t.Fatal(err)
	}
	if err := lab.HandleKey(tea.KeyMsg{Type: tea.KeyEnter}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(lab.View(), "kubecrypt-split") {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("PTY view never showed echo output:\n%s", lab.View())
}
