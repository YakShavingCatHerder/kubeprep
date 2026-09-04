package curriculum

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
)

const CatalogAPIVersionV1Alpha1 = "kubecrypt.io/catalog/v1alpha1"

// Catalog defines one scenario pack and its authored module order.
type Catalog struct {
	APIVersion string   `yaml:"apiVersion"`
	Name       string   `yaml:"name"`
	Title      string   `yaml:"title"`
	Revision   string   `yaml:"revision"`
	Modules    []Module `yaml:"modules"`
}

type Module struct {
	ID        string        `yaml:"id"`
	Title     string        `yaml:"title"`
	Scenarios []ScenarioRef `yaml:"scenarios"`
}

type ScenarioRef struct {
	ID   string `yaml:"id"`
	Path string `yaml:"path"`
}

// ScenarioRefs returns every scenario reference in authored order.
func (c Catalog) ScenarioRefs() []ScenarioRef {
	var refs []ScenarioRef
	for _, module := range c.Modules {
		refs = append(refs, module.Scenarios...)
	}
	return refs
}

func (c Catalog) ScenarioIDs() []string {
	var ids []string
	for _, ref := range c.ScenarioRefs() {
		ids = append(ids, ref.ID)
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
	if len(catalog.Modules) == 0 {
		return fmt.Errorf("modules: must contain at least one module")
	}
	seenModules := make(map[string]struct{})
	seenScenarios := make(map[string]struct{})
	for moduleIndex, module := range catalog.Modules {
		if !idPattern.MatchString(module.ID) {
			return fmt.Errorf("modules[%d].id: %q must be lowercase kebab-case", moduleIndex, module.ID)
		}
		if _, exists := seenModules[module.ID]; exists {
			return fmt.Errorf("modules[%d].id: duplicate module %q", moduleIndex, module.ID)
		}
		seenModules[module.ID] = struct{}{}
		if strings.TrimSpace(module.Title) == "" {
			return fmt.Errorf("modules[%d].title: must not be empty", moduleIndex)
		}
		if len(module.Scenarios) == 0 {
			return fmt.Errorf("modules[%d].scenarios: must contain at least one scenario", moduleIndex)
		}
		for scenarioIndex, ref := range module.Scenarios {
			if !idPattern.MatchString(ref.ID) {
				return fmt.Errorf("modules[%d].scenarios[%d].id: %q must be lowercase kebab-case", moduleIndex, scenarioIndex, ref.ID)
			}
			if _, exists := seenScenarios[ref.ID]; exists {
				return fmt.Errorf("modules[%d].scenarios[%d].id: duplicate scenario %q", moduleIndex, scenarioIndex, ref.ID)
			}
			if !fs.ValidPath(ref.Path) || path.Clean(ref.Path) != ref.Path {
				return fmt.Errorf("modules[%d].scenarios[%d].path: %q must be a clean relative path", moduleIndex, scenarioIndex, ref.Path)
			}
			ext := path.Ext(ref.Path)
			if ext != ".yaml" && ext != ".yml" {
				return fmt.Errorf("modules[%d].scenarios[%d].path: %q must reference YAML", moduleIndex, scenarioIndex, ref.Path)
			}
			seenScenarios[ref.ID] = struct{}{}
		}
	}
	return nil
}
