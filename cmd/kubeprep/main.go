package main

import (
	"fmt"
	"os"

	"github.com/YakShavingCatHerder/kubeprep/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "kubeprep:", err)
		os.Exit(1)
	}
}
