package validator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Runner is the command boundary used by checks. Arguments are kubectl
// arguments, not shell source.
type Runner interface {
	Run(context.Context, ...string) ([]byte, error)
}

// KubectlRunner invokes kubectl with an explicit kubeconfig.
type KubectlRunner struct {
	Kubeconfig string
	Executable string
}

// Run executes kubectl directly. It never invokes a shell and removes any
// inherited KUBECONFIG before adding the configured value.
func (r KubectlRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	if strings.TrimSpace(r.Kubeconfig) == "" {
		return nil, errors.New("validator: kubectl runner requires an explicit kubeconfig")
	}

	executable := r.Executable
	if executable == "" {
		executable = "kubectl"
	}

	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Env = kubeconfigEnv(os.Environ(), r.Kubeconfig)
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			return output, fmt.Errorf("validator: kubectl %s: %w: %s", strings.Join(args, " "), err, detail)
		}
		return output, fmt.Errorf("validator: kubectl %s: %w", strings.Join(args, " "), err)
	}
	return output, nil
}

func kubeconfigEnv(environ []string, kubeconfig string) []string {
	filtered := make([]string, 0, len(environ)+1)
	for _, entry := range environ {
		if !strings.HasPrefix(entry, "KUBECONFIG=") {
			filtered = append(filtered, entry)
		}
	}
	return append(filtered, "KUBECONFIG="+kubeconfig)
}
