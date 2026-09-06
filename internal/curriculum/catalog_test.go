package curriculum

import (
	"strings"
	"testing"
)

func TestValidateCatalogRejectsInvalidNesting(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Catalog)
		want   string
	}{
		{
			name:   "empty paths",
			mutate: func(c *Catalog) { c.Paths = nil },
			want:   "at least one path",
		},
		{
			name:   "duplicate path id",
			mutate: func(c *Catalog) { c.Paths = append(c.Paths, c.Paths[0]) },
			want:   "duplicate path",
		},
		{
			name: "lab listed twice on one path",
			mutate: func(c *Catalog) {
				c.Paths[0].Sections[0].Labs = []string{"test-scenario", "test-scenario"}
			},
			want: "duplicate lab",
		},
		{
			name: "same lab in two sections",
			mutate: func(c *Catalog) {
				c.Paths = append(c.Paths, Path{
					ID: "cka", Title: "CKA",
					Sections: []Section{{ID: "other", Labs: []string{"test-scenario"}}},
				})
			},
			want: "already in section",
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

func TestLabFilePath(t *testing.T) {
	if got := LabFilePath("pods", "pod-creation"); got != "pods/pod-creation.yaml" {
		t.Fatalf("LabFilePath() = %q", got)
	}
}

func TestCatalogLabIDsOnPathAllowsReuse(t *testing.T) {
	catalog := validCatalog()
	catalog.Paths = append(catalog.Paths, Path{
		ID: "cka", Title: "CKA",
		Sections: []Section{{ID: "workloads", Labs: []string{"test-scenario"}}},
	})
	if err := validateCatalog(&catalog); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(catalog.LabIDsOnPath("cka"), ","); got != "test-scenario" {
		t.Fatalf("cka labs = %s", got)
	}
	if got := strings.Join(catalog.ScenarioIDs(), ","); got != "test-scenario" {
		t.Fatalf("unique labs = %s", got)
	}
}
