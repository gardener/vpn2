package pprof

import (
	"context"
	"errors"
	"net/http"
	_ "net/http/pprof"

	"github.com/go-logr/logr"
)

// Serve starts a new http server serving pprof endpoints on :6060
func Serve(ctx context.Context, log logr.Logger) {
	server := &http.Server{
		Addr: ":6060",
	}
	go func() {
		log.Info("serving on :6060")
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Error(err, "HTTP server error")
		}
	}()
	<-ctx.Done()
	if err := server.Shutdown(ctx); err != nil {
		log.Error(err, "HTTP shutdown error: %v")
	}
}
