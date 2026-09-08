package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/YakShavingCatHerder/kubeprep/internal/releasecheck"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: checkreleasetag <vMAJOR.MINOR.PATCH>\n")
		os.Exit(2)
	}
	candidate := os.Args[1]
	tags, err := gitTags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "checkreleasetag: %v\n", err)
		os.Exit(1)
	}
	if err := releasecheck.Validate(candidate, tags); err != nil {
		fmt.Fprintf(os.Stderr, "checkreleasetag: %v\n", err)
		os.Exit(1)
	}
}

func gitTags() ([]string, error) {
	out, err := exec.Command("git", "tag", "-l", "v*").Output()
	if err != nil {
		return nil, fmt.Errorf("list git tags: %w", err)
	}
	var tags []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			tags = append(tags, line)
		}
	}
	return tags, nil
}
