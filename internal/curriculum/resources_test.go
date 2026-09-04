package curriculum

import (
	"bytes"
	"strings"
	"testing"
)

func TestComposeResourcesPrependsDeclaredNamespace(t *testing.T) {
	scenario := &Scenario{Namespace: "kubecrypt-foundations"}
	got, err := scenario.ComposeResources(ResourceSet{Manifests: []string{"workload.yaml"}}, func(reference string) ([]byte, error) {
		if reference != "workload.yaml" {
			t.Fatalf("read %q", reference)
		}
		return []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: record\n  namespace: kubecrypt-foundations\n"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(got, NamespaceDocument("kubecrypt-foundations")) {
		t.Fatalf("namespace document is not first:\n%s", got)
	}
	if !bytes.Contains(got, []byte("kind: ConfigMap")) {
		t.Fatalf("missing setup manifest:\n%s", got)
	}
}

func TestComposeResourcesNamespaceOnly(t *testing.T) {
	scenario := &Scenario{Namespace: "kubecrypt-foundations"}
	got, err := scenario.ComposeResources(ResourceSet{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(got)); got != strings.TrimSpace(string(NamespaceDocument("kubecrypt-foundations"))) {
		t.Fatalf("got %q", got)
	}
}

func TestComposeResourcesEmptyWithoutNamespace(t *testing.T) {
	scenario := &Scenario{}
	got, err := scenario.ComposeResources(ResourceSet{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("got %q", got)
	}
}
