// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/go-logr/logr"

	"github.com/gardener/vpn2/pkg/constants"
)

type TunnelController struct {
	HAVPNClients       int `env:"HA_VPN_CLIENTS"`
	WatchdogWindowSize int `env:"WATCHDOG_WINDOW_SIZE"`
	WatchdogThreshold  int `env:"WATCHDOG_THRESHOLD"`
	WatchdogCooldown   int `env:"WATCHDOG_COOLDOWN"`
}

var DefaultTunnelControllerConfig = &TunnelController{
	HAVPNClients:       2,
	WatchdogWindowSize: constants.WatchdogWindowSize,
	WatchdogCooldown:   constants.WatchdogCooldown,
	WatchdogThreshold:  constants.WatchdogThreshold,
}

func GetTunnelControllerConfig(log logr.Logger) (*TunnelController, error) {
	cfg := DefaultTunnelControllerConfig
	if err := env.Parse(cfg); err != nil {
		return cfg, err
	}

	log.Info("config parsed", "config", cfg)
	return cfg, nil
}
