//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/YakShavingCatHerder/kubecrypt/internal/cluster"
	"github.com/YakShavingCatHerder/kubecrypt/internal/curriculum"
	"github.com/YakShavingCatHerder/kubecrypt/internal/validator"
)

func TestOpeningOrientationObservesRealCluster(t *testing.T) {
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
	for _, id := range []string{"shell-orientation", "cluster-components"} {
		scenario, err := registry.LoadScenario(id)
		if err != nil {
			t.Fatal(err)
		}
		if scenario.Mode != "orientation" || scenario.Namespace != "" || len(scenario.Setup.Manifests) != 0 || len(scenario.Reset.Manifests) != 0 {
			t.Fatalf("%s must be a resource-free orientation", id)
		}
	}

	runner := validator.KubectlRunner{Kubeconfig: manager.Paths().Kubeconfig}
	waitForSuccess(t, ctx, runner, openingCheck(), 2*time.Minute)

	if err := manager.Destroy(ctx); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	if err := manager.Destroy(ctx); err != nil {
		t.Fatalf("idempotent destroy: %v", err)
	}
}

func openingCheck() validator.Check {
	return validator.All(
		validator.NodeTopology{Count: 3, ControlPlanes: 1, Workers: 2},
		validator.ObjectExists{Kind: "namespace", Name: "kube-system"},
		validator.PodReady{Namespace: "kube-system", Selector: "component=kube-apiserver", MinReady: 1},
		validator.PodReady{Namespace: "kube-system", Selector: "component=etcd", MinReady: 1},
		validator.PodReady{Namespace: "kube-system", Selector: "component=kube-scheduler", MinReady: 1},
		validator.PodReady{Namespace: "kube-system", Selector: "component=kube-controller-manager", MinReady: 1},
		validator.PodReady{Namespace: "kube-system", Selector: "k8s-app=kube-dns", MinReady: 2},
		validator.PodReady{Namespace: "kube-system", Selector: "k8s-app=kube-proxy", MinReady: 3},
		validator.PodReady{Namespace: "kube-system", Selector: "app=kindnet", MinReady: 3},
	)
}

func waitForSuccess(t *testing.T, parent context.Context, runner validator.Runner, check validator.Check, timeout time.Duration) {
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
			if result.Status == validator.Success {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for opening components: %v; last result: %s (%s)", ctx.Err(), last.Status, last.Message)
		case <-ticker.C:
		}
	}
}
