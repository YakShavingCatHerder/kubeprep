package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/YakShavingCatHerder/kubeprep/internal/curriculum"
	"github.com/YakShavingCatHerder/kubeprep/internal/game"
	"github.com/YakShavingCatHerder/kubeprep/internal/validator"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func TestEvaluateUngradedScenarioPassesWithoutCluster(t *testing.T) {
	result, err := evaluateScenario(context.Background(), &curriculum.Scenario{Ungraded: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != validator.Success {
		t.Fatalf("status = %s, want success", result.Status)
	}
}

func TestSetupObjectProbeAllowsUngradedLabs(t *testing.T) {
	kind, namespace, name := setupObjectProbe(&curriculum.Scenario{Ungraded: true})
	if kind != "" || namespace != "" || name != "" {
		t.Fatalf("probe = %s %s/%s, want empty", kind, namespace, name)
	}
	kind, namespace, name = setupObjectProbe(&curriculum.Scenario{
		Checks: []curriculum.Check{{
			Type:      curriculum.CheckObjectExists,
			Kind:      "pod",
			Namespace: "kubeprep-test",
			Name:      "welcome",
		}},
	})
	if kind != "pod" || namespace != "kubeprep-test" || name != "welcome" {
		t.Fatalf("probe = %s %s/%s", kind, namespace, name)
	}
}

func TestBareCommandShowsHelp(t *testing.T) {
	var output bytes.Buffer
	a := &app{in: strings.NewReader(""), out: &output, err: &output}
	root := a.rootCommand()
	root.SetArgs([]string{})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("kubeprep: %v", err)
	}
	got := output.String()
	if !strings.Contains(got, "kubeprep [command] [flags]") {
		t.Errorf("help missing combined usage:\n%s", got)
	}
	if strings.Contains(got, "kubeprep [flags]\n") || strings.Contains(got, "  kubeprep [command]\n") {
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
	for _, removed := range []string{"setup", "resume", "check", "hint", "objective", "completion", "help", "check-in", "checkout", "lesson", "pack"} {
		if names[removed] {
			t.Errorf("removed command %q is still visible", removed)
		}
	}
}

func TestDestroyAllIsASubcommand(t *testing.T) {
	a := &app{in: strings.NewReader(""), out: &bytes.Buffer{}, err: &bytes.Buffer{}}
	root := a.rootCommand()
	cmd, args, err := root.Find([]string{"destroy", "all"})
	if err != nil {
		t.Fatalf("find destroy all: %v", err)
	}
	if cmd.Name() != "all" {
		t.Fatalf("destroy all resolved to %q", cmd.Name())
	}
	if len(args) != 0 {
		t.Fatalf("destroy all leftover args = %v", args)
	}
	if cmd.InheritedFlags().Lookup("yes") == nil && cmd.Flags().Lookup("yes") == nil {
		t.Fatal("destroy all is missing -y")
	}
}

func TestDestroyHelpListsAllActionAndYesFlag(t *testing.T) {
	var output bytes.Buffer
	a := &app{in: strings.NewReader(""), out: &output, err: &output}
	root := a.rootCommand()
	root.SetArgs([]string{"destroy", "--help"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("destroy --help: %v", err)
	}
	got := output.String()
	if !strings.Contains(got, "  all ") {
		t.Fatalf("destroy help missing all action:\n%s", got)
	}
	if !strings.Contains(got, "-y, --yes") {
		t.Fatalf("destroy help missing -y:\n%s", got)
	}
	if strings.Contains(got, "--all") {
		t.Fatalf("destroy help still lists --all:\n%s", got)
	}
	if strings.Contains(got, "--force") {
		t.Fatalf("destroy help still lists --force:\n%s", got)
	}
}

func TestDestroyAllFlagRedirectsToAction(t *testing.T) {
	var output bytes.Buffer
	a := &app{in: strings.NewReader(""), out: &output, err: &output}
	root := a.rootCommand()
	root.SetArgs([]string{"destroy", "--all"})
	err := root.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "kubeprep destroy all") {
		t.Fatalf("destroy --all error = %v, want a destroy all redirect", err)
	}
}

func TestDestroyAllRequiresYesWhenNonInteractive(t *testing.T) {
	var output bytes.Buffer
	a := &app{in: strings.NewReader(""), out: &output, err: &output}
	root := a.rootCommand()
	root.SetArgs([]string{"destroy", "all"})
	err := root.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "-y") {
		t.Fatalf("destroy all error = %v, want a -y requirement", err)
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
	if got := strings.TrimSpace(output.String()); got != "kubeprep test-0.1.0" {
		t.Fatalf("--version = %q", got)
	}
}

func TestCurrentScenarioAdvancesThroughCatalog(t *testing.T) {
	store, err := game.NewStore(game.WithConfigDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	registry := playlistRegistry(t, playlistPath{id: "beginner", labs: []string{"alpha", "beta", "gamma"}})
	order := registry.PlayOrder("beginner")
	if len(order) < 2 {
		t.Fatalf("fixture play order = %v", order)
	}
	scenario, err := currentScenario(store, registry)
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range order {
		if scenario.ID != id {
			t.Fatalf("scenario %d = %q, want %q", i+1, scenario.ID, id)
		}
		if err := store.CompleteScenario(scenario.ID); err != nil {
			t.Fatal(err)
		}
		scenario, err = currentScenario(store, registry)
		if err != nil {
			t.Fatal(err)
		}
	}
	last := order[len(order)-1]
	if scenario.ID != last {
		t.Fatalf("completed catalog still resumes %q, want %q", scenario.ID, last)
	}
	if err := store.SelectScenario(order[0]); err != nil {
		t.Fatal(err)
	}
	scenario, err = currentScenario(store, registry)
	if err != nil {
		t.Fatal(err)
	}
	if scenario.ID != order[0] {
		t.Fatalf("selected scenario = %q, want %q", scenario.ID, order[0])
	}
}

func TestPinLabWorkspaceUsesDeclaredOrDefault(t *testing.T) {
	if got := contextNamespaceForLab(&curriculum.Scenario{Namespace: "kubeprep-beginner"}); got != "kubeprep-beginner" {
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
	registry := playlistRegistry(t, playlistPath{id: "beginner", labs: []string{"alpha", "beta", "gamma"}})
	order := registry.PlayOrder("beginner")
	if len(order) < 2 {
		t.Fatalf("fixture play order = %v", order)
	}
	next, err := followingIncompleteScenario(store, registry, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range order {
		if next == nil || next.ID != id {
			t.Fatalf("following scenario = %v, want %s", next, id)
		}
		if err := store.CompleteScenario(id); err != nil {
			t.Fatal(err)
		}
		next, err = followingIncompleteScenario(store, registry, "")
		if err != nil {
			t.Fatal(err)
		}
	}
	if next != nil {
		t.Fatalf("expected no following incomplete scenario, got %s", next.ID)
	}
}

func TestFollowingIncompleteScenarioSkipsCompletedLabs(t *testing.T) {
	store, err := game.NewStore(game.WithConfigDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	registry := playlistRegistry(t, playlistPath{id: "beginner", labs: []string{"alpha", "beta", "gamma"}})
	if err := store.CompleteScenario("beta"); err != nil {
		t.Fatal(err)
	}
	next, err := followingIncompleteScenario(store, registry, "")
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.ID != "alpha" {
		t.Fatalf("first incomplete = %v, want alpha", next)
	}
	if err := store.CompleteScenario("alpha"); err != nil {
		t.Fatal(err)
	}
	next, err = followingIncompleteScenario(store, registry, "")
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.ID != "gamma" {
		t.Fatalf("after skipping completed = %v, want gamma", next)
	}
}

func TestCurrentScenarioUsesCertificationTrackPlayOrder(t *testing.T) {
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
	registry := playlistRegistry(t,
		playlistPath{id: "beginner", labs: []string{"alpha", "beta"}},
		playlistPath{id: "cka", labs: []string{"beta"}},
	)
	scenario, err := currentScenario(store, registry)
	if err != nil {
		t.Fatal(err)
	}
	if scenario.ID != "beta" {
		t.Fatalf("first CKA scenario = %q, want beta", scenario.ID)
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

func TestLabValidateIsVisible(t *testing.T) {
	a := &app{in: strings.NewReader(""), out: &bytes.Buffer{}, err: &bytes.Buffer{}}
	root := a.rootCommand()
	lab, _, err := root.Find([]string{"lab", "validate"})
	if err != nil {
		t.Fatal(err)
	}
	if lab.Hidden || lab.Name() != "validate" {
		t.Fatalf("lab validate hidden=%v name=%q", lab.Hidden, lab.Name())
	}
}

func TestLabValidateAcceptsMaterializedDraft(t *testing.T) {
	source := writeCLITryLab(t, t.TempDir(), "draft.yaml")
	packDir := t.TempDir()
	if _, err := curriculum.MaterializeDraft(source, packDir); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	a := &app{in: strings.NewReader(""), out: &output, err: &output}
	root := a.rootCommand()
	root.SetArgs([]string{"lab", "validate", packDir})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.Contains(got, "validated draft (1 lab)") {
		t.Fatalf("lab validate output = %q", got)
	}
}

func TestLabValidateRejectsMissingDirectory(t *testing.T) {
	var output bytes.Buffer
	a := &app{in: strings.NewReader(""), out: &output, err: &output}
	root := a.rootCommand()
	root.SetArgs([]string{"lab", "validate", filepath.Join(t.TempDir(), "missing")})
	if err := root.ExecuteContext(context.Background()); err == nil {
		t.Fatal("lab validate missing dir should fail")
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

type playlistPath struct {
	id   string
	labs []string
}

func playlistRegistry(t *testing.T, paths ...playlistPath) *curriculum.Registry {
	t.Helper()
	if len(paths) == 0 {
		t.Fatal("playlistRegistry requires at least one path")
	}
	catalog := curriculum.Catalog{
		APIVersion: curriculum.CatalogAPIVersionV1Alpha1,
		Name:       "test-pack",
		Title:      "Test Pack",
		Revision:   "2026-09",
	}
	files := fstest.MapFS{}
	seen := make(map[string]struct{})
	for _, learningPath := range paths {
		catalog.Paths = append(catalog.Paths, curriculum.Path{
			ID:    learningPath.id,
			Title: learningPath.id,
			Sections: []curriculum.Section{{
				ID:   "fixtures",
				Labs: learningPath.labs,
			}},
		})
		for _, id := range learningPath.labs {
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			data, err := yaml.Marshal(fixtureScenario(id))
			if err != nil {
				t.Fatal(err)
			}
			files[curriculum.LabFilePath("fixtures", id)] = &fstest.MapFile{Data: data}
		}
	}
	catalogData, err := yaml.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	files["catalog.yaml"] = &fstest.MapFile{Data: catalogData}
	registry, err := curriculum.NewRegistryFromSources(curriculum.Source{Name: "local", FS: files})
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func fixtureScenario(id string) *curriculum.Scenario {
	return &curriculum.Scenario{
		APIVersion:  curriculum.APIVersionV1Alpha1,
		ID:          id,
		Mode:        "challenge",
		Title:       id,
		Description: "Fixture lab.",
		Revision:    "2026-09",
		Module:      "fixtures",
		Difficulty:  1,
		Kubernetes:  curriculum.KubernetesCompatibility{Min: "1.35", Max: "1.35"},
		Tracks:      []string{"beginner", "cka", "ckad"},
		Objective:   "Finish the fixture.",
		Concepts:    []string{"pods"},
		Namespace:   "kubeprep-fixtures",
		Ungraded:    true,
		Hints:       []string{"conceptual", "procedural", "explicit"},
		Completion:  "Done.",
		Debrief:     curriculum.Debrief{Explanation: "Fixture."},
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
apiVersion: kubeprep.io/v1alpha1
id: test-scenario
mode: challenge
title: Test Scenario
description: Restore the workload.
revision: 2026-09
module: workloads
namespace: kubeprep-test
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
    namespace: kubeprep-test
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
