package run

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bynow2code/rotail/internal/tailer"
)

func Run(cfg *Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	errors := make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()

		if cfg.File != "" {
			if err := tailer.RunFileTailer(ctx, cfg.File); err != nil {
				select {
				case errors <- err:
				default:
				}
			}
		} else if cfg.Dir != "" {
			if err := tailer.RunDirTailer(ctx, cfg.Dir, cfg.Extensions); err != nil {
				select {
				case errors <- err:
				default:
				}
			}
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		fmt.Println("Rotail received stop signal, shutting down...")
	case err := <-errors:
		fmt.Fprintf(os.Stderr, "Rotail exited due to error: %v\n", err)
	}

	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("Rotail graceful shutdown completed.")
	case <-time.After(5 * time.Second):
		fmt.Println("Rotail shutdown timeout, forcing exit.")
	}

	return nil
}
