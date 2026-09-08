package cluster

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	ClusterName = "kubeprep"
	ContextName = "kind-" + ClusterName
	NodeImage   = "kindest/node:v1.35.5@sha256:ce977ae6d65918d0b58a5f8b5e940429c2ce42fa3a5619ec2bbc60b949c0ac95"
)

// Paths contains all host files owned by the cluster subsystem.
type Paths struct {
	Directory  string
	Kubeconfig string
	KindConfig string
	Ownership  string
}

// DefaultPaths resolves state beneath os.UserConfigDir.
func DefaultPaths() (Paths, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user config directory: %w", err)
	}
	return PathsForDirectory(filepath.Join(root, "kubeprep")), nil
}

// PathsForDirectory constructs paths rooted at directory. It is primarily
// useful for tests and explicitly configured installations.
func PathsForDirectory(directory string) Paths {
	return Paths{
		Directory:  directory,
		Kubeconfig: filepath.Join(directory, "kubeconfig"),
		KindConfig: filepath.Join(directory, "kind.yaml"),
		Ownership:  filepath.Join(directory, "cluster-ownership.json"),
	}
}

// BinDir is where KubePrep stores pinned kind and kubectl binaries.
func (p Paths) BinDir() string {
	return filepath.Join(p.Directory, "bin")
}

// KindBinary is the managed kind executable path.
func (p Paths) KindBinary() string {
	return filepath.Join(p.BinDir(), "kind")
}

// KubectlBinary is the managed kubectl executable path.
func (p Paths) KubectlBinary() string {
	return filepath.Join(p.BinDir(), "kubectl")
}

// KubectlExecutable returns the managed kubectl if present, otherwise PATH kubectl.
func (p Paths) KubectlExecutable() string {
	if fileExists(p.KubectlBinary()) {
		return p.KubectlBinary()
	}
	return "kubectl"
}

func (p Paths) ensureDirectory() error {
	if err := os.MkdirAll(p.Directory, 0o700); err != nil {
		return fmt.Errorf("create config directory %q: %w", p.Directory, err)
	}
	if err := os.Chmod(p.Directory, 0o700); err != nil {
		return fmt.Errorf("secure config directory %q: %w", p.Directory, err)
	}
	return nil
}
