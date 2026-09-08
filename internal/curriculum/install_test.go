package curriculum

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInstallLabCopiesFileAndCatalog(t *testing.T) {
	packDir := t.TempDir()
	writePackCatalog(t, packDir, validCatalog())
	source := writeInstallableLab(t, t.TempDir(), "draft.yaml", nil)

	install, err := InstallLab(source, packDir)
	if err != nil {
		t.Fatal(err)
	}
	if install.ID != "test-scenario" || install.Section != "workloads" || install.Path != "workloads/test-scenario.yaml" {
		t.Fatalf("install = %#v", install)
	}
	copied, err := os.ReadFile(filepath.Join(packDir, "workloads", "test-scenario.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(copied), "id: test-scenario") {
		t.Fatalf("copied lab missing id: %s", copied)
	}
	catalog, err := loadCatalogFile(filepath.Join(packDir, "catalog.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(catalog.LabIDsOnPath("beginner"), ","); got != "test-scenario" {
		t.Fatalf("beginner labs = %s", got)
	}
	if got := strings.Join(catalog.LabIDsOnPath("cka"), ","); got != "test-scenario" {
		t.Fatalf("cka labs = %s", got)
	}
}

func TestInstallLabIsIdempotent(t *testing.T) {
	packDir := t.TempDir()
	writePackCatalog(t, packDir, validCatalog())
	source := writeInstallableLab(t, t.TempDir(), "draft.yaml", nil)

	if _, err := InstallLab(source, packDir); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallLab(source, packDir); err != nil {
		t.Fatal(err)
	}
	catalog, err := loadCatalogFile(filepath.Join(packDir, "catalog.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(catalog.LabIDsOnPath("beginner"), ","); got != "test-scenario" {
		t.Fatalf("beginner labs = %s", got)
	}
}

func TestInstallLabRejectsSectionMismatch(t *testing.T) {
	packDir := t.TempDir()
	writePackCatalog(t, packDir, validCatalog())
	sourceDir := t.TempDir()
	scenario := validScenario()
	scenario.Setup.Manifests = nil
	scenario.Reset.Manifests = nil
	scenario.Namespace = "kubeprep-test"
	body, err := yaml.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sourceDir, "draft.yaml")
	if err := os.WriteFile(source, append([]byte("authoring:\n  section: pods\n"), body...), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = InstallLab(source, packDir)
	if err == nil || !strings.Contains(err.Error(), "does not match module") {
		t.Fatalf("InstallLab() error = %v", err)
	}
}

func TestInstallLabCopiesManifests(t *testing.T) {
	packDir := t.TempDir()
	writePackCatalog(t, packDir, validCatalog())
	sourceDir := t.TempDir()
	source := writeInstallableLab(t, sourceDir, "draft.yaml", []string{"workload.yaml"})
	if err := os.WriteFile(filepath.Join(sourceDir, "workload.yaml"), []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: record
  namespace: kubeprep-test
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallLab(source, packDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(packDir, "workloads", "workload.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallLabSkipsCopyWhenAlreadyInPack(t *testing.T) {
	packDir := t.TempDir()
	writePackCatalog(t, packDir, validCatalog())
	source := writeInstallableLab(t, filepath.Join(packDir, "workloads"), "test-scenario.yaml", nil)
	if _, err := InstallLab(source, packDir); err != nil {
		t.Fatal(err)
	}
	catalog, err := loadCatalogFile(filepath.Join(packDir, "catalog.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(catalog.LabIDsOnPath("beginner"), ","); got != "test-scenario" {
		t.Fatalf("beginner labs = %s", got)
	}
}

func TestMaterializeDraftWritesOverlayPack(t *testing.T) {
	packDir := t.TempDir()
	source := writeInstallableLab(t, t.TempDir(), "draft.yaml", nil)

	install, err := MaterializeDraft(source, packDir)
	if err != nil {
		t.Fatal(err)
	}
	if install.ID != "test-scenario" || install.Section != "workloads" || install.Path != "workloads/test-scenario.yaml" {
		t.Fatalf("install = %#v", install)
	}
	if _, err := os.Stat(filepath.Join(packDir, "workloads", "test-scenario.yaml")); err != nil {
		t.Fatal(err)
	}
	catalog, err := loadCatalogFile(filepath.Join(packDir, "catalog.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Name != "draft" {
		t.Fatalf("catalog name = %q", catalog.Name)
	}
	if got := strings.Join(catalog.LabIDsOnPath("beginner"), ","); got != "test-scenario" {
		t.Fatalf("beginner labs = %s", got)
	}
}

func TestMaterializeDraftDoesNotNeedExistingCatalog(t *testing.T) {
	packDir := t.TempDir()
	sourceDir := t.TempDir()
	source := writeInstallableLab(t, sourceDir, "draft.yaml", []string{"workload.yaml"})
	if err := os.WriteFile(filepath.Join(sourceDir, "workload.yaml"), []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: record
  namespace: kubeprep-test
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeDraft(source, packDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(packDir, "workloads", "workload.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestMaterializeDraftRejectsSectionMismatch(t *testing.T) {
	sourceDir := t.TempDir()
	scenario := validScenario()
	scenario.Setup.Manifests = nil
	scenario.Reset.Manifests = nil
	scenario.Namespace = "kubeprep-test"
	body, err := yaml.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sourceDir, "draft.yaml")
	if err := os.WriteFile(source, append([]byte("authoring:\n  section: pods\n"), body...), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = MaterializeDraft(source, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "does not match module") {
		t.Fatalf("MaterializeDraft() error = %v", err)
	}
}

func TestAddLabRejectsExistingSectionConflict(t *testing.T) {
	catalog := validCatalog()
	err := catalog.AddLab("other", "test-scenario", []string{"beginner"})
	if err == nil || !strings.Contains(err.Error(), "already in section") {
		t.Fatalf("AddLab() error = %v", err)
	}
}

func writePackCatalog(t *testing.T, packDir string, catalog Catalog) {
	t.Helper()
	if err := writeCatalogFile(filepath.Join(packDir, "catalog.yaml"), &catalog); err != nil {
		t.Fatal(err)
	}
}

func writeInstallableLab(t *testing.T, dir, name string, manifests []string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	scenario := validScenario()
	scenario.Setup.Manifests = manifests
	scenario.Reset.Manifests = manifests
	if len(manifests) == 0 {
		scenario.Namespace = "kubeprep-test"
	}
	body, err := yaml.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(dir, name)
	if err := os.WriteFile(filename, append([]byte("authoring:\n  section: workloads\n"), body...), 0o600); err != nil {
		t.Fatal(err)
	}
	return filename
}
