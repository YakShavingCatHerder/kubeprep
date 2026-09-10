package terminal

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// ShellSession describes the environment handed to a learner's real shell.
type ShellSession struct {
	Kubeconfig string
	Namespace  string
	ScenarioID string
	Objective  string
	ToolBinDir string
}

// ShellRunner builds a real learner shell. The scenario view hosts it in a
// PTY pane and does not inspect or record commands.
type ShellRunner struct {
	In         io.Reader
	Out        io.Writer
	Err        io.Writer
	LookupEnv  func(string) (string, bool)
	Environ    func() []string
	Executable func() (string, error)
}

func NewShellRunner() *ShellRunner {
	return &ShellRunner{
		In:         os.Stdin,
		Out:        os.Stdout,
		Err:        os.Stderr,
		LookupEnv:  os.LookupEnv,
		Environ:    os.Environ,
		Executable: os.Executable,
	}
}

// Command builds the user's preferred shell with KubePrep-scoped environment.
// Stdin, stdout, and stderr are left unset so a PTY can attach them.
func (r *ShellRunner) Command(ctx context.Context, session ShellSession) (*exec.Cmd, error) {
	shell := "/bin/sh"
	if configured, ok := r.LookupEnv("SHELL"); ok && configured != "" {
		shell = configured
	}

	executable, err := r.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve kubeprep executable: %w", err)
	}

	cmd := exec.CommandContext(ctx, shell)
	env := map[string]string{
		"TERM":                "xterm-256color",
		"KUBECONFIG":          session.Kubeconfig,
		"KUBEPREP_SCENARIO":   session.ScenarioID,
		"KUBEPREP_NAMESPACE":  session.Namespace,
		"KUBEPREP_EXECUTABLE": executable,
		"KUBEPREP_LAB_SHELL":  "1",
		"KUBEPREP_OBJECTIVE":  session.Objective,
		"KUBEPREP_SHELL":      shell,
	}
	if session.ToolBinDir != "" {
		current := "/usr/bin:/bin"
		if value, ok := r.LookupEnv("PATH"); ok && value != "" {
			current = value
		}
		env["PATH"] = session.ToolBinDir + string(os.PathListSeparator) + current
	}
	cmd.Env = scopedEnvironment(r.Environ(), env)
	return cmd, nil
}

// Run starts the child shell directly.
func (r *ShellRunner) Run(ctx context.Context, session ShellSession) error {
	cmd, err := r.Command(ctx, session)
	if err != nil {
		return err
	}
	cmd.Stdin = r.In
	cmd.Stdout = r.Out
	cmd.Stderr = r.Err
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Lab Shell exited: %w", err)
	}
	return nil
}

func scopedEnvironment(base []string, values map[string]string) []string {
	blocked := make(map[string]struct{}, len(values))
	for key := range values {
		blocked[key] = struct{}{}
	}

	result := make([]string, 0, len(base)+len(values))
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if _, replace := blocked[key]; !replace {
			result = append(result, entry)
		}
	}
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}
