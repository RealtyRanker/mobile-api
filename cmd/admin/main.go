package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/asmisnik/mobile-api/internal/admin"
	"github.com/asmisnik/mobile-api/internal/db"
)

func main() {
	if err := run(); err != nil {
		slog.Error("web admin", "error", err)
		os.Exit(1)
	}
}

func run() error {
	path := flag.String("db", "/data/realtyranker.db", "path to the RealtyRanker SQLite file")
	listen := flag.String("listen", "127.0.0.1:8098", "HTTP listen address")
	flag.Parse()
	absolute, err := filepath.Abs(*path)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	catalog, err := db.OpenAdmin(ctx, absolute)
	if err != nil {
		return err
	}
	defer catalog.Close()

	server := &http.Server{
		Addr: *listen, Handler: admin.Handler(catalog),
		ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: time.Minute,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			slog.Warn("shutting down admin server", "error", err)
		}
	}()
	slog.Info("web admin listening", "url", "http://"+*listen)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
