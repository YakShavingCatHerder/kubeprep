package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/YakShavingCatHerder/kubeprep/internal/cluster"
)

func TestWriteDoctorReportSectionsEndWithStatus(t *testing.T) {
	var out bytes.Buffer
	failed := writeDoctorReport(&out, []cluster.CheckResult{
		{
			Checking: "checking for kind version v0.32.0",
			Status:   "kind version v0.32.0 installed",
			OK:       true,
			Detail:   "kind v0.32.0 go1.24",
		},
		{
			Checking:    "checking docker daemon",
			Status:      "docker daemon is not reachable",
			Detail:      "Cannot connect to daemon",
			Remediation: "Start Docker Desktop.",
		},
	})
	if !failed {
		t.Fatal("expected a failed check")
	}
	got := out.String()
	blocks := strings.Split(strings.TrimSpace(got), "\n\n")
	if len(blocks) != 2 {
		t.Fatalf("sections = %d, want 2:\n%s", len(blocks), got)
	}
	kindLines := strings.Split(blocks[0], "\n")
	if kindLines[0] != "checking for kind version v0.32.0" {
		t.Fatalf("kind header = %q", kindLines[0])
	}
	if kindLines[1] != "  kind v0.32.0 go1.24" {
		t.Fatalf("kind detail = %q", kindLines[1])
	}
	if kindLines[len(kindLines)-1] != "✓ kind version v0.32.0 installed" {
		t.Fatalf("kind status = %q", kindLines[len(kindLines)-1])
	}
	dockerLines := strings.Split(blocks[1], "\n")
	if dockerLines[0] != "checking docker daemon" {
		t.Fatalf("docker header = %q", dockerLines[0])
	}
	if dockerLines[len(dockerLines)-1] != "✗ docker daemon is not reachable" {
		t.Fatalf("docker status = %q", dockerLines[len(dockerLines)-1])
	}
	if !strings.Contains(blocks[1], "  Cannot connect to daemon") {
		t.Fatalf("docker detail missing:\n%s", blocks[1])
	}
	if strings.Contains(got, "[ok]") || strings.Contains(got, "[fail]") {
		t.Fatalf("legacy markers still present:\n%s", got)
	}
}
