package main

import (
	"fmt"
	"os"

	"github.com/YakShavingCatHerder/kubecrypt/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "kubecrypt:", err)
		os.Exit(1)
	}
}
