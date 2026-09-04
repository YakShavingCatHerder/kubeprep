package main

import (
	"fmt"
	"os"

	"github.com/elongmusty/kubecrypt/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "kubecrypt:", err)
		os.Exit(1)
	}
}
