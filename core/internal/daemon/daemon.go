// Package daemon runs the long-lived core: the OTLP receiver (sole writer to the store) plus
// the ring-buffer prune loop. CLI/TUI/MCP are separate read clients over the same DB file.
package daemon

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/SrinjoyDev/rewynd/internal/config"
	"github.com/SrinjoyDev/rewynd/internal/otlp"
	"github.com/SrinjoyDev/rewynd/internal/store"
)

type Options struct {
	Addr        string
	DBPath      string
	MaxRequests int
}

func (o *Options) withDefaults() {
	if o.Addr == "" {
		o.Addr = config.DefaultOTLPAddr
	}
	if o.DBPath == "" {
		o.DBPath = config.DBPath()
	}
	if o.MaxRequests == 0 {
		o.MaxRequests = config.MaxRequests()
	}
}

// Run starts the receiver and blocks until ctx is cancelled or the server errors.
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

	srv := &http.Server{Addr: opts.Addr, Handler: otlp.NewReceiver(st).Handler()}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	go pruneLoop(ctx, st, opts.MaxRequests)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
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
