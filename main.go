package main

import (
	"fmt"
	"os"

	"github.com/bynow2code/rotail/internal/run"
)

func main() {
	cfg, err := run.ParseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid command-line arguments: %v\n", err)
		os.Exit(1)
	}

	if err := run.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "rotail exited due to an error: %v\n", err)
		os.Exit(1)
	}
}
