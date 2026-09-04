package cluster

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Command describes an external command. Env is added to the current process
// environment, with later values overriding earlier values.
type Command struct {
	Name  string
	Args  []string
	Env   []string
	Stdin []byte
}

// Result contains captured command output.
type Result struct {
	Stdout string
	Stderr string
}

// Runner executes external commands.
type Runner interface {
	Run(context.Context, Command) (Result, error)
}

// ExecRunner executes commands using os/exec.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, command Command) (Result, error) {
	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	cmd.Env = mergeEnv(os.Environ(), command.Env)
	if command.Stdin != nil {
		cmd.Stdin = bytes.NewReader(command.Stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return result, fmt.Errorf("run %s: %w", command.Name, ctxErr)
		}
		return result, fmt.Errorf("run %s: %w", command.Name, err)
	}
	return result, nil
}

func mergeEnv(base, overrides []string) []string {
	positions := make(map[string]int, len(base)+len(overrides))
	merged := make([]string, 0, len(base)+len(overrides))
	add := func(entry string) {
		key := entry
		for i, r := range entry {
			if r == '=' {
				key = entry[:i]
				break
			}
		}
		if position, ok := positions[key]; ok {
			merged[position] = entry
			return
		}
		positions[key] = len(merged)
		merged = append(merged, entry)
	}
	for _, entry := range base {
		add(entry)
	}
	for _, entry := range overrides {
		add(entry)
	}
	return merged
}

func commandError(command Command, result Result, err error) error {
	if err == nil {
		return nil
	}
	detail := result.Stderr
	if detail == "" {
		detail = result.Stdout
	}
	if detail == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, trimOutput(detail))
}

func trimOutput(output string) string {
	const max = 2048
	if len(output) > max {
		output = output[:max] + "..."
	}
	for len(output) > 0 && (output[len(output)-1] == '\n' || output[len(output)-1] == '\r') {
		output = output[:len(output)-1]
	}
	return output
}
