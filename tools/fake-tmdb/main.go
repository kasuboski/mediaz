// fake-tmdb serves the embedded deterministic metadata fixture catalog.
package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kasuboski/mediaz/internal/testsupport/faketmdb"
)

const shutdownTimeout = 5 * time.Second

func run(ctx context.Context, listener net.Listener) error {
	server := &http.Server{Handler: faketmdb.New()}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return err
		}
		err := <-errCh
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func main() {
	addr := os.Getenv("FAKE_TMDB_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8765"
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("fake TMDB listening at http://%s (API base URI; client appends /3)", listener.Addr())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, listener); err != nil {
		log.Printf("fake TMDB server: %v", err)
		os.Exit(1)
	}
}
