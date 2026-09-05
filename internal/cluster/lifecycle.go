package cluster

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	ownershipMarkerName      = "kubecrypt-ownership"
	ownershipMarkerNamespace = "kube-system"
)

var ErrOwnershipMismatch = errors.New("kubecrypt cluster ownership verification failed")

// Identity binds local state to one specific kind cluster.
type Identity struct {
	ClusterName   string `json:"clusterName"`
	APIServer     string `json:"apiServer"`
	CAFingerprint string `json:"caSHA256"`
}

// Manager owns lifecycle and guarded mutation operations for the dedicated
// KubeCrypt kind cluster.
type Manager struct {
	runner Runner
	paths  Paths
}

// NewManager creates a Manager using production defaults.
func NewManager(runner Runner) (*Manager, error) {
	paths, err := DefaultPaths()
	if err != nil {
		return nil, err
	}
	return NewManagerWithPaths(runner, paths), nil
}

// NewManagerWithPaths creates a Manager with explicit paths.
func NewManagerWithPaths(runner Runner, paths Paths) *Manager {
	if runner == nil {
		runner = ExecRunner{}
	}
	return &Manager{runner: runner, paths: paths}
}

// Paths returns the dedicated host paths used by this manager.
func (m *Manager) Paths() Paths {
	return m.paths
}

// CheckIn idempotently creates the dedicated cluster or verifies the identity
// of an existing cluster. It never reads or changes the global kubeconfig.
func (m *Manager) CheckIn(ctx context.Context) (Identity, error) {
	if err := m.paths.ensureDirectory(); err != nil {
		return Identity{}, err
	}
	clusters, err := m.kindClusters(ctx)
	if err != nil {
		return Identity{}, err
	}
	if containsLine(clusters, ClusterName) {
		if _, err := os.Stat(m.paths.Ownership); err != nil {
			if os.IsNotExist(err) {
				return Identity{}, fmt.Errorf("%w: kind cluster %q already exists but has no local ownership record; refusing to adopt it", ErrOwnershipMismatch, ClusterName)
			}
			return Identity{}, fmt.Errorf("inspect ownership record: %w", err)
		}
		return m.VerifyOwnership(ctx)
	}

	if err := m.writeKindConfig(); err != nil {
		return Identity{}, err
	}
	command := m.command("kind", "create", "cluster",
		"--name", ClusterName,
		"--config", m.paths.KindConfig,
		"--image", NodeImage,
		"--kubeconfig", m.paths.Kubeconfig,
	)
	result, err := m.runner.Run(ctx, command)
	if err != nil {
		return Identity{}, fmt.Errorf("create kind cluster %q: %w", ClusterName, commandError(command, result, err))
	}

	identity, err := m.inspectFreshCluster(ctx)
	if err != nil {
		return Identity{}, fmt.Errorf("verify newly created cluster: %w", err)
	}
	if err := m.createOwnershipMarker(ctx, identity); err != nil {
		return Identity{}, err
	}
	if err := writeJSONAtomic(m.paths.Ownership, identity, 0o600); err != nil {
		return Identity{}, fmt.Errorf("persist cluster ownership: %w", err)
	}
	return identity, nil
}

// VerifyOwnership verifies local identity, the live dedicated kubeconfig, kind
// membership, and the in-cluster ownership marker.
func (m *Manager) VerifyOwnership(ctx context.Context) (Identity, error) {
	identity, err := m.readIdentity()
	if err != nil {
		return Identity{}, err
	}
	if identity.ClusterName != ClusterName {
		return Identity{}, ownershipError("ownership record names cluster %q", identity.ClusterName)
	}
	clusters, err := m.kindClusters(ctx)
	if err != nil {
		return Identity{}, err
	}
	if !containsLine(clusters, ClusterName) {
		return Identity{}, ownershipError("kind does not list cluster %q", ClusterName)
	}
	live, err := m.inspectLiveKubeconfig(ctx)
	if err != nil {
		return Identity{}, err
	}
	if live != identity {
		return Identity{}, ownershipError("live cluster identity differs from the persisted ownership record")
	}
	if err := m.verifyOwnershipMarker(ctx, identity); err != nil {
		return Identity{}, err
	}
	return identity, nil
}

const workspaceWipeTimeout = "60s"

// WipeWorkspace deletes a lab namespace and waits until it is gone. Extra
// objects the learner created in that namespace go with it. Only kubecrypt-*
// names are accepted. A missing namespace is ignored.
func (m *Manager) WipeWorkspace(ctx context.Context, namespace string) error {
	if err := validateLabWorkspace(namespace); err != nil {
		return err
	}
	if _, err := m.VerifyOwnership(ctx); err != nil {
		return err
	}
	if err := m.runKubectl(ctx, nil, "delete", "namespace", namespace,
		"--ignore-not-found=true", "--wait=true", "--timeout="+workspaceWipeTimeout); err != nil {
		return fmt.Errorf("wipe workspace %q: %w", namespace, err)
	}
	return nil
}

func validateLabWorkspace(namespace string) error {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return fmt.Errorf("wipe workspace: namespace must not be empty")
	}
	if !strings.HasPrefix(namespace, "kubecrypt-") {
		return fmt.Errorf("wipe workspace: refusing namespace %q (must be kubecrypt-*)", namespace)
	}
	return nil
}

// Apply applies trusted manifest data after verifying cluster ownership.
func (m *Manager) Apply(ctx context.Context, manifest []byte) error {
	if _, err := m.VerifyOwnership(ctx); err != nil {
		return err
	}
	return m.runKubectl(ctx, manifest, "apply", "-f", "-")
}

// Reset deletes resources described by a trusted manifest and applies the
// manifest again. Ownership is reverified before each mutation.
func (m *Manager) Reset(ctx context.Context, manifest []byte) error {
	if _, err := m.VerifyOwnership(ctx); err != nil {
		return err
	}
	if err := m.runKubectl(ctx, manifest, "delete", "--ignore-not-found=true", "-f", "-"); err != nil {
		return fmt.Errorf("delete resources for reset: %w", err)
	}
	if _, err := m.VerifyOwnership(ctx); err != nil {
		return err
	}
	if err := m.runKubectl(ctx, manifest, "apply", "-f", "-"); err != nil {
		return fmt.Errorf("apply resources for reset: %w", err)
	}
	return nil
}

// Destroy removes the verified framework-owned kind cluster and local
// cluster identity files.
func (m *Manager) Destroy(ctx context.Context) error {
	clusters, err := m.kindClusters(ctx)
	if err != nil {
		return err
	}
	if !containsLine(clusters, ClusterName) {
		for _, path := range []string{m.paths.Ownership, m.paths.Kubeconfig, m.paths.KindConfig} {
			if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("remove stale cluster file %q: %w", path, removeErr)
			}
		}
		return nil
	}
	if _, err := m.VerifyOwnership(ctx); err != nil {
		return err
	}
	command := m.command("kind", "delete", "cluster", "--name", ClusterName)
	result, err := m.runner.Run(ctx, command)
	if err != nil {
		return fmt.Errorf("delete kind cluster %q: %w", ClusterName, commandError(command, result, err))
	}
	for _, path := range []string{m.paths.Ownership, m.paths.Kubeconfig, m.paths.KindConfig} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove cluster file %q: %w", path, err)
		}
	}
	return nil
}

func (m *Manager) inspectFreshCluster(ctx context.Context) (Identity, error) {
	clusters, err := m.kindClusters(ctx)
	if err != nil {
		return Identity{}, err
	}
	if !containsLine(clusters, ClusterName) {
		return Identity{}, ownershipError("new cluster is not listed by kind")
	}
	return m.inspectLiveKubeconfig(ctx)
}

func (m *Manager) inspectLiveKubeconfig(ctx context.Context) (Identity, error) {
	currentContext, err := m.kubectlOutput(ctx, "config", "current-context")
	if err != nil {
		return Identity{}, fmt.Errorf("read dedicated kubeconfig current context: %w", err)
	}
	if strings.TrimSpace(currentContext) != ContextName {
		return Identity{}, ownershipError("dedicated kubeconfig current context is %q, expected %q", strings.TrimSpace(currentContext), ContextName)
	}
	server, err := m.kubectlOutput(ctx, "config", "view", "--minify", "--raw", "-o", "jsonpath={.clusters[0].cluster.server}")
	if err != nil {
		return Identity{}, fmt.Errorf("read cluster API server: %w", err)
	}
	server = strings.TrimSpace(server)
	if server == "" {
		return Identity{}, ownershipError("dedicated kubeconfig has an empty API server")
	}
	caData, err := m.kubectlOutput(ctx, "config", "view", "--minify", "--raw", "-o", "jsonpath={.clusters[0].cluster.certificate-authority-data}")
	if err != nil {
		return Identity{}, fmt.Errorf("read cluster CA: %w", err)
	}
	fingerprint, err := caFingerprint(strings.TrimSpace(caData))
	if err != nil {
		return Identity{}, ownershipError("invalid cluster CA data: %v", err)
	}
	return Identity{ClusterName: ClusterName, APIServer: server, CAFingerprint: fingerprint}, nil
}

func (m *Manager) kindClusters(ctx context.Context) (string, error) {
	command := m.command("kind", "get", "clusters")
	result, err := m.runner.Run(ctx, command)
	if err != nil {
		return "", fmt.Errorf("list kind clusters: %w", commandError(command, result, err))
	}
	return result.Stdout, nil
}

func (m *Manager) createOwnershipMarker(ctx context.Context, identity Identity) error {
	manifest := map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      ownershipMarkerName,
			"namespace": ownershipMarkerNamespace,
			"labels": map[string]string{
				"app.kubernetes.io/managed-by": "kubecrypt",
			},
		},
		"data": map[string]string{
			"clusterName": identity.ClusterName,
			"apiServer":   identity.APIServer,
			"caSHA256":    identity.CAFingerprint,
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode ownership marker: %w", err)
	}
	if err := m.runKubectl(ctx, data, "apply", "-f", "-"); err != nil {
		return fmt.Errorf("bootstrap ownership marker: %w", err)
	}
	return nil
}

func (m *Manager) verifyOwnershipMarker(ctx context.Context, identity Identity) error {
	output, err := m.kubectlOutput(ctx, "get", "configmap", ownershipMarkerName,
		"--namespace", ownershipMarkerNamespace, "-o", "json")
	if err != nil {
		return ownershipError("read in-cluster ownership marker: %v", err)
	}
	var marker struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &marker); err != nil {
		return ownershipError("decode in-cluster ownership marker: %v", err)
	}
	if marker.Data["clusterName"] != identity.ClusterName ||
		marker.Data["apiServer"] != identity.APIServer ||
		marker.Data["caSHA256"] != identity.CAFingerprint {
		return ownershipError("in-cluster ownership marker differs from the persisted identity")
	}
	return nil
}

func (m *Manager) readIdentity() (Identity, error) {
	data, err := os.ReadFile(m.paths.Ownership)
	if err != nil {
		return Identity{}, ownershipError("read ownership record: %v", err)
	}
	var identity Identity
	if err := json.Unmarshal(data, &identity); err != nil {
		return Identity{}, ownershipError("decode ownership record: %v", err)
	}
	if identity.ClusterName == "" || identity.APIServer == "" || identity.CAFingerprint == "" {
		return Identity{}, ownershipError("ownership record is incomplete")
	}
	return identity, nil
}

func (m *Manager) kubectlOutput(ctx context.Context, args ...string) (string, error) {
	command := m.command("kubectl", args...)
	result, err := m.runner.Run(ctx, command)
	if err != nil {
		return "", commandError(command, result, err)
	}
	return result.Stdout, nil
}

func (m *Manager) runKubectl(ctx context.Context, stdin []byte, args ...string) error {
	command := m.command("kubectl", args...)
	command.Stdin = stdin
	result, err := m.runner.Run(ctx, command)
	return commandError(command, result, err)
}

func (m *Manager) command(name string, args ...string) Command {
	resolved := name
	switch name {
	case "kind":
		if path := m.paths.KindBinary(); fileExists(path) {
			resolved = path
		}
	case "kubectl":
		if path := m.paths.KubectlBinary(); fileExists(path) {
			resolved = path
		}
	}
	return Command{
		Name: resolved,
		Args: append([]string(nil), args...),
		Env:  []string{"KUBECONFIG=" + m.paths.Kubeconfig},
	}
}

func (m *Manager) writeKindConfig() error {
	const config = `kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: kubecrypt
nodes:
  - role: control-plane
  - role: worker
  - role: worker
`
	return writeFileAtomic(m.paths.KindConfig, []byte(config), 0o600)
}

func writeJSONAtomic(path string, value any, mode os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomic(path, data, mode)
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempName := file.Name()
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(tempName)
		}
	}()
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func caFingerprint(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return "", errors.New("CA is not a PEM certificate")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse certificate: %w", err)
	}
	sum := sha256.Sum256(certificate.Raw)
	return hex.EncodeToString(sum[:]), nil
}

func containsLine(output, wanted string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == wanted {
			return true
		}
	}
	return false
}

func ownershipError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrOwnershipMismatch, fmt.Sprintf(format, args...))
}
