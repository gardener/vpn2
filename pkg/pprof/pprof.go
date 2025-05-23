// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pprof

import (
	"context"
	"errors"
	"net/http"
	_ "net/http/pprof" // #nosec: G108 -- default http mux is only used when profiling is enable, only other server (exporter) uses separate http mux.
	"time"

	"github.com/go-logr/logr"
)

// Serve starts a new http server serving pprof endpoints on :6060
func Serve(ctx context.Context, log logr.Logger) {
	server := &http.Server{
		Addr:              ":6060",
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      10 * time.Second,
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
