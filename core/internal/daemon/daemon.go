// Package daemon runs the long-lived core: the OTLP receiver (sole writer to the store) plus
// the ring-buffer prune loop. CLI/TUI/MCP are separate read clients over the same DB file.
package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/SrinjoyDev/rewynd/core/internal/config"
	"github.com/SrinjoyDev/rewynd/core/internal/otlp"
	"github.com/SrinjoyDev/rewynd/core/internal/store"
)

type Options struct {
	Addr        string // OTLP/HTTP listen address
	GRPCAddr    string // OTLP/gRPC listen address
	DBPath      string
	MaxRequests int
}

func (o *Options) withDefaults() {
	if o.Addr == "" {
		o.Addr = config.DefaultOTLPAddr
	}
	if o.GRPCAddr == "" {
		o.GRPCAddr = config.DefaultOTLPGRPCAddr
	}
	if o.DBPath == "" {
		o.DBPath = config.DBPath()
	}
	if o.MaxRequests == 0 {
		o.MaxRequests = config.MaxRequests()
	}
}

// Run starts the OTLP/HTTP and OTLP/gRPC receivers and blocks until ctx is cancelled or a
// server errors. gRPC is best-effort: if its port is taken (e.g. another collector), the core
// still serves HTTP rather than refusing to start.
func Run(ctx context.Context, opts Options) error {
	opts.withDefaults()
	if err := os.MkdirAll(filepath.Dir(opts.DBPath), 0o755); err != nil {
		return err
	}
	st, err := store.Open(opts.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()

	go pruneLoop(ctx, st, opts.MaxRequests)

	httpSrv := &http.Server{Addr: opts.Addr, Handler: otlp.NewReceiver(st).Handler()}
	errc := make(chan error, 2)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errc <- err
		}
	}()

	grpcSrv := otlp.NewGRPCServer(st)
	if ln, err := net.Listen("tcp", opts.GRPCAddr); err != nil {
		fmt.Fprintf(os.Stderr, "rewynd: OTLP/gRPC disabled (%s unavailable: %v); HTTP only\n", opts.GRPCAddr, err)
		grpcSrv = nil
	} else {
		go func() {
			if err := grpcSrv.Serve(ln); err != nil {
				errc <- err
			}
		}()
	}

	select {
	case <-ctx.Done():
	case err = <-errc:
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	if grpcSrv != nil {
		stopGRPC(grpcSrv)
	}
	return err
}

// stopGRPC drains in-flight exports, but never hangs shutdown waiting on a stuck stream.
func stopGRPC(s interface {
	GracefulStop()
	Stop()
}) {
	done := make(chan struct{})
	go func() { s.GracefulStop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		s.Stop()
	}
}

func pruneLoop(ctx context.Context, st *store.Store, max int) {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = st.Prune(max)
		}
	}
}
