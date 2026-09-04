package curriculum

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strconv"
	"strings"
)

const CatalogAPIVersionV1Alpha1 = "kubecrypt.io/catalog/v1alpha1"

// numberedScenarioPath is {slotNN}-{module}/{labNN}-{id}.yaml.
// Lab numbers are the play order inside that module: 01 is the first lab.
var numberedScenarioPath = regexp.MustCompile(`^(\d{2})-([a-z0-9]+(?:-[a-z0-9]+)*)/(\d{2})-([a-z0-9]+(?:-[a-z0-9]+)*)\.(yaml|yml)$`)

type numberedPath struct {
	Slot     int
	ModuleID string
	Index    int
	ID       string
}

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
	previousSlot := -1
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
		moduleSlot := -1
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
			parsed, parseErr := parseNumberedScenarioPath(ref.Path)
			if parseErr != nil {
				return fmt.Errorf("modules[%d].scenarios[%d].path: %w", moduleIndex, scenarioIndex, parseErr)
			}
			if parsed.ModuleID != module.ID {
				return fmt.Errorf("modules[%d].scenarios[%d].path: directory module %q does not match catalog module %q",
					moduleIndex, scenarioIndex, parsed.ModuleID, module.ID)
			}
			if parsed.ID != ref.ID {
				return fmt.Errorf("modules[%d].scenarios[%d].path: file id %q does not match catalog id %q",
					moduleIndex, scenarioIndex, parsed.ID, ref.ID)
			}
			if parsed.Index != scenarioIndex+1 {
				return fmt.Errorf("modules[%d].scenarios[%d].path: lab number %02d must be %02d, the %s lab in module %q",
					moduleIndex, scenarioIndex, parsed.Index, scenarioIndex+1, ordinal(scenarioIndex+1), module.ID)
			}
			if scenarioIndex == 0 {
				if parsed.Slot <= previousSlot {
					return fmt.Errorf("modules[%d]: slot %02d must come after slot %02d", moduleIndex, parsed.Slot, previousSlot)
				}
				previousSlot = parsed.Slot
				moduleSlot = parsed.Slot
			} else if parsed.Slot != moduleSlot {
				return fmt.Errorf("modules[%d].scenarios[%d].path: slot %02d does not match module slot %02d",
					moduleIndex, scenarioIndex, parsed.Slot, moduleSlot)
			}
			seenScenarios[ref.ID] = struct{}{}
		}
	}
	return nil
}

func parseNumberedScenarioPath(filename string) (numberedPath, error) {
	match := numberedScenarioPath.FindStringSubmatch(filename)
	if match == nil {
		return numberedPath{}, fmt.Errorf("%q must be {slot}/{nn}-{id}.yaml (for example 01-foundations/01-pod-creation.yaml)", filename)
	}
	slot, _ := strconv.Atoi(match[1])
	index, _ := strconv.Atoi(match[3])
	if index < 1 {
		return numberedPath{}, fmt.Errorf("%q lab number must be 01 or higher", filename)
	}
	return numberedPath{Slot: slot, ModuleID: match[2], Index: index, ID: match[4]}, nil
}

func ordinal(n int) string {
	switch n {
	case 1:
		return "first"
	case 2:
		return "second"
	case 3:
		return "third"
	default:
		return fmt.Sprintf("%dth", n)
	}
}
