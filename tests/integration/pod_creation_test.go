//go:build integration

package integration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/YakShavingCatHerder/kubecrypt/internal/cluster"
	"github.com/YakShavingCatHerder/kubecrypt/internal/curriculum"
	"github.com/YakShavingCatHerder/kubecrypt/internal/validator"
)

func TestPodCreationGradesStoredAPIObject(t *testing.T) {
	if os.Getenv("KUBECRYPT_INTEGRATION") != "1" {
		t.Skip("set KUBECRYPT_INTEGRATION=1 to create a disposable kind cluster")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	configDir := t.TempDir()
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "unrelated-kubeconfig"))
	manager := cluster.NewManagerWithPaths(cluster.ExecRunner{}, cluster.PathsForDirectory(configDir))
	if _, err := manager.CheckIn(ctx); err != nil {
		t.Fatalf("check in: %v", err)
	}
	t.Cleanup(func() {
		cleanup, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()
		_ = manager.Destroy(cleanup)
	})

	registry, err := curriculum.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := registry.LoadScenario("pod-creation")
	if err != nil {
		t.Fatal(err)
	}

	setup, err := scenario.ComposeResources(scenario.Setup, func(reference string) ([]byte, error) {
		return registry.ReadManifest(scenario.ID, reference)
	})
	if err != nil {
		t.Fatal(err)
	}
	reset, err := scenario.ComposeResources(scenario.Reset, func(reference string) ([]byte, error) {
		return registry.ReadManifest(scenario.ID, reference)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(setup, []byte("kubecrypt-foundations")) {
		t.Fatalf("setup missing declared namespace: %s", setup)
	}
	if err := manager.Apply(ctx, setup); err != nil {
		t.Fatalf("setup: %v", err)
	}

	runner := validator.KubectlRunner{
		Kubeconfig: manager.Paths().Kubeconfig,
		Executable: manager.Paths().KubectlExecutable(),
	}
	check := podCreationCheck()

	result, err := check.Evaluate(ctx, runner)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != validator.Wrong {
		t.Fatalf("initial state = %s (%s), want wrong", result.Status, result.Message)
	}

	if err := manager.Apply(ctx, []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: nginx
  namespace: kubecrypt-foundations
spec:
  containers:
    - name: web
      image: nginx:1.27
`)); err != nil {
		t.Fatalf("apply declarative pod: %v", err)
	}
	waitForStatus(t, ctx, runner, check, validator.Success, 30*time.Second)

	if err := manager.Reset(ctx, reset); err != nil {
		t.Fatalf("reset: %v", err)
	}
	waitForStatus(t, ctx, runner, check, validator.Wrong, 2*time.Minute)

	if _, err := runner.Run(ctx, "run", "nginx", "--image=nginx:1.27", "--namespace", "kubecrypt-foundations"); err != nil {
		t.Fatalf("kubectl run: %v", err)
	}
	waitForStatus(t, ctx, runner, check, validator.Success, 30*time.Second)
}

func podCreationCheck() validator.Check {
	return validator.All(
		validator.ObjectExists{Kind: "pod", Namespace: "kubecrypt-foundations", Name: "nginx"},
		validator.FieldEquals{
			Kind: "pod", Namespace: "kubecrypt-foundations", Name: "nginx",
			Field: "spec.containers[0].image", Value: "nginx:1.27",
		},
	)
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
