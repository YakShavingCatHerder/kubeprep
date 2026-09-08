package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestWaitForNonTTYPrintsLineThenRuns(t *testing.T) {
	var out bytes.Buffer
	called := false
	err := waitFor(&out, "Preparing training cluster", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("waitFor did not run the work function")
	}
	if got := out.String(); got != "Preparing training cluster...\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestWaitForNonTTYReturnsError(t *testing.T) {
	var out bytes.Buffer
	want := errors.New("kind create failed")
	err := waitFor(&out, "Preparing training cluster", func() error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if !strings.Contains(out.String(), "Preparing training cluster...") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestWaitForNonTTYWaitsForWork(t *testing.T) {
	started := make(chan struct{})
	err := waitFor(&bytes.Buffer{}, "Checking training cluster", func() error {
		close(started)
		time.Sleep(20 * time.Millisecond)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	default:
		t.Fatal("work function was not started")
	}
}
