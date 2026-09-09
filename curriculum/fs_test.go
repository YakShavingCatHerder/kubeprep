package corepack

import "testing"

func TestFilesContainCorePack(t *testing.T) {
	for _, name := range []string{"catalog.yaml", "welcome/kubectl-basics.yaml", "welcome/pod-creation.yaml"} {
		if _, err := Files.ReadFile(name); err != nil {
			t.Fatalf("Files.ReadFile(%q): %v", name, err)
		}
	}
}
