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

func TestObjectiveCommandUsesBundledScenario(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var output bytes.Buffer
	a := &app{in: strings.NewReader(""), out: &output, err: &output}
	root := a.rootCommand()
	root.SetArgs([]string{"objective"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("objective: %v", err)
	}
	if strings.TrimSpace(output.String()) == "" {
		t.Fatalf("unexpected objective output: %q", output.String())
	}
}

func TestRootUsesNeutralCommands(t *testing.T) {
	a := &app{in: strings.NewReader(""), out: &bytes.Buffer{}, err: &bytes.Buffer{}}
	root := a.rootCommand()
	names := make(map[string]bool)
	for _, command := range root.Commands() {
		names[command.Name()] = true
	}
	for _, expected := range []string{"setup", "destroy", "pack"} {
		if !names[expected] {
			t.Errorf("missing command %q", expected)
		}
	}
	for _, removed := range []string{"check-in", "checkout", "lesson"} {
		if names[removed] {
			t.Errorf("themed command %q is still registered", removed)
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
	if scenario.ID != "shell-orientation" {
		t.Fatalf("first scenario = %q", scenario.ID)
	}
	if err := store.CompleteScenario(scenario.ID); err != nil {
		t.Fatal(err)
	}
	scenario, err = currentScenario(store, registry)
	if err != nil {
		t.Fatal(err)
	}
	if scenario.ID != "cluster-components" {
		t.Fatalf("second scenario = %q", scenario.ID)
	}
	if err := store.CompleteScenario(scenario.ID); err != nil {
		t.Fatal(err)
	}
	scenario, err = currentScenario(store, registry)
	if err != nil {
		t.Fatal(err)
	}
	if scenario.ID != "pod-creation" {
		t.Fatalf("third scenario = %q", scenario.ID)
	}
	if err := store.SelectScenario("shell-orientation"); err != nil {
		t.Fatal(err)
	}
	scenario, err = currentScenario(store, registry)
	if err != nil {
		t.Fatal(err)
	}
	if scenario.ID != "shell-orientation" {
		t.Fatalf("selected scenario = %q", scenario.ID)
	}
}

func TestShouldWipeLabWorkspace(t *testing.T) {
	tests := []struct {
		current  string
		entering string
		want     bool
	}{
		{current: "", entering: "pod-creation", want: true},
		{current: "shell-orientation", entering: "pod-creation", want: true},
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
	next, err := followingIncompleteScenario(store, registry, "shell-orientation")
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.ID != "cluster-components" {
		t.Fatalf("following scenario = %v", next)
	}
	if err := store.CompleteScenario("cluster-components"); err != nil {
		t.Fatal(err)
	}
	next, err = followingIncompleteScenario(store, registry, "shell-orientation")
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.ID != "pod-creation" {
		t.Fatalf("following scenario after cluster-components = %v", next)
	}
	if err := store.CompleteScenario("pod-creation"); err != nil {
		t.Fatal(err)
	}
	next, err = followingIncompleteScenario(store, registry, "shell-orientation")
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
	if scenario.ID != "cluster-components" {
		t.Fatalf("first CKA scenario = %q, want cluster-components", scenario.ID)
	}
}

func TestEnsureProfileRequiresTutorialChoiceWhenNonInteractive(t *testing.T) {
	store, err := game.NewStore(game.WithConfigDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	a := &app{}
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&bytes.Buffer{})

	if _, err := a.ensureProfile(cmd, store, "", ""); err == nil || !strings.Contains(err.Error(), "--tutorial") {
		t.Fatalf("ensureProfile() error = %v", err)
	}
}

func TestEnsureProfileAcceptsExplicitTutorial(t *testing.T) {
	store, err := game.NewStore(game.WithConfigDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	a := &app{}
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&bytes.Buffer{})

	profile, err := a.ensureProfile(cmd, store, "yes", "")
	if err != nil {
		t.Fatalf("ensureProfile(): %v", err)
	}
	if profile.Experience != game.ExperienceBeginner || !profile.OnboardingComplete {
		t.Fatalf("unexpected profile: %#v", profile)
	}
}

func TestEnsureProfileAcceptsSkippedTutorialTrack(t *testing.T) {
	store, err := game.NewStore(game.WithConfigDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	a := &app{}
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&bytes.Buffer{})

	profile, err := a.ensureProfile(cmd, store, "no", "ckad")
	if err != nil {
		t.Fatalf("ensureProfile(): %v", err)
	}
	if profile.Experience != game.ExperienceCKADCandidate || !profile.OnboardingComplete {
		t.Fatalf("unexpected profile: %#v", profile)
	}
}
