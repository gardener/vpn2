// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
)

const (
	ReadinessPort       = 8080
	StatusReady         = "ready"
	StatusNotReady      = "not ready"
	StatusOK            = "ok"
	StatusNotOK         = "not ok"
	StatusInternalError = "internal error"
)

// Config is the configuration of the OpenVPN liveness/readiness server.
type Config struct {
	// ListenAddress is the address to listen on for web interface and telemetry.
	ListenAddress string
	// HealthPath is the path under which to expose liveness status.
	HealthPath string
	// ReadinessPath is the path under which to expose readiness status.
	ReadinessPath string
	// OpenVPNStatusPath is the path at which OpenVPN places its status file.
	OpenVPNStatusPath string
	// OpenVPNStatusUpdateInterval is the interval in seconds during which OpenVPN is expected to update its status file.
	OpenVPNStatusUpdateInterval int
	// IsHA indicates whether the OpenVPN server is running in HA mode.
	IsHA bool
}

// NewDefaultConfig creates Config with default values.
func NewDefaultConfig() Config {
	return Config{
		ListenAddress:               fmt.Sprintf(":%d", ReadinessPort),
		HealthPath:                  "/healthz",
		ReadinessPath:               "/readyz",
		OpenVPNStatusPath:           "/srv/status/openvpn.status",
		OpenVPNStatusUpdateInterval: 15,
		IsHA:                        false,
	}
}

// NewReadinessServer returns a new HTTP server that serves liveness and readiness endpoints.
func NewReadinessServer(cfg Config, log logr.Logger) *http.Server {
	log.Info("Starting OpenVPN Readiness/Liveness Server")
	log.Info(fmt.Sprintf("Configuration: %+v", cfg))

	handler := http.NewServeMux()
	handler.HandleFunc(cfg.HealthPath, func(w http.ResponseWriter, r *http.Request) {
		status, err := ParseFile(cfg.OpenVPNStatusPath)
		if err != nil {
			log.Error(err, "failed to parse OpenVPN status file", "path", cfg.OpenVPNStatusPath)
			http.Error(w, StatusInternalError, http.StatusInternalServerError)
			return
		}
		if isUp(status, cfg.OpenVPNStatusUpdateInterval) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(StatusOK))
			return
		}
		http.Error(w, StatusNotOK, http.StatusServiceUnavailable)
	})
	handler.HandleFunc(cfg.ReadinessPath, func(w http.ResponseWriter, r *http.Request) {
		status, err := ParseFile(cfg.OpenVPNStatusPath)
		if err != nil {
			log.Error(err, "failed to parse OpenVPN status file", "path", cfg.OpenVPNStatusPath)
			http.Error(w, StatusInternalError, http.StatusInternalServerError)
			return
		}
		if isReady(status, cfg.IsHA) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(StatusReady))
			return
		}
		http.Error(w, StatusNotReady, http.StatusServiceUnavailable)
	})

	return &http.Server{
		Addr:         cfg.ListenAddress,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
}
