package main

import (
	"fmt"
	"os"

	"github.com/bynow2code/rotail/internal/run"
)

func main() {
	cfg, err := run.ParseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid flags: %v\n", err)
		os.Exit(1)
	}

	if err := run.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "service exited with error: %v\n", err)
		os.Exit(1)
	}
}
