package corepack

import "testing"

func TestFilesContainCorePack(t *testing.T) {
	for _, name := range []string{"catalog.yaml", "pods/pod-creation.yaml"} {
		if _, err := Files.ReadFile(name); err != nil {
			t.Fatalf("Files.ReadFile(%q): %v", name, err)
		}
	}
}
