package cluster

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	KindVersion    = "v0.32.0"
	KubectlVersion = "v1.35.5"
	maxToolBytes   = 80 << 20
)

// Artifact is a pinned downloadable CLI binary.
type Artifact struct {
	Name    string
	Version string
	URL     string
	SHA256  string
	Args    []string
}

// ToolOptions controls how EnsureTools locates and installs kind and kubectl.
type ToolOptions struct {
	GOOS     string
	GOARCH   string
	HTTPGet  func(context.Context, string) ([]byte, error)
	LookPath func(string) (string, error)
	Run      func(context.Context, string, ...string) (string, error)
	Kind     *Artifact
	Kubectl  *Artifact
}

// DefaultToolOptions uses the host platform and HTTPS downloads.
func DefaultToolOptions() ToolOptions {
	return ToolOptions{}
}

// EnsureTools installs pinned kind and kubectl into paths.BinDir when they are
// missing. A PATH binary whose version matches the pin is copied instead of
// downloading. Existing managed binaries with the right version are left alone.
func EnsureTools(ctx context.Context, paths Paths, opts ToolOptions) error {
	if err := paths.ensureDirectory(); err != nil {
		return err
	}
	if err := os.MkdirAll(paths.BinDir(), 0o700); err != nil {
		return fmt.Errorf("create tool directory %q: %w", paths.BinDir(), err)
	}
	kind, kubectl, err := resolveArtifacts(opts)
	if err != nil {
		return err
	}
	if err := ensureBinary(ctx, paths.KindBinary(), kind, opts); err != nil {
		return err
	}
	return ensureBinary(ctx, paths.KubectlBinary(), kubectl, opts)
}

// RequireTools reports whether the pinned kind and kubectl binaries are already
// installed. It does not download anything.
func RequireTools(paths Paths) error {
	for _, tool := range []struct {
		name string
		path string
	}{
		{name: "kind", path: paths.KindBinary()},
		{name: "kubectl", path: paths.KubectlBinary()},
	} {
		if !fileExists(tool.path) {
			return fmt.Errorf("%s is not installed; run `kubeprep doctor`", tool.name)
		}
	}
	return nil
}

func resolveArtifacts(opts ToolOptions) (Artifact, Artifact, error) {
	if opts.Kind != nil && opts.Kubectl != nil {
		return *opts.Kind, *opts.Kubectl, nil
	}
	goos := opts.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := opts.GOARCH
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	kind, kubectl, ok := pinnedArtifacts(goos, goarch)
	if !ok {
		return Artifact{}, Artifact{}, fmt.Errorf("no pinned kind/kubectl builds for %s/%s", goos, goarch)
	}
	if opts.Kind != nil {
		kind = *opts.Kind
	}
	if opts.Kubectl != nil {
		kubectl = *opts.Kubectl
	}
	return kind, kubectl, nil
}

func pinnedArtifacts(goos, goarch string) (Artifact, Artifact, bool) {
	kindSums := map[string]string{
		"linux/amd64":  "50030de23cf40a18505f20426f6a8506bedf13c6e509244bd1fa9463721b0f54",
		"linux/arm64":  "b92cd615e97585de8ddade28ed5cd7feb4248d717c233eea5b03c37298900f5d",
		"darwin/amd64": "295ac6d0d634c9819c9907df45e3017d1f13166bd13c3404c45e79f7faa47498",
		"darwin/arm64": "dca67911095a110c2b5c36e26df6cac860c602033e456c0db47be498cdef1ebb",
	}
	kubectlSums := map[string]string{
		"linux/amd64":  "90f75ea6ecc9ea5633262e1c0b83a40560003b30fc94a04cb099404fcef0c224",
		"linux/arm64":  "ac69e06fd6860d69786692f5af1c3a1208ed54f8366a4d97ab15c172e99765ee",
		"darwin/amd64": "d6af0a35e78865ab3352a3f76e7346041f6657d08641db1c1e93830210fce334",
		"darwin/arm64": "4d79aa4ca3015d6c70ac593d8c387efe09f4000b5ff1098f42610e6030a8ccb3",
	}
	key := goos + "/" + goarch
	kindSum, okKind := kindSums[key]
	kubectlSum, okKubectl := kubectlSums[key]
	if !okKind || !okKubectl {
		return Artifact{}, Artifact{}, false
	}
	kindName := "kind-" + goos + "-" + goarch
	return Artifact{
		Name:    "kind",
		Version: KindVersion,
		URL:     "https://kind.sigs.k8s.io/dl/" + KindVersion + "/" + kindName,
		SHA256:  kindSum,
		Args:    []string{"version"},
	}, Artifact{
		Name:    "kubectl",
		Version: KubectlVersion,
		URL:     "https://dl.k8s.io/release/" + KubectlVersion + "/bin/" + goos + "/" + goarch + "/kubectl",
		SHA256:  kubectlSum,
		Args:    []string{"version", "--client"},
	}, true
}

func ensureBinary(ctx context.Context, dest string, artifact Artifact, opts ToolOptions) error {
	run := opts.Run
	if run == nil {
		run = runCommandOutput
	}
	if fileExists(dest) {
		output, err := run(ctx, dest, artifact.Args...)
		if err == nil && strings.Contains(output, artifact.Version) {
			return nil
		}
	}
	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if path, err := lookPath(artifact.Name); err == nil && path != "" {
		output, runErr := run(ctx, path, artifact.Args...)
		if runErr == nil && strings.Contains(output, artifact.Version) {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("read %s from PATH: %w", artifact.Name, readErr)
			}
			return installBinary(dest, data)
		}
	}
	get := opts.HTTPGet
	if get == nil {
		get = httpGet
	}
	data, err := get(ctx, artifact.URL)
	if err != nil {
		return fmt.Errorf("download %s %s: %w", artifact.Name, artifact.Version, err)
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, artifact.SHA256) {
		return fmt.Errorf("download %s %s: checksum mismatch (got %s)", artifact.Name, artifact.Version, got)
	}
	return installBinary(dest, data)
}

func installBinary(dest string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	return writeFileAtomic(dest, data, 0o755)
}

func httpGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "kubeprep")
	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxToolBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxToolBytes {
		return nil, fmt.Errorf("GET %s: response exceeds %d bytes", url, maxToolBytes)
	}
	return data, nil
}

func runCommandOutput(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail != "" {
			return "", fmt.Errorf("run %s: %w: %s", name, err, detail)
		}
		return "", fmt.Errorf("run %s: %w", name, err)
	}
	return stdout.String() + stderr.String(), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
