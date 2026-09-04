package curriculum

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"gopkg.in/yaml.v3"
)

func TestBundledRegistryLoadsCoreScenarios(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	ids := registry.ScenarioIDs()
	if strings.Join(ids, ",") != "shell-orientation,cluster-components,pod-creation" {
		t.Fatalf("scenario order = %v", ids)
	}
	scenario, err := registry.LoadScenario("cluster-components")
	if err != nil {
		t.Fatal(err)
	}
	if scenario.Checks[0].Type != CheckNodeTopology {
		t.Fatalf("first check = %q", scenario.Checks[0].Type)
	}
}

func TestLoadFileCanonicalScenario(t *testing.T) {
	filename := filepath.Join("..", "..", "curriculum", "00-orientation", "02-cluster-components.yaml")
	scenario, err := LoadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if scenario.ID != "cluster-components" || scenario.Module != "orientation" {
		t.Fatalf("unexpected scenario: %#v", scenario)
	}
}

func TestLoadFilePodCreation(t *testing.T) {
	filename := filepath.Join("..", "..", "curriculum", "01-foundations", "01-pod-creation.yaml")
	scenario, err := LoadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if scenario.ID != "pod-creation" || scenario.Module != "foundations" {
		t.Fatalf("unexpected scenario: %#v", scenario)
	}
	if scenario.Mode != "challenge" {
		t.Fatalf("mode = %q, want challenge", scenario.Mode)
	}
	if scenario.Namespace != "kubecrypt-foundations" {
		t.Fatalf("namespace = %q", scenario.Namespace)
	}
	if len(scenario.Setup.Manifests) != 0 {
		t.Fatalf("setup manifests = %#v", scenario.Setup.Manifests)
	}
	if len(scenario.Reset.Manifests) != 0 {
		t.Fatalf("reset manifests = %#v", scenario.Reset.Manifests)
	}
	if len(scenario.Checks) != 2 {
		t.Fatalf("checks = %d, want 2", len(scenario.Checks))
	}
	exists := scenario.Checks[0]
	if exists.Type != CheckObjectExists || exists.Kind != "pod" || exists.Namespace != "kubecrypt-foundations" || exists.Name != "nginx" {
		t.Fatalf("objectExists check = %#v", exists)
	}
	image := scenario.Checks[1]
	if image.Type != CheckFieldEquals || image.Field != "spec.containers[0].image" || image.Value != "nginx:1.27" {
		t.Fatalf("fieldEquals check = %#v", image)
	}
}

func TestParseObserveDelay(t *testing.T) {
	got, err := ParseObserveDelay("60s")
	if err != nil {
		t.Fatal(err)
	}
	if got != time.Minute {
		t.Fatalf("delay = %s, want 1m", got)
	}
	if _, err := ParseObserveDelay("later"); err == nil {
		t.Fatal("expected invalid duration error")
	}
}

func TestBundledOrientationScenariosPauseBeforeValidation(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"shell-orientation", "cluster-components"} {
		scenario, err := registry.LoadScenario(id)
		if err != nil {
			t.Fatal(err)
		}
		delay, err := ParseObserveDelay(scenario.ObserveDelay)
		if err != nil {
			t.Fatal(err)
		}
		if delay != time.Minute {
			t.Fatalf("%s observeDelay = %s, want 60s", id, scenario.ObserveDelay)
		}
	}
}

func TestBundledAssetsMatchCanonicalCurriculum(t *testing.T) {
	files := []string{
		"catalog.yaml",
		"00-orientation/01-shell-orientation.yaml",
		"00-orientation/02-cluster-components.yaml",
		"01-foundations/01-pod-creation.yaml",
	}
	for _, name := range files {
		canonical, err := os.ReadFile(filepath.Join("..", "..", "curriculum", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		bundled, err := bundledFiles.ReadFile("bundled/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(canonical, bundled) {
			t.Errorf("bundled %s differs from canonical pack", name)
		}
	}
}

func TestLoadFSRejectsUnknownFields(t *testing.T) {
	data, err := yaml.Marshal(validScenario())
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("hostCommand: rm -rf /\n")...)
	files := scenarioFS(data)
	_, err = LoadFS(files, "scenario.yaml")
	if err == nil || !strings.Contains(err.Error(), "field hostCommand not found") {
		t.Fatalf("LoadFS() error = %v", err)
	}
}

func TestLoadFSMissingManifest(t *testing.T) {
	data, err := yaml.Marshal(validScenario())
	if err != nil {
		t.Fatal(err)
	}
	_, err = LoadFS(fstest.MapFS{"scenario.yaml": {Data: data}}, "scenario.yaml")
	if err == nil || !strings.Contains(err.Error(), `manifest "workload.yaml"`) {
		t.Fatalf("LoadFS() error = %v", err)
	}
}

func TestRegistryLoadsLocalPack(t *testing.T) {
	scenarioData, err := yaml.Marshal(validScenario())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistryFromSources(Source{Name: "local", FS: packFS(scenarioData)})
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := registry.LoadScenario("test-scenario")
	if err != nil {
		t.Fatal(err)
	}
	if scenario.Title != "Test Scenario" {
		t.Fatalf("title = %q", scenario.Title)
	}
	manifest, err := registry.ReadManifest(scenario.ID, "workload.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(manifest, []byte("kubecrypt-test")) {
		t.Fatalf("unexpected manifest: %s", manifest)
	}
}

func TestNewRegistryAppendsLocalPackToBundledScenarios(t *testing.T) {
	packDir := t.TempDir()
	scenarioData, err := yaml.Marshal(validScenario())
	if err != nil {
		t.Fatal(err)
	}
	for name, file := range packFS(scenarioData) {
		dest := filepath.Join(packDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, file.Data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	registry, err := NewRegistry(packDir)
	if err != nil {
		t.Fatal(err)
	}
	ids := registry.ScenarioIDs()
	if got := strings.Join(ids, ","); got != "shell-orientation,cluster-components,pod-creation,test-scenario" {
		t.Fatalf("scenario order = %s", got)
	}
}

func TestRegistryRejectsDuplicateScenarioIDs(t *testing.T) {
	data, err := yaml.Marshal(validScenario())
	if err != nil {
		t.Fatal(err)
	}
	first := packFS(data)
	second := packFS(data)
	second["catalog.yaml"].Data = bytes.ReplaceAll(second["catalog.yaml"].Data, []byte("name: test-pack"), []byte("name: other-pack"))
	_, err = NewRegistryFromSources(Source{Name: "first", FS: first}, Source{Name: "second", FS: second})
	if err == nil || !strings.Contains(err.Error(), "duplicate scenario id") {
		t.Fatalf("NewRegistryFromSources() error = %v", err)
	}
}

func TestValidateCatalogRejectsUnsafePath(t *testing.T) {
	catalog := validCatalog()
	catalog.Modules[0].Scenarios[0].Path = "../scenario.yaml"
	err := validateCatalog(&catalog)
	if err == nil || !strings.Contains(err.Error(), "clean relative path") {
		t.Fatalf("validateCatalog() error = %v", err)
	}
}

func TestValidatePackRejectsEscapingSymlink(t *testing.T) {
	packDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "scenario.yaml")
	if err := os.WriteFile(outside, []byte("not a scenario"), 0o600); err != nil {
		t.Fatal(err)
	}
	scenarioPath := filepath.Join(packDir, "02-workloads", "01-test-scenario.yaml")
	if err := os.MkdirAll(filepath.Dir(scenarioPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, scenarioPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	catalogData, err := yaml.Marshal(validCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "catalog.yaml"), catalogData, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidatePack(packDir); err == nil {
		t.Fatal("ValidatePack() accepted a scenario symlink outside the pack")
	}
}

func TestValidateScenario(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Scenario)
		want   string
	}{
		{"invalid id", func(s *Scenario) { s.ID = "Bad ID" }, "lowercase kebab-case"},
		{"missing description", func(s *Scenario) { s.Description = "" }, "description"},
		{"unsupported track", func(s *Scenario) { s.Tracks = []string{"cissp"} }, "unsupported track"},
		{"not exactly three hints", func(s *Scenario) { s.Hints = s.Hints[:2] }, "exactly 3"},
		{"empty hint", func(s *Scenario) { s.Hints[1] = " " }, "hints[1]"},
		{"unsafe manifest reference", func(s *Scenario) { s.Setup.Manifests[0] = "../workload.yaml" }, "clean relative path"},
		{"unknown typed check", func(s *Scenario) { s.Checks[0].Type = "runCommand" }, "unsupported check"},
		{"inverted Kubernetes range", func(s *Scenario) { s.Kubernetes.Min = "1.36" }, "newer than"},
		{"unscoped namespace", func(s *Scenario) { s.Namespace = "default" }, "kubecrypt-*"},
		{"challenge without start state", func(s *Scenario) { s.Setup.Manifests = nil; s.Reset.Manifests = nil }, "must contain at least one manifest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenario := validScenario()
			tt.mutate(scenario)
			err := Validate(scenario)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateAllowsChallengeNamespaceWithoutManifests(t *testing.T) {
	scenario := validScenario()
	scenario.Setup.Manifests = nil
	scenario.Reset.Manifests = nil
	scenario.Namespace = "kubecrypt-foundations"
	if err := Validate(scenario); err != nil {
		t.Fatalf("Validate(): %v", err)
	}
}

func validScenario() *Scenario {
	replicas := 1
	return &Scenario{
		APIVersion:  APIVersionV1Alpha1,
		ID:          "test-scenario",
		Mode:        "challenge",
		Title:       "Test Scenario",
		Description: "Restore the workload.",
		Revision:    "2026-09",
		Module:      "workloads",
		Difficulty:  1,
		Kubernetes:  KubernetesCompatibility{Min: "1.35", Max: "1.35"},
		Tracks:      []string{"beginner", "cka", "ckad"},
		Objective:   "Make the workload available.",
		Concepts:    []string{"pods"},
		Setup:       ResourceSet{Manifests: []string{"workload.yaml"}},
		Checks: []Check{{
			Type: CheckDeploymentAvailable, Namespace: "kubecrypt-test", Name: "test", Replicas: &replicas,
		}},
		Hints:      []string{"conceptual", "procedural", "explicit"},
		Completion: "The workload is available.",
		Debrief:    Debrief{Explanation: "The Deployment reconciled its Pods."},
		Reset:      ResourceSet{Manifests: []string{"workload.yaml"}},
	}
}

func validCatalog() Catalog {
	return Catalog{
		APIVersion: CatalogAPIVersionV1Alpha1,
		Name:       "test-pack",
		Title:      "Test Pack",
		Revision:   "2026-09",
		Modules: []Module{{
			ID: "workloads", Title: "Workloads",
			Scenarios: []ScenarioRef{{ID: "test-scenario", Path: "02-workloads/01-test-scenario.yaml"}},
		}},
	}
}

func packFS(scenarioData []byte) fstest.MapFS {
	catalogData, _ := yaml.Marshal(validCatalog())
	return fstest.MapFS{
		"catalog.yaml":                       {Data: catalogData},
		"02-workloads/01-test-scenario.yaml": {Data: scenarioData},
		"02-workloads/workload.yaml": {Data: []byte(`
apiVersion: v1
kind: Namespace
metadata:
  name: kubecrypt-test
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
  namespace: kubecrypt-test
`)},
	}
}

func scenarioFS(scenarioData []byte) fstest.MapFS {
	return fstest.MapFS{
		"scenario.yaml": {Data: scenarioData},
		"workload.yaml": {Data: []byte(`
apiVersion: v1
kind: Namespace
metadata:
  name: kubecrypt-test
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
  namespace: kubecrypt-test
`)},
	}
}
