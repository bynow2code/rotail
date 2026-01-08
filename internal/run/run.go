package run

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bynow2code/rotail/internal/tailer"
)

func Run(cfg *Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	done := make(chan struct{})
	errors := make(chan error, 1)

	go func() {
		defer close(done)

		var err error
		if cfg.File != "" {
			err = tailer.RunFileTailer(ctx, cfg.File)
		} else {
			err = tailer.RunDirTailer(ctx, cfg.Dir, cfg.Extensions)
		}

		if err != nil {
			select {
			case errors <- err:
			default:
			}
		}
	}()

	var runErr error
	select {
	case <-ctx.Done():
		fmt.Println("ℹ️ Received stop signal, shutting down...")
	case runErr = <-errors:
		fmt.Fprintf(os.Stderr, "❌ Exiting due to error: %v\n", runErr)
		stop()
	case <-done:
		return nil
	}

	select {
	case <-done:
		if runErr == nil {
			fmt.Println("✅ Graceful shutdown completed.")
		}
	case <-time.After(5 * time.Second):
		fmt.Println("⚠️ Shutdown timed out, forcing exit.")
	}

	return nil
}
