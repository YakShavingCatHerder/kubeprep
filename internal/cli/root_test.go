package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/YakShavingCatHerder/kubecrypt/internal/curriculum"
	"github.com/YakShavingCatHerder/kubecrypt/internal/game"
	"github.com/spf13/cobra"
)

func TestBareCommandShowsHelp(t *testing.T) {
	var output bytes.Buffer
	a := &app{in: strings.NewReader(""), out: &output, err: &output}
	root := a.rootCommand()
	root.SetArgs([]string{})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("kubecrypt: %v", err)
	}
	got := output.String()
	if !strings.Contains(got, "kubecrypt [command] [flags]") {
		t.Errorf("help missing combined usage:\n%s", got)
	}
	if strings.Contains(got, "kubecrypt [flags]\n") || strings.Contains(got, "  kubecrypt [command]\n") {
		t.Errorf("help still splits usage onto two lines:\n%s", got)
	}
	for _, want := range []string{"start", "status", "reset", "destroy", "doctor"} {
		if !strings.Contains(got, want) {
			t.Errorf("help missing %q:\n%s", want, got)
		}
	}
	for _, hidden := range []string{"\n  pack ", "\n  check ", "\n  setup ", "\n  resume ", "\n  completion ", "\n  help "} {
		if strings.Contains(got, hidden) {
			t.Errorf("help still lists %q:\n%s", strings.TrimSpace(hidden), got)
		}
	}
}

func TestRootUsesNeutralCommands(t *testing.T) {
	a := &app{in: strings.NewReader(""), out: &bytes.Buffer{}, err: &bytes.Buffer{}}
	root := a.rootCommand()
	names := make(map[string]bool)
	for _, command := range root.Commands() {
		if command.Hidden {
			continue
		}
		names[command.Name()] = true
	}
	for _, expected := range []string{"start", "status", "reset", "destroy", "doctor"} {
		if !names[expected] {
			t.Errorf("missing command %q", expected)
		}
	}
	for _, removed := range []string{"setup", "resume", "check", "hint", "objective", "completion", "help", "check-in", "checkout", "lesson"} {
		if names[removed] {
			t.Errorf("removed command %q is still visible", removed)
		}
	}
}

func TestRootReportsVersion(t *testing.T) {
	previous := Version
	Version = "test-0.1.0"
	t.Cleanup(func() { Version = previous })

	var output bytes.Buffer
	a := &app{in: strings.NewReader(""), out: &output, err: &output}
	root := a.rootCommand()
	root.SetArgs([]string{"--version"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("--version: %v", err)
	}
	if got := strings.TrimSpace(output.String()); got != "kubecrypt test-0.1.0" {
		t.Fatalf("--version = %q", got)
	}
}

func TestCurrentScenarioAdvancesThroughCatalog(t *testing.T) {
	store, err := game.NewStore(game.WithConfigDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := curriculum.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := currentScenario(store, registry)
	if err != nil {
		t.Fatal(err)
	}
	if scenario.ID != "pod-creation" {
		t.Fatalf("first scenario = %q", scenario.ID)
	}
	if err := store.CompleteScenario(scenario.ID); err != nil {
		t.Fatal(err)
	}
	scenario, err = currentScenario(store, registry)
	if err != nil {
		t.Fatal(err)
	}
	if scenario.ID != "pod-creation" {
		t.Fatalf("completed catalog still resumes %q", scenario.ID)
	}
	if err := store.SelectScenario("pod-creation"); err != nil {
		t.Fatal(err)
	}
	scenario, err = currentScenario(store, registry)
	if err != nil {
		t.Fatal(err)
	}
	if scenario.ID != "pod-creation" {
		t.Fatalf("selected scenario = %q", scenario.ID)
	}
}

func TestPinLabWorkspaceUsesDeclaredOrDefault(t *testing.T) {
	if got := contextNamespaceForLab(&curriculum.Scenario{Namespace: "kubecrypt-beginner"}); got != "kubecrypt-beginner" {
		t.Fatalf("declared namespace = %q", got)
	}
	if got := contextNamespaceForLab(&curriculum.Scenario{}); got != "default" {
		t.Fatalf("empty namespace = %q, want default", got)
	}
}

func TestShouldWipeLabWorkspace(t *testing.T) {
	tests := []struct {
		current  string
		entering string
		want     bool
	}{
		{current: "", entering: "pod-creation", want: true},
		{current: "other-lab", entering: "pod-creation", want: true},
		{current: "pod-creation", entering: "pod-creation", want: false},
	}
	for _, test := range tests {
		if got := shouldWipeLabWorkspace(test.current, test.entering); got != test.want {
			t.Errorf("shouldWipeLabWorkspace(%q, %q) = %v, want %v", test.current, test.entering, got, test.want)
		}
	}
}

func TestFollowingIncompleteScenario(t *testing.T) {
	store, err := game.NewStore(game.WithConfigDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := curriculum.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	next, err := followingIncompleteScenario(store, registry, "")
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.ID != "pod-creation" {
		t.Fatalf("following scenario = %v", next)
	}
	if err := store.CompleteScenario("pod-creation"); err != nil {
		t.Fatal(err)
	}
	next, err = followingIncompleteScenario(store, registry, "")
	if err != nil {
		t.Fatal(err)
	}
	if next != nil {
		t.Fatalf("expected no following incomplete scenario, got %s", next.ID)
	}
}

func TestCurrentScenarioSkipsTutorialForCertificationTrack(t *testing.T) {
	store, err := game.NewStore(game.WithConfigDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveProfile(game.Profile{
		Experience:         game.ExperienceCKACandidate,
		OnboardingComplete: true,
	}); err != nil {
		t.Fatal(err)
	}
	registry, err := curriculum.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := currentScenario(store, registry)
	if err != nil {
		t.Fatal(err)
	}
	if scenario.ID != "pod-creation" {
		t.Fatalf("first CKA scenario = %q, want pod-creation", scenario.ID)
	}
}

func TestEnsureProfileRequiresTrackWhenNonInteractive(t *testing.T) {
	store, err := game.NewStore(game.WithConfigDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	a := &app{}
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&bytes.Buffer{})

	if _, err := a.ensureProfile(cmd, store, ""); err == nil || !strings.Contains(err.Error(), "--track") {
		t.Fatalf("ensureProfile() error = %v", err)
	}
}

func TestEnsureProfileAcceptsBeginnerTrack(t *testing.T) {
	store, err := game.NewStore(game.WithConfigDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	a := &app{}
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&bytes.Buffer{})

	profile, err := a.ensureProfile(cmd, store, "beginner")
	if err != nil {
		t.Fatalf("ensureProfile(): %v", err)
	}
	if profile.Experience != game.ExperienceBeginner || !profile.OnboardingComplete {
		t.Fatalf("unexpected profile: %#v", profile)
	}
}

func TestEnsureProfileAcceptsCertificationTrack(t *testing.T) {
	store, err := game.NewStore(game.WithConfigDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	a := &app{}
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&bytes.Buffer{})

	profile, err := a.ensureProfile(cmd, store, "ckad")
	if err != nil {
		t.Fatalf("ensureProfile(): %v", err)
	}
	if profile.Experience != game.ExperienceCKADCandidate || !profile.OnboardingComplete {
		t.Fatalf("unexpected profile: %#v", profile)
	}
}
