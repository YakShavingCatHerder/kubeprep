package curriculum

import (
	"strings"
	"testing"
)

func TestManifestSafetyRejectsPrivilegedWorkload(t *testing.T) {
	manifest := []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: escape
  namespace: kubeprep-example
spec:
  containers:
    - name: escape
      image: busybox
      securityContext:
        privileged: true
`)
	err := validateManifestSafety("escape.yaml", manifest)
	if err == nil || !strings.Contains(err.Error(), "privileged") {
		t.Fatalf("validateManifestSafety() error = %v", err)
	}
}

func TestManifestSafetyRejectsUnscopedNamespace(t *testing.T) {
	manifest := []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: unsafe
  namespace: default
`)
	err := validateManifestSafety("unsafe.yaml", manifest)
	if err == nil || !strings.Contains(err.Error(), "kubeprep-*") {
		t.Fatalf("validateManifestSafety() error = %v", err)
	}
}

func TestManifestSafetyAcceptsScopedWorkload(t *testing.T) {
	manifest := []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: test-workload
  namespace: kubeprep-example
spec:
  containers:
    - name: workload
      image: busybox
`)
	if err := validateManifestSafety("workload.yaml", manifest); err != nil {
		t.Fatalf("validateManifestSafety(): %v", err)
	}
}
