// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/caarlos0/env/v10"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/go-logr/logr"
)

type PathController struct {
	IPFamilies     string       `env:"IP_FAMILIES" envDefault:"IPv4"`
	VPNNetwork     network.CIDR `env:"VPN_NETWORK"`
	HAVPNClients   int          `env:"HA_VPN_CLIENTS"`
	PodNetwork     network.CIDR `env:"POD_NETWORK"`
	NodeNetwork    network.CIDR `env:"NODE_NETWORK"`
	ServiceNetwork network.CIDR `env:"SERVICE_NETWORK"`
}

func GetPathControllerConfig(log logr.Logger) (PathController, error) {
	cfg := PathController{}
	if err := env.Parse(&cfg); err != nil {
		return cfg, err
	}
	if cfg.VPNNetwork.String() == "" {
		var err error
		cfg.VPNNetwork, err = getVPNNetworkDefault()
		if err != nil {
			return PathController{}, err
		}
	}
	if err := network.ValidateCIDR(cfg.VPNNetwork, cfg.IPFamilies); err != nil {
		return PathController{}, err
	}

	log.Info("config parsed", "config", cfg)
	return cfg, nil
}
