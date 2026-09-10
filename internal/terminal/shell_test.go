package terminal

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestScopedEnvironmentReplacesSensitiveValues(t *testing.T) {
	got := scopedEnvironment(
		[]string{"PATH=/bin", "KUBECONFIG=/unsafe", "KUBEPREP_SCENARIO=old", "TERM=dumb"},
		map[string]string{
			"KUBECONFIG":        "/safe/config",
			"KUBEPREP_SCENARIO": "pod-creation",
			"TERM":              "xterm-256color",
		},
	)

	if slices.Contains(got, "KUBECONFIG=/unsafe") {
		t.Fatal("unsafe kubeconfig remained in child environment")
	}
	if slices.Contains(got, "TERM=dumb") {
		t.Fatal("previous TERM remained in child environment")
	}
	for _, want := range []string{
		"PATH=/bin",
		"KUBECONFIG=/safe/config",
		"KUBEPREP_SCENARIO=pod-creation",
		"TERM=xterm-256color",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("environment missing %q: %s", want, strings.Join(got, ", "))
		}
	}
}

func TestScopedEnvironmentPrependsToolBinDir(t *testing.T) {
	runner := &ShellRunner{
		LookupEnv: func(key string) (string, bool) {
			if key == "PATH" {
				return "/usr/bin:/bin", true
			}
			if key == "SHELL" {
				return "/bin/zsh", true
			}
			return "", false
		},
		Environ:    func() []string { return []string{"PATH=/usr/bin:/bin"} },
		Executable: func() (string, error) { return "/usr/local/bin/kubeprep", nil },
	}
	cmd, err := runner.Command(context.Background(), ShellSession{
		Kubeconfig: "/tmp/kubeconfig",
		ToolBinDir: "/tmp/kubeprep/bin",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "PATH=/tmp/kubeprep/bin:/usr/bin:/bin"
	if !slices.Contains(cmd.Env, want) {
		t.Fatalf("PATH override missing %q in %v", want, cmd.Env)
	}
}

func TestLabShellStartsWithoutABanner(t *testing.T) {
	runner := &ShellRunner{
		LookupEnv: func(key string) (string, bool) {
			if key == "SHELL" {
				return "/bin/zsh", true
			}
			return "", false
		},
		Environ:    func() []string { return nil },
		Executable: func() (string, error) { return "/usr/local/bin/kubeprep", nil },
	}
	cmd, err := runner.Command(context.Background(), ShellSession{ScenarioID: "kubectl-basics"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Path != "/bin/zsh" && (len(cmd.Args) == 0 || cmd.Args[0] != "/bin/zsh") {
		t.Fatalf("lab shell should start the user shell, got path=%q args=%v", cmd.Path, cmd.Args)
	}
	joined := strings.Join(cmd.Args, " ")
	if strings.Contains(joined, "printf") || strings.Contains(joined, "Isolated kubeconfig") {
		t.Fatalf("lab shell still prints a banner: %v", cmd.Args)
	}
}
