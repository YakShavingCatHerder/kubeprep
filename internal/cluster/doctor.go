package cluster

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// CheckResult is one prerequisite check produced by Doctor.
type CheckResult struct {
	Name        string
	Checking    string
	Status      string
	OK          bool
	Detail      string
	Remediation string
}

// Doctor checks whether this host can run the training cluster.
type Doctor struct {
	Runner Runner
	Paths  Paths
	GOOS   string
}

// NewDoctor creates a Doctor using production defaults.
func NewDoctor(runner Runner) (*Doctor, error) {
	paths, err := DefaultPaths()
	if err != nil {
		return nil, err
	}
	return &Doctor{Runner: runner, Paths: paths, GOOS: runtime.GOOS}, nil
}

// Check runs every prerequisite check and returns all results, including
// failures, so callers can present complete remediation guidance.
func (d *Doctor) Check(ctx context.Context) []CheckResult {
	runner := d.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	goos := d.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}

	results := []CheckResult{checkOS(goos)}
	results = append(results,
		checkDocker(ctx, runner),
		d.checkManagedTool(ctx, runner, "kind", KindVersion, d.Paths.KindBinary()),
		d.checkManagedTool(ctx, runner, "kubectl", KubectlVersion, d.Paths.KubectlBinary()),
		d.checkConfigDirectory(),
	)
	return results
}

func checkOS(goos string) CheckResult {
	checking := "checking operating system"
	if goos == "darwin" || goos == "linux" {
		return CheckResult{
			Name:     "Operating system",
			Checking: checking,
			Status:   "operating system is " + goos,
			OK:       true,
			Detail:   goos,
		}
	}
	return CheckResult{
		Name:        "Operating system",
		Checking:    checking,
		Status:      "operating system " + goos + " is unsupported",
		Detail:      goos,
		Remediation: "Run KubePrep on macOS or Linux.",
	}
}

func checkDocker(ctx context.Context, runner Runner) CheckResult {
	const (
		checking    = "checking docker daemon"
		okStatus    = "docker daemon is reachable"
		failStatus  = "docker daemon is not reachable"
		remediation = "Start Docker Desktop, Colima, OrbStack, or another Docker-compatible daemon, then verify `docker info` succeeds."
	)
	result := checkCommand(ctx, runner, "Docker daemon", Command{Name: "docker", Args: []string{"info"}}, remediation)
	result.Checking = checking
	if result.OK {
		result.Status = okStatus
		result.Detail = summarizeDockerInfo(result.Detail)
		return result
	}
	result.Status = failStatus
	return result
}

func checkCommand(ctx context.Context, runner Runner, name string, command Command, remediation string) CheckResult {
	result, err := runner.Run(ctx, command)
	if err != nil {
		return CheckResult{
			Name:        name,
			Detail:      commandError(command, result, err).Error(),
			Remediation: remediation,
		}
	}
	detail := trimOutput(result.Stdout)
	if detail == "" {
		detail = "available"
	}
	return CheckResult{Name: name, OK: true, Detail: firstLines(detail, 8)}
}

func (d *Doctor) checkManagedTool(ctx context.Context, runner Runner, name, version, path string) CheckResult {
	checking := fmt.Sprintf("checking for %s version %s", name, version)
	installed := fmt.Sprintf("%s version %s installed", name, version)
	missing := fmt.Sprintf("%s version %s is not installed", name, version)
	unpinned := fmt.Sprintf("%s version %s is not pinned", name, version)
	if !fileExists(path) {
		return CheckResult{
			Name:        name,
			Checking:    checking,
			Status:      missing,
			Detail:      fmt.Sprintf("%s %s is not installed", name, version),
			Remediation: fmt.Sprintf("Run `kubeprep doctor` to install %s %s.", name, version),
		}
	}
	args := []string{"version"}
	if name == "kubectl" {
		args = []string{"version", "--client"}
	}
	result := checkCommand(ctx, runner, name, Command{Name: path, Args: args},
		fmt.Sprintf("Remove %q and run `kubeprep doctor` to reinstall %s %s.", path, name, version))
	result.Checking = checking
	if result.OK && !strings.Contains(result.Detail, version) {
		result.OK = false
		result.Status = unpinned
		result.Remediation = fmt.Sprintf("Remove %q and run `kubeprep doctor` to install %s %s.", path, name, version)
		return result
	}
	if result.OK {
		result.Status = installed
		return result
	}
	result.Status = missing
	return result
}

func (d *Doctor) checkConfigDirectory() CheckResult {
	const checking = "checking config directory"
	if err := d.Paths.ensureDirectory(); err != nil {
		return CheckResult{
			Name:        "Config directory",
			Checking:    checking,
			Status:      "config directory is not writable",
			Detail:      err.Error(),
			Remediation: fmt.Sprintf("Ensure %q exists and is writable by the current user.", d.Paths.Directory),
		}
	}
	file, err := os.CreateTemp(d.Paths.Directory, ".writable-*")
	if err != nil {
		return CheckResult{
			Name:        "Config directory",
			Checking:    checking,
			Status:      "config directory is not writable",
			Detail:      err.Error(),
			Remediation: fmt.Sprintf("Grant the current user write access to %q.", d.Paths.Directory),
		}
	}
	name := file.Name()
	closeErr := file.Close()
	removeErr := os.Remove(name)
	if closeErr != nil || removeErr != nil {
		return CheckResult{
			Name:        "Config directory",
			Checking:    checking,
			Status:      "config directory is not writable",
			Detail:      fmt.Sprintf("temporary-file cleanup failed: close=%v remove=%v", closeErr, removeErr),
			Remediation: fmt.Sprintf("Check permissions and available space for %q.", d.Paths.Directory),
		}
	}
	return CheckResult{
		Name:     "Config directory",
		Checking: checking,
		Status:   "config directory is writable",
		OK:       true,
		Detail:   d.Paths.Directory,
	}
}

func summarizeDockerInfo(output string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Server Version:") {
			return trimmed
		}
	}
	for _, line := range strings.Split(output, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return "daemon responded"
}

func firstLines(output string, n int) string {
	lines := strings.Split(output, "\n")
	if len(lines) <= n {
		return output
	}
	return strings.Join(lines[:n], "\n")
}
