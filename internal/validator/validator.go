// Package validator evaluates observable Kubernetes state without considering
// how that state was produced.
package validator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Status describes whether an observed state is definitely incorrect, still
// reconciling, or satisfies a check.
type Status uint8

const (
	Wrong Status = iota
	Converging
	Success
)

func (s Status) String() string {
	switch s {
	case Wrong:
		return "wrong"
	case Converging:
		return "converging"
	case Success:
		return "success"
	default:
		return "unknown"
	}
}

// Result is the explainable outcome of a check.
type Result struct {
	Status  Status
	Message string
}

// Check evaluates Kubernetes state through a Runner.
type Check interface {
	Evaluate(context.Context, Runner) (Result, error)
}

// CheckFunc adapts a function into a Check.
type CheckFunc func(context.Context, Runner) (Result, error)

func (f CheckFunc) Evaluate(ctx context.Context, runner Runner) (Result, error) {
	return f(ctx, runner)
}

// All composes checks. It evaluates every check and returns the most severe
// status (wrong before converging before success), preserving every message.
func All(checks ...Check) Check {
	copied := append([]Check(nil), checks...)
	return CheckFunc(func(ctx context.Context, runner Runner) (Result, error) {
		if len(copied) == 0 {
			return Result{Status: Success, Message: "all checks passed"}, nil
		}

		status := Success
		messages := make([]string, 0, len(copied))
		for i, check := range copied {
			if check == nil {
				return Result{Status: Wrong, Message: fmt.Sprintf("check %d is nil", i+1)}, errors.New("validator: nil check")
			}
			result, err := check.Evaluate(ctx, runner)
			if err != nil {
				return result, fmt.Errorf("validator: check %d: %w", i+1, err)
			}
			if result.Status < status {
				status = result.Status
			}
			if result.Message != "" {
				messages = append(messages, result.Message)
			}
		}

		return Result{Status: status, Message: strings.Join(messages, "; ")}, nil
	})
}

// Poll evaluates check immediately and then at interval while its result is
// Converging. Wrong and Success are terminal. The context provides the bound.
func Poll(ctx context.Context, runner Runner, check Check, interval time.Duration) (Result, error) {
	if runner == nil {
		return Result{Status: Wrong, Message: "runner is nil"}, errors.New("validator: nil runner")
	}
	if check == nil {
		return Result{Status: Wrong, Message: "check is nil"}, errors.New("validator: nil check")
	}
	if interval <= 0 {
		return Result{Status: Wrong, Message: "poll interval must be positive"}, errors.New("validator: non-positive poll interval")
	}

	var last Result
	for {
		result, err := check.Evaluate(ctx, runner)
		last = result
		if err != nil {
			return result, err
		}
		if result.Status != Converging {
			return result, nil
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return last, ctx.Err()
		case <-timer.C:
		}
	}
}
