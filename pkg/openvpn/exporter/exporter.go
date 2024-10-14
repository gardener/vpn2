/*
 * SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package exporter

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kumina/openvpn_exporter/exporters"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Config is the configuration of the OpenVPN metrics exporter.
type Config struct {
	// ListenAddress is the address to listen on for web interface and telemetry.
	ListenAddress string
	// MetricsPath is the path under which to expose metrics.
	MetricsPath string
	// OpenvpnStatusPaths are the paths at which OpenVPN places its status files.
	OpenvpnStatusPaths string
	// IgnoreIndividuals if true ignores metrics for individuals.
	IgnoreIndividuals bool
}

// NewDefaultConfig creates Config with default values.
func NewDefaultConfig() Config {
	return Config{
		ListenAddress:      ":9176",
		MetricsPath:        "/metrics",
		OpenvpnStatusPaths: "openvpn.status",
		IgnoreIndividuals:  false,
	}
}

// Start listens and serves the metrics service.
func Start(log logr.Logger, cfg Config) error {
	log.Info("Starting OpenVPN Exporter")
	log.Info(fmt.Sprintf("OpenVPN Exporter Configuration: %+v", cfg))

	exporter, err := exporters.NewOpenVPNExporter(strings.Split(cfg.OpenvpnStatusPaths, ","), cfg.IgnoreIndividuals)
	if err != nil {
		return err
	}
	if err := prometheus.Register(exporter); err != nil {
		return err
	}

	http.Handle(cfg.MetricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
			<html>
			<head><title>OpenVPN Exporter</title></head>
			<body>
			<h1>OpenVPN Exporter</h1>
			<p><a href='` + cfg.MetricsPath + `'>Metrics</a></p>
			</body>
			</html>`))
	})

	return (&http.Server{
		Addr:         cfg.ListenAddress,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}).ListenAndServe()
}
