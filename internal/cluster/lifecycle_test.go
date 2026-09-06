package cluster

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	run func(Command) (Result, error)
	got []Command
}

func (f *fakeRunner) Run(_ context.Context, command Command) (Result, error) {
	f.got = append(f.got, command)
	if f.run == nil {
		return Result{}, nil
	}
	return f.run(command)
}

func TestCheckInCreatesDedicatedClusterAndIdentity(t *testing.T) {
	paths := PathsForDirectory(t.TempDir())
	ca, fingerprint := testCA(t)
	server := "https://127.0.0.1:6443"
	runner := &fakeRunner{}
	runner.run = func(command Command) (Result, error) {
		switch {
		case command.Name == "kind" && slices.Equal(command.Args, []string{"get", "clusters"}):
			if countCommand(runner.got, "kind", "get", "clusters") == 1 {
				return Result{}, nil
			}
			return Result{Stdout: ClusterName + "\n"}, nil
		case command.Name == "kind" && len(command.Args) >= 2 && command.Args[0] == "create":
			if !containsArgs(command.Args, "--image", NodeImage) {
				t.Errorf("kind create args do not contain pinned image: %v", command.Args)
			}
			if !containsArgs(command.Args, "--kubeconfig", paths.Kubeconfig) {
				t.Errorf("kind create args do not contain dedicated kubeconfig: %v", command.Args)
			}
			return Result{Stdout: "created"}, nil
		case command.Name == "kubectl" && slices.Equal(command.Args, []string{"config", "current-context"}):
			return Result{Stdout: ContextName + "\n"}, nil
		case command.Name == "kubectl" && strings.Contains(strings.Join(command.Args, " "), ".cluster.server"):
			return Result{Stdout: server}, nil
		case command.Name == "kubectl" && strings.Contains(strings.Join(command.Args, " "), "certificate-authority-data"):
			return Result{Stdout: ca}, nil
		case command.Name == "kubectl" && slices.Equal(command.Args, []string{"apply", "-f", "-"}):
			var marker struct {
				Data map[string]string `json:"data"`
			}
			if err := json.Unmarshal(command.Stdin, &marker); err != nil {
				t.Fatalf("decode marker input: %v", err)
			}
			if marker.Data["caSHA256"] != fingerprint {
				t.Errorf("marker fingerprint = %q, want %q", marker.Data["caSHA256"], fingerprint)
			}
			return Result{}, nil
		default:
			return Result{}, fmt.Errorf("unexpected command: %s %v", command.Name, command.Args)
		}
	}

	identity, err := NewManagerWithPaths(runner, paths).CheckIn(context.Background())
	if err != nil {
		t.Fatalf("CheckIn() error = %v", err)
	}
	if identity != (Identity{ClusterName: ClusterName, APIServer: server, CAFingerprint: fingerprint}) {
		t.Fatalf("CheckIn() identity = %#v", identity)
	}
	for _, command := range runner.got {
		if !slices.Contains(command.Env, "KUBECONFIG="+paths.Kubeconfig) {
			t.Errorf("%s %v environment = %v, missing dedicated KUBECONFIG", command.Name, command.Args, command.Env)
		}
	}
	info, err := os.Stat(paths.Ownership)
	if err != nil {
		t.Fatalf("stat ownership: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("ownership mode = %o, want 600", got)
	}
	config, err := os.ReadFile(paths.KindConfig)
	if err != nil {
		t.Fatalf("read kind config: %v", err)
	}
	if strings.Count(string(config), "role:") != 3 {
		t.Errorf("kind config is not the expected three-node topology:\n%s", config)
	}
}

func TestApplyRejectsUnrelatedCurrentContext(t *testing.T) {
	paths := PathsForDirectory(t.TempDir())
	_, fingerprint := testCA(t)
	writeIdentityForTest(t, paths, Identity{
		ClusterName:   ClusterName,
		APIServer:     "https://127.0.0.1:6443",
		CAFingerprint: fingerprint,
	})
	runner := &fakeRunner{run: func(command Command) (Result, error) {
		switch {
		case command.Name == "kind":
			return Result{Stdout: ClusterName + "\n"}, nil
		case command.Name == "kubectl" && slices.Equal(command.Args, []string{"config", "current-context"}):
			return Result{Stdout: "production\n"}, nil
		default:
			return Result{}, fmt.Errorf("mutation or unexpected command reached: %s %v", command.Name, command.Args)
		}
	}}

	err := NewManagerWithPaths(runner, paths).Apply(context.Background(), []byte("apiVersion: v1"))
	if !errors.Is(err, ErrOwnershipMismatch) {
		t.Fatalf("Apply() error = %v, want ErrOwnershipMismatch", err)
	}
	if got := countCommand(runner.got, "kubectl", "apply", "-f", "-"); got != 0 {
		t.Fatalf("kubectl apply called %d times after ownership rejection", got)
	}
}

func TestCheckInExistingClusterRequiresOwnershipRecord(t *testing.T) {
	paths := PathsForDirectory(t.TempDir())
	runner := &fakeRunner{run: func(command Command) (Result, error) {
		if command.Name == "kind" {
			return Result{Stdout: ClusterName + "\n"}, nil
		}
		return Result{}, fmt.Errorf("unexpected command: %s", command.Name)
	}}
	_, err := NewManagerWithPaths(runner, paths).CheckIn(context.Background())
	if !errors.Is(err, ErrOwnershipMismatch) {
		t.Fatalf("CheckIn() error = %v, want ErrOwnershipMismatch", err)
	}
	if len(runner.got) != 1 {
		t.Fatalf("commands = %d, want only kind membership check", len(runner.got))
	}
}

func TestVerifyOwnershipChecksMarker(t *testing.T) {
	paths := PathsForDirectory(t.TempDir())
	ca, fingerprint := testCA(t)
	identity := Identity{
		ClusterName:   ClusterName,
		APIServer:     "https://127.0.0.1:6443",
		CAFingerprint: fingerprint,
	}
	writeIdentityForTest(t, paths, identity)
	runner := &fakeRunner{run: func(command Command) (Result, error) {
		joined := strings.Join(command.Args, " ")
		switch {
		case command.Name == "kind":
			return Result{Stdout: ClusterName + "\n"}, nil
		case joined == "config current-context":
			return Result{Stdout: ContextName}, nil
		case strings.Contains(joined, ".cluster.server"):
			return Result{Stdout: identity.APIServer}, nil
		case strings.Contains(joined, "certificate-authority-data"):
			return Result{Stdout: ca}, nil
		case strings.HasPrefix(joined, "get configmap"):
			data, _ := json.Marshal(map[string]any{"data": map[string]string{
				"clusterName": identity.ClusterName,
				"apiServer":   identity.APIServer,
				"caSHA256":    identity.CAFingerprint,
			}})
			return Result{Stdout: string(data)}, nil
		default:
			return Result{}, fmt.Errorf("unexpected command: %s %v", command.Name, command.Args)
		}
	}}

	got, err := NewManagerWithPaths(runner, paths).VerifyOwnership(context.Background())
	if err != nil {
		t.Fatalf("VerifyOwnership() error = %v", err)
	}
	if got != identity {
		t.Fatalf("VerifyOwnership() = %#v, want %#v", got, identity)
	}
}

func TestSetContextNamespacePinsKubecryptWorkspace(t *testing.T) {
	paths := PathsForDirectory(t.TempDir())
	ca, fingerprint := testCA(t)
	identity := Identity{
		ClusterName:   ClusterName,
		APIServer:     "https://127.0.0.1:6443",
		CAFingerprint: fingerprint,
	}
	writeIdentityForTest(t, paths, identity)
	runner := ownershipOKRunner(t, identity, ca, func(command Command) (Result, error) {
		want := []string{"config", "set-context", "--current", "--namespace", "kubecrypt-foundations"}
		if command.Name == "kubectl" && slices.Equal(command.Args, want) {
			return Result{}, nil
		}
		return Result{}, fmt.Errorf("unexpected command: %s %v", command.Name, command.Args)
	})

	if err := NewManagerWithPaths(runner, paths).SetContextNamespace(context.Background(), "kubecrypt-foundations"); err != nil {
		t.Fatalf("SetContextNamespace() error = %v", err)
	}
	if got := countCommand(runner.got, "kubectl", "config", "set-context", "--current", "--namespace", "kubecrypt-foundations"); got != 1 {
		t.Fatalf("set-context called %d times, want 1", got)
	}
}

func TestSetContextNamespaceAllowsDefault(t *testing.T) {
	paths := PathsForDirectory(t.TempDir())
	ca, fingerprint := testCA(t)
	identity := Identity{
		ClusterName:   ClusterName,
		APIServer:     "https://127.0.0.1:6443",
		CAFingerprint: fingerprint,
	}
	writeIdentityForTest(t, paths, identity)
	runner := ownershipOKRunner(t, identity, ca, func(command Command) (Result, error) {
		want := []string{"config", "set-context", "--current", "--namespace", "default"}
		if command.Name == "kubectl" && slices.Equal(command.Args, want) {
			return Result{}, nil
		}
		return Result{}, fmt.Errorf("unexpected command: %s %v", command.Name, command.Args)
	})

	if err := NewManagerWithPaths(runner, paths).SetContextNamespace(context.Background(), "default"); err != nil {
		t.Fatalf("SetContextNamespace() error = %v", err)
	}
}

func TestSetContextNamespaceRejectsUnscopedNamespace(t *testing.T) {
	err := NewManagerWithPaths(&fakeRunner{}, PathsForDirectory(t.TempDir())).SetContextNamespace(context.Background(), "kube-system")
	if err == nil || !strings.Contains(err.Error(), "kubecrypt-") {
		t.Fatalf("SetContextNamespace() error = %v, want kubecrypt-* refusal", err)
	}
}

func TestWipeWorkspaceDeletesKubecryptNamespace(t *testing.T) {
	paths := PathsForDirectory(t.TempDir())
	ca, fingerprint := testCA(t)
	identity := Identity{
		ClusterName:   ClusterName,
		APIServer:     "https://127.0.0.1:6443",
		CAFingerprint: fingerprint,
	}
	writeIdentityForTest(t, paths, identity)
	runner := ownershipOKRunner(t, identity, ca, func(command Command) (Result, error) {
		want := []string{"delete", "namespace", "kubecrypt-foundations", "--ignore-not-found=true", "--wait=true", "--timeout=60s"}
		if command.Name == "kubectl" && slices.Equal(command.Args, want) {
			return Result{}, nil
		}
		return Result{}, fmt.Errorf("unexpected command: %s %v", command.Name, command.Args)
	})

	if err := NewManagerWithPaths(runner, paths).WipeWorkspace(context.Background(), "kubecrypt-foundations"); err != nil {
		t.Fatalf("WipeWorkspace() error = %v", err)
	}
	if got := countCommand(runner.got, "kubectl", "delete", "namespace", "kubecrypt-foundations", "--ignore-not-found=true", "--wait=true", "--timeout=60s"); got != 1 {
		t.Fatalf("namespace delete called %d times, want 1", got)
	}
}

func TestWipeWorkspaceRejectsEmptyNamespace(t *testing.T) {
	err := NewManagerWithPaths(&fakeRunner{}, PathsForDirectory(t.TempDir())).WipeWorkspace(context.Background(), "  ")
	if err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("WipeWorkspace() error = %v, want empty-namespace refusal", err)
	}
}

func TestWipeWorkspaceRejectsUnscopedNamespace(t *testing.T) {
	paths := PathsForDirectory(t.TempDir())
	runner := &fakeRunner{run: func(command Command) (Result, error) {
		return Result{}, fmt.Errorf("unexpected command: %s %v", command.Name, command.Args)
	}}
	err := NewManagerWithPaths(runner, paths).WipeWorkspace(context.Background(), "kube-system")
	if err == nil || !strings.Contains(err.Error(), "kubecrypt-") {
		t.Fatalf("WipeWorkspace() error = %v, want kubecrypt-* refusal", err)
	}
	if len(runner.got) != 0 {
		t.Fatalf("commands = %d, want none after namespace refusal", len(runner.got))
	}
}

func TestWipeWorkspaceRejectsUnrelatedCurrentContext(t *testing.T) {
	paths := PathsForDirectory(t.TempDir())
	_, fingerprint := testCA(t)
	writeIdentityForTest(t, paths, Identity{
		ClusterName:   ClusterName,
		APIServer:     "https://127.0.0.1:6443",
		CAFingerprint: fingerprint,
	})
	runner := &fakeRunner{run: func(command Command) (Result, error) {
		switch {
		case command.Name == "kind":
			return Result{Stdout: ClusterName + "\n"}, nil
		case command.Name == "kubectl" && slices.Equal(command.Args, []string{"config", "current-context"}):
			return Result{Stdout: "production\n"}, nil
		default:
			return Result{}, fmt.Errorf("mutation or unexpected command reached: %s %v", command.Name, command.Args)
		}
	}}

	err := NewManagerWithPaths(runner, paths).WipeWorkspace(context.Background(), "kubecrypt-foundations")
	if !errors.Is(err, ErrOwnershipMismatch) {
		t.Fatalf("WipeWorkspace() error = %v, want ErrOwnershipMismatch", err)
	}
	if got := countCommand(runner.got, "kubectl", "delete", "namespace", "kubecrypt-foundations", "--ignore-not-found=true", "--wait=true", "--timeout=60s"); got != 0 {
		t.Fatalf("namespace delete called %d times after ownership rejection", got)
	}
}

func TestDestroyIsIdempotentWhenClusterIsAbsent(t *testing.T) {
	paths := PathsForDirectory(t.TempDir())
	for _, filename := range []string{paths.Ownership, paths.Kubeconfig, paths.KindConfig} {
		if err := os.WriteFile(filename, []byte("stale"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	runner := &fakeRunner{run: func(command Command) (Result, error) {
		if command.Name == "kind" && strings.Join(command.Args, " ") == "get clusters" {
			return Result{}, nil
		}
		return Result{}, fmt.Errorf("unexpected command: %s %v", command.Name, command.Args)
	}}

	manager := NewManagerWithPaths(runner, paths)
	if err := manager.Destroy(context.Background()); err != nil {
		t.Fatalf("first Destroy() error = %v", err)
	}
	if err := manager.Destroy(context.Background()); err != nil {
		t.Fatalf("second Destroy() error = %v", err)
	}
	for _, filename := range []string{paths.Ownership, paths.Kubeconfig, paths.KindConfig} {
		if _, err := os.Stat(filename); !os.IsNotExist(err) {
			t.Fatalf("%s still exists or returned unexpected error: %v", filename, err)
		}
	}
}

func testCA(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "kubecrypt-test-ca"},
		NotBefore:             time.Unix(0, 0),
		NotAfter:              time.Unix(4102444800, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	pemData := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw})
	sum := sha256ForTest(raw)
	return base64.StdEncoding.EncodeToString(pemData), sum
}

func sha256ForTest(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}

func ownershipOKRunner(_ *testing.T, identity Identity, ca string, extra func(Command) (Result, error)) *fakeRunner {
	runner := &fakeRunner{}
	runner.run = func(command Command) (Result, error) {
		joined := strings.Join(command.Args, " ")
		switch {
		case command.Name == "kind":
			return Result{Stdout: ClusterName + "\n"}, nil
		case command.Name == "kubectl" && joined == "config current-context":
			return Result{Stdout: ContextName}, nil
		case strings.Contains(joined, ".cluster.server"):
			return Result{Stdout: identity.APIServer}, nil
		case strings.Contains(joined, "certificate-authority-data"):
			return Result{Stdout: ca}, nil
		case strings.HasPrefix(joined, "get configmap"):
			data, _ := json.Marshal(map[string]any{"data": map[string]string{
				"clusterName": identity.ClusterName,
				"apiServer":   identity.APIServer,
				"caSHA256":    identity.CAFingerprint,
			}})
			return Result{Stdout: string(data)}, nil
		default:
			if extra != nil {
				return extra(command)
			}
			return Result{}, fmt.Errorf("unexpected command: %s %v", command.Name, command.Args)
		}
	}
	return runner
}

func writeIdentityForTest(t *testing.T, paths Paths, identity Identity) {
	t.Helper()
	if err := paths.ensureDirectory(); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONAtomic(paths.Ownership, identity, 0o600); err != nil {
		t.Fatal(err)
	}
}

func countCommand(commands []Command, name string, args ...string) int {
	count := 0
	for _, command := range commands {
		if command.Name == name && slices.Equal(command.Args, args) {
			count++
		}
	}
	return count
}

func containsArgs(args []string, pair ...string) bool {
	for i := 0; i+len(pair) <= len(args); i++ {
		if slices.Equal(args[i:i+len(pair)], pair) {
			return true
		}
	}
	return false
}
