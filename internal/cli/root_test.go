package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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
	for _, want := range []string{"start", "lab", "status", "reset", "destroy", "doctor"} {
		if !strings.Contains(got, want) {
			t.Errorf("help missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "--pack") {
		t.Errorf("help still lists --pack:\n%s", got)
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
	for _, expected := range []string{"start", "lab", "status", "reset", "destroy", "doctor"} {
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

func TestLabTryRequiresLabFile(t *testing.T) {
	var output bytes.Buffer
	a := &app{in: strings.NewReader(""), out: &output, err: &output}
	root := a.rootCommand()
	root.SetArgs([]string{"lab", "try"})
	err := root.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("lab try without a file should fail")
	}
}

func TestLabTryRejectsMissingFile(t *testing.T) {
	t.Chdir(t.TempDir())
	var output bytes.Buffer
	a := &app{in: strings.NewReader(""), out: &output, err: &output}
	root := a.rootCommand()
	root.SetArgs([]string{"lab", "try", "missing.yaml", "--track=beginner", "--prepare-only"})
	err := root.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("lab try missing.yaml should fail")
	}
	if strings.Contains(err.Error(), "curriculum") {
		t.Fatalf("lab try should not require curriculum/: %v", err)
	}
}

func TestLabPublishRequiresCurriculum(t *testing.T) {
	t.Chdir(t.TempDir())
	var output bytes.Buffer
	a := &app{in: strings.NewReader(""), out: &output, err: &output}
	root := a.rootCommand()
	root.SetArgs([]string{"lab", "publish", "missing.yaml", "--track=beginner", "--prepare-only"})
	err := root.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("lab publish without curriculum/ should fail")
	}
	if !strings.Contains(err.Error(), "curriculum") {
		t.Fatalf("lab publish error = %v", err)
	}
}

func TestLabPublishRequiresLabFile(t *testing.T) {
	var output bytes.Buffer
	a := &app{in: strings.NewReader(""), out: &output, err: &output}
	root := a.rootCommand()
	root.SetArgs([]string{"lab", "publish"})
	err := root.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("lab publish without a file should fail")
	}
}

func TestResolveContributeLab(t *testing.T) {
	got, err := resolveContributeLab("test-lab.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join("contribute", "test-lab.yaml") {
		t.Fatalf("got %q", got)
	}
	got, err = resolveContributeLab("contribute/test-lab.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join("contribute", "test-lab.yaml") {
		t.Fatalf("prefixed got %q", got)
	}
	if _, err := resolveContributeLab("../secret.yaml"); err == nil {
		t.Fatal("expected escape to fail")
	}
	if _, err := resolveContributeLab("/tmp/lab.yaml"); err == nil {
		t.Fatal("expected absolute path to fail")
	}
}

func TestLabTryIsVisible(t *testing.T) {
	a := &app{in: strings.NewReader(""), out: &bytes.Buffer{}, err: &bytes.Buffer{}}
	root := a.rootCommand()
	lab, _, err := root.Find([]string{"lab", "try"})
	if err != nil {
		t.Fatal(err)
	}
	if lab.Hidden || lab.Name() != "try" {
		t.Fatalf("lab try hidden=%v name=%q", lab.Hidden, lab.Name())
	}
}

func TestLabPublishIsVisible(t *testing.T) {
	a := &app{in: strings.NewReader(""), out: &bytes.Buffer{}, err: &bytes.Buffer{}}
	root := a.rootCommand()
	lab, _, err := root.Find([]string{"lab", "publish"})
	if err != nil {
		t.Fatal(err)
	}
	if lab.Hidden || lab.Name() != "publish" {
		t.Fatalf("lab publish hidden=%v name=%q", lab.Hidden, lab.Name())
	}
}

func TestPrepareTryDoesNotWriteCurriculum(t *testing.T) {
	repo := t.TempDir()
	t.Chdir(repo)
	if err := os.Mkdir("curriculum", 0o700); err != nil {
		t.Fatal(err)
	}
	marker := []byte("do-not-touch\n")
	if err := os.WriteFile(filepath.Join("curriculum", "catalog.yaml"), marker, 0o600); err != nil {
		t.Fatal(err)
	}
	source := writeCLITryLab(t, t.TempDir(), "draft.yaml")
	a := &app{}
	cleanup, err := a.prepareTry(source)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	got, err := os.ReadFile(filepath.Join("curriculum", "catalog.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(marker) {
		t.Fatalf("try mutated curriculum catalog:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join("curriculum", "workloads")); !os.IsNotExist(err) {
		t.Fatalf("try wrote curriculum/workloads: %v", err)
	}
	if a.livePackDir == "" || a.livePackDir == liveCurriculumDir {
		t.Fatalf("livePackDir = %q", a.livePackDir)
	}
	if a.targetLabID != "test-scenario" {
		t.Fatalf("targetLabID = %q", a.targetLabID)
	}
	if !a.preview {
		t.Fatal("preview should be set")
	}
	if _, err := os.Stat(filepath.Join(a.livePackDir, "workloads", "test-scenario.yaml")); err != nil {
		t.Fatal(err)
	}
}

func writeCLITryLab(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(dir, name)
	const body = `authoring:
  section: workloads
apiVersion: kubecrypt.io/v1alpha1
id: test-scenario
mode: challenge
title: Test Scenario
description: Restore the workload.
revision: 2026-09
module: workloads
namespace: kubecrypt-test
difficulty: 1
kubernetes:
  min: "1.35"
  max: "1.35"
tracks:
  - beginner
  - cka
  - ckad
objective: Make the workload available.
concepts:
  - pods
setup:
  manifests: []
checks:
  - type: objectExists
    kind: pod
    namespace: kubecrypt-test
    name: nginx
hints:
  - conceptual
  - procedural
  - explicit
completion: The workload is available.
debrief:
  explanation: The Pod exists.
reset:
  manifests: []
`
	if err := os.WriteFile(filename, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return filename
}
