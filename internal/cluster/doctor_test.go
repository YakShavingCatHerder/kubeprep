package cluster

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestDoctorChecksExpectedCommandsAndWritableDirectory(t *testing.T) {
	paths := PathsForDirectory(t.TempDir() + "/config")
	runner := &fakeRunner{run: func(command Command) (Result, error) {
		switch {
		case command.Name == "docker" && slices.Equal(command.Args, []string{"info"}):
			return Result{Stdout: "Docker healthy\n"}, nil
		case command.Name == "kind" && slices.Equal(command.Args, []string{"version"}):
			return Result{Stdout: "kind v1"}, nil
		case command.Name == "kubectl" && slices.Equal(command.Args, []string{"version", "--client"}):
			return Result{Stdout: "Client Version: v1.35.0"}, nil
		default:
			return Result{}, errors.New("unexpected command")
		}
	}}

	results := (&Doctor{Runner: runner, Paths: paths, GOOS: "linux"}).Check(context.Background())
	if len(results) != 5 {
		t.Fatalf("Check() returned %d results, want 5", len(results))
	}
	for _, result := range results {
		if !result.OK {
			t.Errorf("%s failed: %s", result.Name, result.Detail)
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
