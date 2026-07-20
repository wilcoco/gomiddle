// Command server is the gomiddle entry point: it wires config, the silo
// poller, and the HTTP API together, then runs until interrupted.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wilcoco/gomiddle/internal/api"
	"github.com/wilcoco/gomiddle/internal/config"
	"github.com/wilcoco/gomiddle/internal/silo"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	// ctx is cancelled when the process receives Ctrl-C or SIGTERM,
	// which tells every goroutine to shut down cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	poller := silo.NewPoller(cfg, log)
	go poller.Run(ctx)

	srv := api.New(cfg.HTTPAddr, poller, log)
	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr, "mock_plc", cfg.MockPLC)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	// Give in-flight HTTP requests up to 5 seconds to finish.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "err", err)
	}
}
