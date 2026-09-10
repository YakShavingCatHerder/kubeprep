package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallScriptSelfTest(t *testing.T) {
	script := filepath.Join(repoRoot(t), "scripts", "install.sh")
	out, err := exec.Command("bash", script, "--self-test").CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh --self-test: %v\n%s", err, out)
	}
	if string(out) != "install.sh self-test ok\n" {
		t.Fatalf("unexpected self-test output: %q", out)
	}
}

func TestInstallScriptRunsWhenPipedToBash(t *testing.T) {
	script := filepath.Join(repoRoot(t), "scripts", "install.sh")
	file, err := os.Open(script)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	cmd := exec.Command("bash", "-s", "--", "--self-test")
	cmd.Stdin = file
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cat install.sh | bash -s -- --self-test: %v\n%s", err, out)
	}
	if string(out) != "install.sh self-test ok\n" {
		t.Fatalf("unexpected piped self-test output: %q", out)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}
