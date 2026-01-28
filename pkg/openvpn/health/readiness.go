// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"github.com/go-logr/logr"
)

// Config is the configuration of the OpenVPN liveness/readiness server.
type Config struct {
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
		OpenVPNStatusPath:           "/srv/status/openvpn.status",
		OpenVPNStatusUpdateInterval: 15,
		IsHA:                        false,
	}
}

// IsAlive checks whether the OpenVPN server is alive.
func IsAlive(cfg Config, log logr.Logger) bool {
	status, err := ParseFile(cfg.OpenVPNStatusPath)
	if err != nil {
		log.Error(err, "failed to parse OpenVPN status file", "path", cfg.OpenVPNStatusPath)
		return false
	}
	if isUp(log, status, cfg.OpenVPNStatusUpdateInterval) {
		return true
	}
	return false
}

// IsReady checks whether the OpenVPN server is ready.
func IsReady(cfg Config, log logr.Logger) bool {
	status, err := ParseFile(cfg.OpenVPNStatusPath)
	if err != nil {
		log.Error(err, "failed to parse OpenVPN status file", "path", cfg.OpenVPNStatusPath)
		return false
	}
	if isReady(log, status, cfg.IsHA) {
		return true
	}
	return false
}
