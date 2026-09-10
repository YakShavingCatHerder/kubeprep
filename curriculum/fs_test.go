package corepack

import (
	"path"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFilesContainCatalogLabs(t *testing.T) {
	data, err := Files.ReadFile("catalog.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Paths []struct {
			Sections []struct {
				ID   string   `yaml:"id"`
				Labs []string `yaml:"labs"`
			} `yaml:"sections"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("catalog.yaml: %v", err)
	}
	seen := make(map[string]struct{})
	for _, learningPath := range catalog.Paths {
		for _, section := range learningPath.Sections {
			for _, id := range section.Labs {
				name := path.Join(section.ID, id+".yaml")
				if _, exists := seen[name]; exists {
					continue
				}
				seen[name] = struct{}{}
				if _, err := Files.ReadFile(name); err != nil {
					t.Fatalf("Files.ReadFile(%q): %v", name, err)
				}
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("catalog.yaml lists no labs")
	}
}
