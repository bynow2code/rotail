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

	var hadError bool
	select {
	case <-sigChan:
		fmt.Println("ℹ️ Received stop signal, shutting down...")
	case err := <-errors:
		hadError = true
		fmt.Fprintf(os.Stderr, "❌ Exiting due to error: %v\n", err)
	}

	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if !hadError {
			fmt.Println("✅ Graceful shutdown completed.")
		}
	case <-time.After(5 * time.Second):
		fmt.Println("⚠️ Shutdown timed out, forcing exit.")
	}

	return nil
}
