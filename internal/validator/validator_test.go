package validator

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPollConvergesToSuccess(t *testing.T) {
	calls := 0
	check := CheckFunc(func(context.Context, Runner) (Result, error) {
		calls++
		if calls < 3 {
			return Result{Status: Converging, Message: "still reconciling"}, nil
		}
		return Result{Status: Success, Message: "ready"}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := Poll(ctx, &fakeRunner{}, check, time.Millisecond)
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}
	assertStatus(t, result, Success)
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
}

func TestPollReturnsWrongImmediately(t *testing.T) {
	calls := 0
	check := CheckFunc(func(context.Context, Runner) (Result, error) {
		calls++
		return Result{Status: Wrong, Message: "definitely broken"}, nil
	})

	result, err := Poll(context.Background(), &fakeRunner{}, check, time.Hour)
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}
	assertStatus(t, result, Wrong)
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestPollTimeoutReturnsLastResult(t *testing.T) {
	check := staticCheck(Result{Status: Converging, Message: "one pod is pending"})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	result, err := Poll(ctx, &fakeRunner{}, check, time.Hour)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Poll() error = %v, want DeadlineExceeded", err)
	}
	assertStatus(t, result, Converging)
	if result.Message != "one pod is pending" {
		t.Fatalf("message = %q", result.Message)
	}
}

func TestPollCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	check := CheckFunc(func(context.Context, Runner) (Result, error) {
		calls++
		cancel()
		return Result{Status: Converging, Message: "waiting"}, nil
	})

	result, err := Poll(ctx, &fakeRunner{}, check, time.Hour)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Poll() error = %v, want Canceled", err)
	}
	assertStatus(t, result, Converging)
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestPollRejectsInvalidInputs(t *testing.T) {
	validCheck := staticCheck(Result{Status: Success})
	tests := []struct {
		name     string
		runner   Runner
		check    Check
		interval time.Duration
	}{
		{name: "nil runner", check: validCheck, interval: time.Second},
		{name: "nil check", runner: &fakeRunner{}, interval: time.Second},
		{name: "invalid interval", runner: &fakeRunner{}, check: validCheck},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Poll(context.Background(), tt.runner, tt.check, tt.interval)
			if err == nil {
				t.Fatal("Poll() error = nil")
			}
			assertStatus(t, result, Wrong)
		})
	}
}

func TestKubeconfigEnvOverridesInheritedValue(t *testing.T) {
	got := kubeconfigEnv([]string{"PATH=/bin", "KUBECONFIG=/unsafe/config", "HOME=/tmp"}, "/game/config")
	want := []string{"PATH=/bin", "HOME=/tmp", "KUBECONFIG=/game/config"}
	if len(got) != len(want) {
		t.Fatalf("environment = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("environment = %#v, want %#v", got, want)
		}
	}
}

func TestKubectlRunnerRequiresExplicitKubeconfig(t *testing.T) {
	_, err := (KubectlRunner{}).Run(context.Background(), "get", "pods")
	if err == nil {
		t.Fatal("Run() error = nil")
	}
	assertContains(t, err.Error(), "explicit kubeconfig")
}
