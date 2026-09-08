package curriculum

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	corepack "github.com/YakShavingCatHerder/kubecrypt/curriculum"
	"gopkg.in/yaml.v3"
)

type Source struct {
	Name string
	FS   fs.FS
}

type containedDirFS struct {
	root string
}

func (f containedDirFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(f.root, filepath.FromSlash(name)))
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(f.root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return os.Open(resolved)
}

type scenarioLocation struct {
	sourceIndex int
	filename    string
	scenario    *Scenario
}

// Registry is an ordered collection of validated scenario packs.
type Registry struct {
	sources   []Source
	catalogs  []Catalog
	locations map[string]scenarioLocation
	order     []string
}

// NewRegistry loads the embedded core pack followed by explicitly selected
// local packs. Scenario IDs must be unique across every source.
func NewRegistry(localPackDirectories ...string) (*Registry, error) {
	sources := []Source{{Name: "core", FS: corepack.Files}}
	for _, directory := range localPackDirectories {
		clean := filepath.Clean(directory)
		info, statErr := os.Stat(clean)
		if statErr != nil {
			return nil, fmt.Errorf("open local scenario pack %q: %w", directory, statErr)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("open local scenario pack %q: not a directory", directory)
		}
		absolute, absErr := containedRoot(clean)
		if absErr != nil {
			return nil, fmt.Errorf("open local scenario pack %q: %w", directory, absErr)
		}
		sources = append(sources, Source{Name: absolute, FS: containedDirFS{root: absolute}})
	}
	return NewRegistryFromSources(sources...)
}

// NewRegistryFromSources is the injectable registry constructor used by tests
// and alternate front ends.
func NewRegistryFromSources(sources ...Source) (*Registry, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("scenario registry requires at least one source")
	}
	registry := &Registry{
		sources:   append([]Source(nil), sources...),
		locations: make(map[string]scenarioLocation),
	}
	seenPacks := make(map[string]struct{})
	for sourceIndex, source := range registry.sources {
		if source.FS == nil {
			return nil, fmt.Errorf("scenario source %q: filesystem is nil", source.Name)
		}
		catalog, err := loadCatalogFS(source.FS, "catalog.yaml", source.Name)
		if err != nil {
			return nil, err
		}
		if _, exists := seenPacks[catalog.Name]; exists {
			return nil, fmt.Errorf("scenario source %q: duplicate pack name %q", source.Name, catalog.Name)
		}
		seenPacks[catalog.Name] = struct{}{}
		for _, ref := range catalog.UniqueLabs() {
			if previous, exists := registry.locations[ref.ID]; exists {
				return nil, fmt.Errorf("scenario source %q: duplicate scenario id %q already provided by %q",
					source.Name, ref.ID, registry.sources[previous.sourceIndex].Name)
			}
			scenario, loadErr := loadFS(source.FS, ref.Path, source.Name+":"+ref.Path)
			if loadErr != nil {
				return nil, loadErr
			}
			if scenario.ID != ref.ID {
				return nil, fmt.Errorf("scenario source %q: catalog id %q does not match file id %q", source.Name, ref.ID, scenario.ID)
			}
			if scenario.Module != ref.Section {
				return nil, fmt.Errorf("scenario source %q: scenario %q declares module %q, catalog section is %q",
					source.Name, ref.ID, scenario.Module, ref.Section)
			}
			registry.locations[ref.ID] = scenarioLocation{
				sourceIndex: sourceIndex,
				filename:    ref.Path,
				scenario:    scenario,
			}
			registry.order = append(registry.order, ref.ID)
		}
		registry.catalogs = append(registry.catalogs, *catalog)
	}
	return registry, nil
}

// ValidatePack validates a standalone local pack without loading the core pack.
func ValidatePack(directory string) (*Catalog, error) {
	clean := filepath.Clean(directory)
	info, err := os.Stat(clean)
	if err != nil {
		return nil, fmt.Errorf("validate scenario pack %q: %w", directory, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("validate scenario pack %q: not a directory", directory)
	}
	absolute, err := containedRoot(clean)
	if err != nil {
		return nil, fmt.Errorf("validate scenario pack %q: %w", directory, err)
	}
	registry, err := NewRegistryFromSources(Source{Name: absolute, FS: containedDirFS{root: absolute}})
	if err != nil {
		return nil, err
	}
	catalog := registry.catalogs[0]
	return &catalog, nil
}

func containedRoot(directory string) (string, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}

func (r *Registry) Catalogs() []Catalog {
	return append([]Catalog(nil), r.catalogs...)
}

func (r *Registry) ScenarioIDs() []string {
	return append([]string(nil), r.order...)
}

// PlayOrder is lab ids for one learner path across every loaded pack.
func (r *Registry) PlayOrder(pathID string) []string {
	var ids []string
	seen := make(map[string]struct{})
	for _, catalog := range r.catalogs {
		for _, id := range catalog.LabIDsOnPath(pathID) {
			if _, exists := seen[id]; exists {
				continue
			}
			if _, known := r.locations[id]; !known {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids
}

func (r *Registry) LoadScenario(id string) (*Scenario, error) {
	location, ok := r.locations[id]
	if !ok {
		return nil, fmt.Errorf("load scenario: unknown id %q", id)
	}
	copy := *location.scenario
	return &copy, nil
}

func (r *Registry) ReadManifest(scenarioID, reference string) ([]byte, error) {
	location, ok := r.locations[scenarioID]
	if !ok {
		return nil, fmt.Errorf("read scenario manifest: unknown scenario id %q", scenarioID)
	}
	if !fs.ValidPath(reference) || path.Clean(reference) != reference {
		return nil, fmt.Errorf("read scenario manifest: invalid reference %q", reference)
	}
	filename := path.Join(path.Dir(location.filename), reference)
	data, err := fs.ReadFile(r.sources[location.sourceIndex].FS, filename)
	if err != nil {
		return nil, fmt.Errorf("read scenario manifest %q: %w", reference, err)
	}
	if err := validateManifestSafety(reference, data); err != nil {
		return nil, err
	}
	return data, nil
}

// LoadFile strictly decodes and validates one standalone scenario.
func LoadFile(filename string) (*Scenario, error) {
	clean := filepath.Clean(filename)
	if clean == "." {
		return nil, fmt.Errorf("load scenario %q: invalid filename", filename)
	}
	return loadFS(os.DirFS(filepath.Dir(clean)), filepath.Base(clean), filename)
}

func LoadFS(fsys fs.FS, filename string) (*Scenario, error) {
	if fsys == nil {
		return nil, fmt.Errorf("load scenario %q: filesystem is nil", filename)
	}
	if !fs.ValidPath(filename) {
		return nil, fmt.Errorf("load scenario %q: filename must be a clean relative path", filename)
	}
	return loadFS(fsys, filename, filename)
}

func loadCatalogFS(fsys fs.FS, filename, sourceName string) (*Catalog, error) {
	file, err := fsys.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("load scenario catalog from %q: %w", sourceName, err)
	}
	defer file.Close()
	var catalog Catalog
	if err := decodeOne(file, &catalog); err != nil {
		return nil, fmt.Errorf("decode scenario catalog from %q: %w", sourceName, err)
	}
	if err := validateCatalog(&catalog); err != nil {
		return nil, fmt.Errorf("validate scenario catalog from %q: %w", sourceName, err)
	}
	return &catalog, nil
}

func loadFS(fsys fs.FS, filename, displayName string) (*Scenario, error) {
	file, err := fsys.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("load scenario %q: %w", displayName, err)
	}
	defer file.Close()
	document, err := decodeDocument(file, displayName)
	if err != nil {
		return nil, err
	}
	if err := Validate(&document.Scenario); err != nil {
		return nil, fmt.Errorf("validate scenario %q: %w", displayName, err)
	}
	if start := strings.TrimSpace(document.StartingClusterConfiguration); start != "" {
		if err := validateManifestSafety("startingClusterConfiguration", []byte(start)); err != nil {
			return nil, fmt.Errorf("validate scenario %q: %w", displayName, err)
		}
	}
	if err := verifyManifestFiles(fsys, path.Dir(filename), document.Setup.Manifests, document.Reset.Manifests); err != nil {
		return nil, fmt.Errorf("validate scenario %q: %w", displayName, err)
	}
	scenario := document.Scenario
	return &scenario, nil
}

// LoadDocument strictly decodes one scenario file, including authoring notes.
func LoadDocument(filename string) (*Document, error) {
	clean := filepath.Clean(filename)
	if clean == "." {
		return nil, fmt.Errorf("load scenario %q: invalid filename", filename)
	}
	file, err := os.Open(clean)
	if err != nil {
		return nil, fmt.Errorf("load scenario %q: %w", filename, err)
	}
	defer file.Close()
	document, err := decodeDocument(file, filename)
	if err != nil {
		return nil, err
	}
	if err := Validate(&document.Scenario); err != nil {
		return nil, fmt.Errorf("validate scenario %q: %w", filename, err)
	}
	return document, nil
}

func decodeDocument(reader io.Reader, displayName string) (*Document, error) {
	var document Document
	if err := decodeOne(reader, &document); err != nil {
		return nil, fmt.Errorf("decode scenario %q: %w", displayName, err)
	}
	return &document, nil
}

func decodeOne(reader io.Reader, target any) error {
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("multiple YAML documents are not allowed")
	}
	return nil
}

func verifyManifestFiles(fsys fs.FS, dir string, groups ...[]string) error {
	seen := make(map[string]struct{})
	for _, refs := range groups {
		for _, ref := range refs {
			filename := path.Join(dir, ref)
			if _, ok := seen[filename]; ok {
				continue
			}
			seen[filename] = struct{}{}
			info, err := fs.Stat(fsys, filename)
			if err != nil {
				return fmt.Errorf("manifest %q: %w", ref, err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("manifest %q: is not a regular file", ref)
			}
			data, err := fs.ReadFile(fsys, filename)
			if err != nil {
				return fmt.Errorf("manifest %q: %w", ref, err)
			}
			if err := validateManifestSafety(ref, data); err != nil {
				return err
			}
		}
	}
	return nil
}
