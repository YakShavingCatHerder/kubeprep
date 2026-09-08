package cluster

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestDoctorChecksExpectedCommandsAndWritableDirectory(t *testing.T) {
	paths := PathsForDirectory(t.TempDir() + "/config")
	runner := &fakeRunner{run: func(command Command) (Result, error) {
		switch {
		case command.Name == "docker" && slices.Equal(command.Args, []string{"info"}):
			return Result{Stdout: "Docker healthy\n"}, nil
		default:
			return Result{}, errors.New("unexpected command")
		}
	}}

	results := (&Doctor{Runner: runner, Paths: paths, GOOS: "linux"}).Check(context.Background())
	if len(results) != 5 {
		t.Fatalf("Check() returned %d results, want 5", len(results))
	}
	for _, result := range results {
		switch result.Name {
		case "kind", "kubectl":
			if result.OK {
				t.Errorf("%s passed without a managed binary: %#v", result.Name, result)
			}
		default:
			if !result.OK {
				t.Errorf("%s failed: %s", result.Name, result.Detail)
			}
		}
	}
}

func TestDoctorReportsActionableCommandFailure(t *testing.T) {
	paths := PathsForDirectory(t.TempDir())
	runner := &fakeRunner{run: func(command Command) (Result, error) {
		if command.Name == "docker" {
			return Result{Stderr: "Cannot connect to daemon\n"}, errors.New("exit status 1")
		}
		return Result{Stdout: "ok"}, nil
	}}

	results := (&Doctor{Runner: runner, Paths: paths, GOOS: "darwin"}).Check(context.Background())
	docker := results[1]
	if docker.OK {
		t.Fatal("Docker check unexpectedly succeeded")
	}
	if docker.Detail == "" || docker.Remediation == "" {
		t.Fatalf("Docker failure is not actionable: %#v", docker)
	}
}

func TestDoctorReportsPendingToolInstall(t *testing.T) {
	paths := PathsForDirectory(t.TempDir() + "/config")
	runner := &fakeRunner{run: func(command Command) (Result, error) {
		if command.Name == "docker" {
			return Result{Stdout: "ok"}, nil
		}
		return Result{}, errors.New("unexpected command")
	}}
	results := (&Doctor{Runner: runner, Paths: paths, GOOS: "linux"}).Check(context.Background())
	kind := results[2]
	if kind.OK || !strings.Contains(kind.Detail, KindVersion) || !strings.Contains(kind.Remediation, "kubeprep doctor") {
		t.Fatalf("kind check = %#v", kind)
	}
}
