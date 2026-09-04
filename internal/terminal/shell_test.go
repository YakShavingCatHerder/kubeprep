package terminal

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestScopedEnvironmentReplacesSensitiveValues(t *testing.T) {
	got := scopedEnvironment(
		[]string{"PATH=/bin", "KUBECONFIG=/unsafe", "KUBECRYPT_SCENARIO=old", "TERM=dumb"},
		map[string]string{
			"KUBECONFIG":         "/safe/config",
			"KUBECRYPT_SCENARIO": "cluster-components",
			"KUBECRYPT_PACKS":    "/packs/one:/packs/two",
			"TERM":               "xterm-256color",
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
		"KUBECRYPT_SCENARIO=cluster-components",
		"KUBECRYPT_PACKS=/packs/one:/packs/two",
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
		Executable: func() (string, error) { return "/usr/local/bin/kubecrypt", nil },
	}
	cmd, err := runner.Command(context.Background(), ShellSession{
		Kubeconfig: "/tmp/kubeconfig",
		ToolBinDir: "/tmp/kubecrypt/bin",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "PATH=/tmp/kubecrypt/bin:/usr/bin:/bin"
	if !slices.Contains(cmd.Env, want) {
		t.Fatalf("PATH override missing %q in %v", want, cmd.Env)
	}
}
