package curriculum

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LabInstall is the pack location of a lab after InstallLab or MaterializeDraft.
type LabInstall struct {
	ID      string
	Section string
	Path    string
}

func inspectLab(sourceFile string) (*Document, LabInstall, error) {
	document, err := LoadDocument(sourceFile)
	if err != nil {
		return nil, LabInstall{}, err
	}
	section, err := sectionForDocument(document)
	if err != nil {
		return nil, LabInstall{}, err
	}
	if err := verifySidecarManifests(filepath.Dir(sourceFile), document); err != nil {
		return nil, LabInstall{}, err
	}
	return document, LabInstall{ID: document.ID, Section: section, Path: LabFilePath(section, document.ID)}, nil
}

// MaterializeDraft writes a one-lab overlay pack into packDir. It does not
// read or write an existing catalog. Destination is {section}/{id}.yaml.
func MaterializeDraft(sourceFile, packDir string) (LabInstall, error) {
	document, install, err := inspectLab(sourceFile)
	if err != nil {
		return LabInstall{}, err
	}
	catalog := &Catalog{
		APIVersion: CatalogAPIVersionV1Alpha1,
		Name:       "draft",
		Title:      "Draft",
		Revision:   document.Revision,
	}
	if err := catalog.AddLab(install.Section, install.ID, document.Tracks); err != nil {
		return LabInstall{}, err
	}
	dest := filepath.Join(packDir, filepath.FromSlash(install.Path))
	if err := copyLabTree(sourceFile, dest, document); err != nil {
		return LabInstall{}, err
	}
	if err := writeCatalogFile(filepath.Join(packDir, "catalog.yaml"), catalog); err != nil {
		return LabInstall{}, fmt.Errorf("write catalog: %w", err)
	}
	if _, err := ValidatePack(packDir); err != nil {
		return LabInstall{}, err
	}
	return install, nil
}

// InstallLab copies a lab YAML (and its sidecar manifests) into packDir and
// appends the lab id to catalog paths taken from tracks. Destination is
// {section}/{id}.yaml. The source file is overwritten in place when it is
// already that path. Catalog list order is preserved; a new id is appended.
func InstallLab(sourceFile, packDir string) (LabInstall, error) {
	document, install, err := inspectLab(sourceFile)
	if err != nil {
		return LabInstall{}, err
	}

	catalogPath := filepath.Join(packDir, "catalog.yaml")
	catalog, err := loadCatalogFile(catalogPath)
	if err != nil {
		return LabInstall{}, err
	}
	if err := catalog.AddLab(install.Section, install.ID, document.Tracks); err != nil {
		return LabInstall{}, err
	}

	dest := filepath.Join(packDir, filepath.FromSlash(install.Path))
	if err := copyLabTree(sourceFile, dest, document); err != nil {
		return LabInstall{}, err
	}
	if err := writeCatalogFile(catalogPath, catalog); err != nil {
		return LabInstall{}, fmt.Errorf("write catalog: %w", err)
	}
	if _, err := ValidatePack(packDir); err != nil {
		return LabInstall{}, err
	}
	return install, nil
}

func sectionForDocument(document *Document) (string, error) {
	section := strings.TrimSpace(document.Authoring.Section)
	module := strings.TrimSpace(document.Module)
	if section == "" {
		section = module
	}
	if section == "" {
		return "", fmt.Errorf("authoring.section or module is required")
	}
	if module != "" && section != module {
		return "", fmt.Errorf("authoring.section %q does not match module %q", section, module)
	}
	if !idPattern.MatchString(section) {
		return "", fmt.Errorf("section %q must be lowercase kebab-case", section)
	}
	return section, nil
}

// AddLab appends id under section on each path. Existing entries are left in place.
func (c *Catalog) AddLab(section, id string, pathIDs []string) error {
	if c == nil {
		return fmt.Errorf("catalog is nil")
	}
	if !idPattern.MatchString(section) {
		return fmt.Errorf("section %q must be lowercase kebab-case", section)
	}
	if !idPattern.MatchString(id) {
		return fmt.Errorf("id %q must be lowercase kebab-case", id)
	}
	if len(pathIDs) == 0 {
		return fmt.Errorf("tracks: must contain at least one path")
	}
	if existing := c.sectionForLab(id); existing != "" && existing != section {
		return fmt.Errorf("lab %q is already in section %q", id, existing)
	}
	for _, pathID := range pathIDs {
		if !idPattern.MatchString(pathID) {
			return fmt.Errorf("path %q must be lowercase kebab-case", pathID)
		}
		pathIndex := c.ensurePathIndex(pathID)
		sectionIndex := c.ensureSectionIndex(pathIndex, section)
		if containsLab(c.Paths[pathIndex].Sections[sectionIndex].Labs, id) {
			continue
		}
		c.Paths[pathIndex].Sections[sectionIndex].Labs = append(c.Paths[pathIndex].Sections[sectionIndex].Labs, id)
	}
	return validateCatalog(c)
}

func (c *Catalog) ensurePathIndex(id string) int {
	for i := range c.Paths {
		if c.Paths[i].ID == id {
			return i
		}
	}
	c.Paths = append(c.Paths, Path{ID: id, Title: pathTitle(id)})
	return len(c.Paths) - 1
}

func (c *Catalog) ensureSectionIndex(pathIndex int, id string) int {
	for i := range c.Paths[pathIndex].Sections {
		if c.Paths[pathIndex].Sections[i].ID == id {
			return i
		}
	}
	c.Paths[pathIndex].Sections = append(c.Paths[pathIndex].Sections, Section{ID: id})
	return len(c.Paths[pathIndex].Sections) - 1
}

func (c *Catalog) sectionForLab(id string) string {
	for _, learningPath := range c.Paths {
		for _, section := range learningPath.Sections {
			if containsLab(section.Labs, id) {
				return section.ID
			}
		}
	}
	return ""
}

func containsLab(ids []string, id string) bool {
	for _, existing := range ids {
		if existing == id {
			return true
		}
	}
	return false
}

func pathTitle(id string) string {
	switch id {
	case "beginner":
		return "Beginner"
	case "cka":
		return "CKA"
	case "ckad":
		return "CKAD"
	default:
		return id
	}
}

func verifySidecarManifests(dir string, document *Document) error {
	for _, ref := range uniqueManifests(document) {
		filename := filepath.Join(dir, filepath.FromSlash(ref))
		info, err := os.Stat(filename)
		if err != nil {
			return fmt.Errorf("manifest %q: %w", ref, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("manifest %q: is not a regular file", ref)
		}
	}
	return nil
}

func uniqueManifests(document *Document) []string {
	seen := make(map[string]struct{})
	var refs []string
	for _, ref := range append(append([]string{}, document.Setup.Manifests...), document.Reset.Manifests...) {
		if strings.TrimSpace(ref) == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
	}
	return refs
}

func copyLabTree(sourceFile, destFile string, document *Document) error {
	same, err := samePath(sourceFile, destFile)
	if err != nil {
		return err
	}
	if !same {
		if err := copyFile(sourceFile, destFile); err != nil {
			return fmt.Errorf("copy lab: %w", err)
		}
	}
	sourceDir := filepath.Dir(sourceFile)
	destDir := filepath.Dir(destFile)
	for _, ref := range uniqueManifests(document) {
		from := filepath.Join(sourceDir, filepath.FromSlash(ref))
		to := filepath.Join(destDir, filepath.FromSlash(ref))
		sameManifest, err := samePath(from, to)
		if err != nil {
			return err
		}
		if sameManifest {
			continue
		}
		if err := copyFile(from, to); err != nil {
			return fmt.Errorf("copy manifest %q: %w", ref, err)
		}
	}
	return nil
}

func samePath(a, b string) (bool, error) {
	left, err := filepath.Abs(a)
	if err != nil {
		return false, err
	}
	right, err := filepath.Abs(b)
	if err != nil {
		return false, err
	}
	if resolved, err := filepath.EvalSymlinks(left); err == nil {
		left = resolved
	}
	if resolved, err := filepath.EvalSymlinks(right); err == nil {
		right = resolved
	}
	return left == right, nil
}

func copyFile(from, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	return os.WriteFile(to, data, 0o644)
}

func loadCatalogFile(filename string) (*Catalog, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("load catalog %q: %w", filename, err)
	}
	defer file.Close()
	var catalog Catalog
	if err := decodeOne(file, &catalog); err != nil {
		return nil, fmt.Errorf("decode catalog %q: %w", filename, err)
	}
	if err := validateCatalog(&catalog); err != nil {
		return nil, fmt.Errorf("validate catalog %q: %w", filename, err)
	}
	return &catalog, nil
}

func writeCatalogFile(filename string, catalog *Catalog) error {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(catalog); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	tmp := filename + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, filename); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
