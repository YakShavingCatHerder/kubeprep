package curriculum

import (
	"strings"
	"testing"
)

func TestValidateCatalogRequiresNumberedLabPaths(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Catalog)
		want   string
	}{
		{
			name:   "unnumbered path",
			mutate: func(c *Catalog) { c.Modules[0].Scenarios[0].Path = "02-workloads/test-scenario.yaml" },
			want:   "{slot}/{nn}-{id}.yaml",
		},
		{
			name:   "lab number skips first",
			mutate: func(c *Catalog) { c.Modules[0].Scenarios[0].Path = "02-workloads/02-test-scenario.yaml" },
			want:   "lab number 02 must be 01",
		},
		{
			name:   "path id mismatches catalog id",
			mutate: func(c *Catalog) { c.Modules[0].Scenarios[0].Path = "02-workloads/01-other-lab.yaml" },
			want:   "file id",
		},
		{
			name:   "path module mismatches catalog module",
			mutate: func(c *Catalog) { c.Modules[0].Scenarios[0].Path = "02-foundations/01-test-scenario.yaml" },
			want:   "directory module",
		},
		{
			name: "modules listed out of slot order",
			mutate: func(c *Catalog) {
				c.Modules = []Module{
					{ID: "workloads", Title: "Workloads", Scenarios: []ScenarioRef{
						{ID: "later-lab", Path: "02-workloads/01-later-lab.yaml"},
					}},
					{ID: "foundations", Title: "Foundations", Scenarios: []ScenarioRef{
						{ID: "pod-creation", Path: "01-foundations/01-pod-creation.yaml"},
					}},
				}
			},
			want: "must come after slot 02",
		},
		{
			name: "second lab not numbered 02",
			mutate: func(c *Catalog) {
				c.Modules[0].Scenarios = []ScenarioRef{
					{ID: "test-scenario", Path: "02-workloads/01-test-scenario.yaml"},
					{ID: "later-lab", Path: "02-workloads/03-later-lab.yaml"},
				}
			},
			want: "lab number 03 must be 02",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := validCatalog()
			tt.mutate(&catalog)
			err := validateCatalog(&catalog)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validateCatalog() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestParseNumberedScenarioPath(t *testing.T) {
	got, err := parseNumberedScenarioPath("01-foundations/01-pod-creation.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if got.Slot != 1 || got.ModuleID != "foundations" || got.Index != 1 || got.ID != "pod-creation" {
		t.Fatalf("parsed path = %#v", got)
	}
}
