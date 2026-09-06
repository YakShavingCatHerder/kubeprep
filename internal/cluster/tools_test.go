package cluster

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureToolsDownloadsWhenMissing(t *testing.T) {
	kindBody := []byte("kind-bytes")
	kubectlBody := []byte("kubectl-bytes")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/kind":
			_, _ = w.Write(kindBody)
		case "/kubectl":
			_, _ = w.Write(kubectlBody)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	paths := PathsForDirectory(t.TempDir())
	err := EnsureTools(context.Background(), paths, ToolOptions{
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
		Kind:     testArtifact("kind", KindVersion, server.URL+"/kind", kindBody, []string{"version"}),
		Kubectl:  testArtifact("kubectl", KubectlVersion, server.URL+"/kubectl", kubectlBody, []string{"version", "--client"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	gotKind, err := os.ReadFile(paths.KindBinary())
	if err != nil {
		t.Fatal(err)
	}
	if string(gotKind) != string(kindBody) {
		t.Fatalf("kind binary = %q", gotKind)
	}
	info, err := os.Stat(paths.KindBinary())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("kind is not executable: %o", info.Mode().Perm())
	}
}

func TestEnsureToolsSkipsMatchingManagedBinary(t *testing.T) {
	paths := PathsForDirectory(t.TempDir())
	if err := os.MkdirAll(paths.BinDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.KindBinary(), []byte("existing-kind"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.KubectlBinary(), []byte("existing-kubectl"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := EnsureTools(context.Background(), paths, ToolOptions{
		HTTPGet: func(context.Context, string) ([]byte, error) {
			t.Fatal("download should be skipped")
			return nil, nil
		},
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
		Run: func(_ context.Context, name string, _ ...string) (string, error) {
			switch filepath.Base(name) {
			case "kind":
				return "kind " + KindVersion + "\n", nil
			case "kubectl":
				return "Client Version: " + KubectlVersion + "\n", nil
			default:
				return "", fmt.Errorf("unexpected %s", name)
			}
		},
		Kind:    &Artifact{Name: "kind", Version: KindVersion, URL: "http://unused", SHA256: "00", Args: []string{"version"}},
		Kubectl: &Artifact{Name: "kubectl", Version: KubectlVersion, URL: "http://unused", SHA256: "00", Args: []string{"version", "--client"}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnsureToolsRejectsChecksumMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("tampered"))
	}))
	t.Cleanup(server.Close)
	paths := PathsForDirectory(t.TempDir())
	err := EnsureTools(context.Background(), paths, ToolOptions{
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
		Kind: &Artifact{
			Name: "kind", Version: KindVersion, URL: server.URL, SHA256: strings.Repeat("ab", 32), Args: []string{"version"},
		},
		Kubectl: testArtifact("kubectl", KubectlVersion, server.URL, []byte("kubectl-bytes"), []string{"version", "--client"}),
	})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("EnsureTools() error = %v, want checksum mismatch", err)
	}
}

func TestEnsureToolsCopiesMatchingPATHBinary(t *testing.T) {
	dir := t.TempDir()
	kindSrc := filepath.Join(dir, "kind")
	kubectlSrc := filepath.Join(dir, "kubectl")
	if err := os.WriteFile(kindSrc, []byte("path-kind"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kubectlSrc, []byte("path-kubectl"), 0o755); err != nil {
		t.Fatal(err)
	}
	paths := PathsForDirectory(t.TempDir())
	err := EnsureTools(context.Background(), paths, ToolOptions{
		HTTPGet: func(context.Context, string) ([]byte, error) {
			t.Fatal("download should be skipped when PATH matches")
			return nil, nil
		},
		LookPath: func(name string) (string, error) {
			switch name {
			case "kind":
				return kindSrc, nil
			case "kubectl":
				return kubectlSrc, nil
			default:
				return "", os.ErrNotExist
			}
		},
		Run: func(_ context.Context, name string, _ ...string) (string, error) {
			switch name {
			case kindSrc:
				return KindVersion, nil
			case kubectlSrc:
				return KubectlVersion, nil
			default:
				return "", fmt.Errorf("unexpected %s", name)
			}
		},
		Kind:    &Artifact{Name: "kind", Version: KindVersion, URL: "http://unused", SHA256: "00", Args: []string{"version"}},
		Kubectl: &Artifact{Name: "kubectl", Version: KubectlVersion, URL: "http://unused", SHA256: "00", Args: []string{"version", "--client"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(paths.KindBinary())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "path-kind" {
		t.Fatalf("copied kind = %q", got)
	}
}

func TestRequireToolsReportsMissingBinaries(t *testing.T) {
	paths := PathsForDirectory(t.TempDir())
	err := RequireTools(paths)
	if err == nil || !strings.Contains(err.Error(), "kubecrypt doctor") {
		t.Fatalf("RequireTools() error = %v, want a doctor remediation", err)
	}
}

func TestRequireToolsAcceptsExistingBinaries(t *testing.T) {
	paths := PathsForDirectory(t.TempDir())
	if err := os.MkdirAll(paths.BinDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.KindBinary(), []byte("kind"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.KubectlBinary(), []byte("kubectl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RequireTools(paths); err != nil {
		t.Fatal(err)
	}
}

func TestPinnedArtifactsCoverSupportedPlatforms(t *testing.T) {
	for _, platform := range []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"} {
		goos, goarch, _ := strings.Cut(platform, "/")
		kind, kubectl, ok := pinnedArtifacts(goos, goarch)
		if !ok {
			t.Errorf("missing pins for %s", platform)
			continue
		}
		if kind.SHA256 == "" || kubectl.SHA256 == "" {
			t.Errorf("empty checksum for %s", platform)
		}
	}
	if _, _, ok := pinnedArtifacts("windows", "amd64"); ok {
		t.Fatal("windows should not be pinned")
	}
}

func testArtifact(name, version, url string, body []byte, args []string) *Artifact {
	sum := sha256.Sum256(body)
	return &Artifact{
		Name:    name,
		Version: version,
		URL:     url,
		SHA256:  hex.EncodeToString(sum[:]),
		Args:    args,
	}
}
