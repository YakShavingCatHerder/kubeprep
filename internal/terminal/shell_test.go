package terminal

import (
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
