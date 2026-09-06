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
		checkCommand(ctx, runner, "Docker daemon", Command{Name: "docker", Args: []string{"info"}},
			"Start Docker Desktop, Colima, OrbStack, or another Docker-compatible daemon, then verify `docker info` succeeds."),
		d.checkManagedTool(ctx, runner, "kind", KindVersion, d.Paths.KindBinary()),
		d.checkManagedTool(ctx, runner, "kubectl", KubectlVersion, d.Paths.KubectlBinary()),
		d.checkConfigDirectory(),
	)
	return results
}

func checkOS(goos string) CheckResult {
	if goos == "darwin" || goos == "linux" {
		return CheckResult{Name: "Operating system", OK: true, Detail: goos}
	}
	return CheckResult{
		Name:        "Operating system",
		Detail:      goos,
		Remediation: "Run KubeCrypt on macOS or Linux.",
	}
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
	return CheckResult{Name: name, OK: true, Detail: detail}
}

func (d *Doctor) checkManagedTool(ctx context.Context, runner Runner, name, version, path string) CheckResult {
	if !fileExists(path) {
		return CheckResult{
			Name:   name,
			OK:     true,
			Detail: fmt.Sprintf("not installed yet; start will download %s %s", name, version),
		}
	}
	args := []string{"version"}
	if name == "kubectl" {
		args = []string{"version", "--client"}
	}
	result := checkCommand(ctx, runner, name, Command{Name: path, Args: args},
		fmt.Sprintf("Remove %q and run `kubecrypt start` to reinstall %s %s.", path, name, version))
	if result.OK && !strings.Contains(result.Detail, version) {
		result.OK = false
		result.Remediation = fmt.Sprintf("Remove %q and run `kubecrypt start` to install %s %s.", path, name, version)
	}
	return result
}

func (d *Doctor) checkConfigDirectory() CheckResult {
	if err := d.Paths.ensureDirectory(); err != nil {
		return CheckResult{
			Name:        "Config directory",
			Detail:      err.Error(),
			Remediation: fmt.Sprintf("Ensure %q exists and is writable by the current user.", d.Paths.Directory),
		}
	}
	file, err := os.CreateTemp(d.Paths.Directory, ".writable-*")
	if err != nil {
		return CheckResult{
			Name:        "Config directory",
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
			Detail:      fmt.Sprintf("temporary-file cleanup failed: close=%v remove=%v", closeErr, removeErr),
			Remediation: fmt.Sprintf("Check permissions and available space for %q.", d.Paths.Directory),
		}
	}
	return CheckResult{Name: "Config directory", OK: true, Detail: d.Paths.Directory}
}
