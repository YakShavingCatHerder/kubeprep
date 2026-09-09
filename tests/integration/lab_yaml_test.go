//go:build integration

package integration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/YakShavingCatHerder/kubeprep/internal/cluster"
	"github.com/YakShavingCatHerder/kubeprep/internal/curriculum"
	"github.com/YakShavingCatHerder/kubeprep/internal/validator"
)

// Fixture lab YAML. This is not a shipped curriculum file. It exists so the
// kind job can prove the runner honors authored keys: namespace,
// startingClusterConfiguration, checks, and reset.
const fixtureLab = `apiVersion: kubeprep.io/v1alpha1
id: fixture-start-state
mode: challenge
title: Fixture Start State
description: Engine fixture for kind integration.
revision: 2026-09
module: fixtures
namespace: kubeprep-fixture
difficulty: 1
kubernetes:
  min: "1.35"
  max: "1.35"
tracks:
  - beginner
objective: Admit the record.
concepts:
  - configmaps
startingClusterConfiguration: |
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: record
    namespace: kubeprep-fixture
  data:
    status: pending
setup:
  manifests: []
checks:
  - type: objectExists
    kind: configmap
    namespace: kubeprep-fixture
    name: record
  - type: fieldEquals
    kind: configmap
    namespace: kubeprep-fixture
    name: record
    field: data.status
    value: admitted
hints:
  - conceptual
  - procedural
  - explicit
completion: The record is admitted.
debrief:
  explanation: The API server stored the ConfigMap.
reset:
  manifests: []
`

const fixtureTarget = `apiVersion: v1
kind: ConfigMap
metadata:
  name: record
  namespace: kubeprep-fixture
data:
  status: admitted
`

func TestLabYAMLStartChecksAndReset(t *testing.T) {
	if os.Getenv("KUBEPREP_INTEGRATION") != "1" {
		t.Skip("set KUBEPREP_INTEGRATION=1 to create a disposable kind cluster")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	configDir := t.TempDir()
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "unrelated-kubeconfig"))
	manager := cluster.NewManagerWithPaths(cluster.ExecRunner{}, cluster.PathsForDirectory(configDir))
	if _, err := manager.EnsureCluster(ctx); err != nil {
		t.Fatalf("check in: %v", err)
	}
	t.Cleanup(func() {
		cleanup, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()
		_ = manager.Destroy(cleanup)
	})

	filename := filepath.Join(t.TempDir(), "fixture-start-state.yaml")
	if err := os.WriteFile(filename, []byte(fixtureLab), 0o600); err != nil {
		t.Fatal(err)
	}
	scenario, err := curriculum.LoadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if scenario.Namespace == "" || scenario.StartingClusterConfiguration == "" || len(scenario.Checks) == 0 {
		t.Fatalf("fixture missing required keys: %#v", scenario)
	}

	setup, err := scenario.ComposeResources(scenario.Setup, nil)
	if err != nil {
		t.Fatal(err)
	}
	reset, err := scenario.ComposeResources(scenario.Reset, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(setup, []byte(scenario.Namespace)) {
		t.Fatalf("setup missing declared namespace %q: %s", scenario.Namespace, setup)
	}
	if !bytes.Contains(setup, []byte("status: pending")) {
		t.Fatalf("setup missing startingClusterConfiguration:\n%s", setup)
	}
	if err := manager.Apply(ctx, setup); err != nil {
		t.Fatalf("setup: %v", err)
	}

	runner := validator.KubectlRunner{
		Kubeconfig: manager.Paths().Kubeconfig,
		Executable: manager.Paths().KubectlExecutable(),
	}
	check := checksFromScenario(t, scenario)

	result, err := check.Evaluate(ctx, runner)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != validator.Wrong {
		t.Fatalf("initial state = %s (%s), want wrong", result.Status, result.Message)
	}

	if err := manager.Apply(ctx, []byte(fixtureTarget)); err != nil {
		t.Fatalf("apply target: %v", err)
	}
	waitForStatus(t, ctx, runner, check, validator.Success, 30*time.Second)

	if err := manager.Reset(ctx, reset); err != nil {
		t.Fatalf("reset: %v", err)
	}
	waitForStatus(t, ctx, runner, check, validator.Wrong, 2*time.Minute)
}

func checksFromScenario(t *testing.T, scenario *curriculum.Scenario) validator.Check {
	t.Helper()
	checks := make([]validator.Check, 0, len(scenario.Checks))
	for _, authored := range scenario.Checks {
		switch authored.Type {
		case curriculum.CheckObjectExists:
			checks = append(checks, validator.ObjectExists{
				Kind: authored.Kind, Namespace: authored.Namespace, Name: authored.Name,
			})
		case curriculum.CheckFieldEquals:
			checks = append(checks, validator.FieldEquals{
				Kind: authored.Kind, Namespace: authored.Namespace, Name: authored.Name,
				Field: authored.Field, Value: authored.Value,
			})
		default:
			t.Fatalf("fixture used untested check type %q", authored.Type)
		}
	}
	return validator.All(checks...)
}

func waitForStatus(t *testing.T, parent context.Context, runner validator.Runner, check validator.Check, want validator.Status, timeout time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var last validator.Result
	for {
		result, err := check.Evaluate(ctx, runner)
		if err == nil {
			last = result
			if result.Status == want {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for %s: %v; last result: %s (%s)", want, ctx.Err(), last.Status, last.Message)
		case <-ticker.C:
		}
	}
}
