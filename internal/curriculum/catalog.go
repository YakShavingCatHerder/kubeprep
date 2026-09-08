package curriculum

import (
	"fmt"
	"path"
	"strings"
)

const CatalogAPIVersionV1Alpha1 = "kubecrypt.io/catalog/v1alpha1"

// Catalog defines one scenario pack as learner paths of sections of labs.
// Play order is the nested list. A lab id may appear on more than one path;
// it is one file, under its section directory.
type Catalog struct {
	APIVersion string `yaml:"apiVersion"`
	Name       string `yaml:"name"`
	Title      string `yaml:"title"`
	Revision   string `yaml:"revision"`
	Paths      []Path `yaml:"paths"`
}

// Path is a learner playlist (beginner, cka, ckad).
type Path struct {
	ID       string    `yaml:"id"`
	Title    string    `yaml:"title"`
	Sections []Section `yaml:"sections"`
}

// Section is a domain grouping (pods, rbac). Lab files live in this directory.
type Section struct {
	ID   string   `yaml:"id"`
	Labs []string `yaml:"labs"`
}

// LabRef is a lab id and the file path derived from its section.
type LabRef struct {
	ID      string
	Section string
	Path    string
}

// LabFilePath is {section}/{id}.yaml.
func LabFilePath(section, id string) string {
	return path.Join(section, id+".yaml")
}

// UniqueLabs returns each lab once, in first-seen catalog order.
func (c Catalog) UniqueLabs() []LabRef {
	var refs []LabRef
	seen := make(map[string]struct{})
	for _, learningPath := range c.Paths {
		for _, section := range learningPath.Sections {
			for _, id := range section.Labs {
				if _, exists := seen[id]; exists {
					continue
				}
				seen[id] = struct{}{}
				refs = append(refs, LabRef{ID: id, Section: section.ID, Path: LabFilePath(section.ID, id)})
			}
		}
	}
	return refs
}

func (c Catalog) ScenarioIDs() []string {
	refs := c.UniqueLabs()
	ids := make([]string, len(refs))
	for i, ref := range refs {
		ids[i] = ref.ID
	}
	return ids
}

// PathByID returns the named learner path, or nil.
func (c Catalog) PathByID(id string) *Path {
	for i := range c.Paths {
		if c.Paths[i].ID == id {
			return &c.Paths[i]
		}
	}
	return nil
}

// LabIDsOnPath is play order for one learner path.
func (c Catalog) LabIDsOnPath(pathID string) []string {
	learningPath := c.PathByID(pathID)
	if learningPath == nil {
		return nil
	}
	var ids []string
	for _, section := range learningPath.Sections {
		ids = append(ids, section.Labs...)
	}
	return ids
}

func validateCatalog(catalog *Catalog) error {
	if catalog == nil {
		return fmt.Errorf("catalog is nil")
	}
	if catalog.APIVersion != CatalogAPIVersionV1Alpha1 {
		return fmt.Errorf("apiVersion: unsupported value %q (want %q)", catalog.APIVersion, CatalogAPIVersionV1Alpha1)
	}
	if !idPattern.MatchString(catalog.Name) {
		return fmt.Errorf("name: %q must be lowercase kebab-case", catalog.Name)
	}
	if strings.TrimSpace(catalog.Title) == "" {
		return fmt.Errorf("title: must not be empty")
	}
	if !revisionPattern.MatchString(catalog.Revision) {
		return fmt.Errorf("revision: %q must use YYYY-MM format", catalog.Revision)
	}
	if len(catalog.Paths) == 0 {
		return fmt.Errorf("paths: must contain at least one path")
	}
	seenPaths := make(map[string]struct{})
	labSection := make(map[string]string)
	for pathIndex, learningPath := range catalog.Paths {
		if !idPattern.MatchString(learningPath.ID) {
			return fmt.Errorf("paths[%d].id: %q must be lowercase kebab-case", pathIndex, learningPath.ID)
		}
		if _, exists := seenPaths[learningPath.ID]; exists {
			return fmt.Errorf("paths[%d].id: duplicate path %q", pathIndex, learningPath.ID)
		}
		seenPaths[learningPath.ID] = struct{}{}
		if strings.TrimSpace(learningPath.Title) == "" {
			return fmt.Errorf("paths[%d].title: must not be empty", pathIndex)
		}
		if len(learningPath.Sections) == 0 {
			return fmt.Errorf("paths[%d].sections: must contain at least one section", pathIndex)
		}
		seenSections := make(map[string]struct{})
		seenOnPath := make(map[string]struct{})
		for sectionIndex, section := range learningPath.Sections {
			if !idPattern.MatchString(section.ID) {
				return fmt.Errorf("paths[%d].sections[%d].id: %q must be lowercase kebab-case", pathIndex, sectionIndex, section.ID)
			}
			if _, exists := seenSections[section.ID]; exists {
				return fmt.Errorf("paths[%d].sections[%d].id: duplicate section %q", pathIndex, sectionIndex, section.ID)
			}
			seenSections[section.ID] = struct{}{}
			if len(section.Labs) == 0 {
				return fmt.Errorf("paths[%d].sections[%d].labs: must contain at least one lab", pathIndex, sectionIndex)
			}
			for labIndex, id := range section.Labs {
				if !idPattern.MatchString(id) {
					return fmt.Errorf("paths[%d].sections[%d].labs[%d]: %q must be lowercase kebab-case", pathIndex, sectionIndex, labIndex, id)
				}
				if _, exists := seenOnPath[id]; exists {
					return fmt.Errorf("paths[%d].sections[%d].labs[%d]: duplicate lab %q on path %q", pathIndex, sectionIndex, labIndex, id, learningPath.ID)
				}
				seenOnPath[id] = struct{}{}
				if previous, exists := labSection[id]; exists && previous != section.ID {
					return fmt.Errorf("paths[%d].sections[%d].labs[%d]: lab %q is already in section %q", pathIndex, sectionIndex, labIndex, id, previous)
				}
				labSection[id] = section.ID
			}
		}
	}
	return nil
}
