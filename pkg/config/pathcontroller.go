// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"strings"

	"github.com/caarlos0/env/v10"
	"github.com/go-logr/logr"

	"github.com/gardener/vpn2/pkg/network"
)

type PathController struct {
	IPFamilies      string         `env:"IP_FAMILIES" envDefault:"IPv4"`
	VPNNetwork      network.CIDR   `env:"VPN_NETWORK"`
	HAVPNClients    int            `env:"HA_VPN_CLIENTS"`
	ServiceNetworks []network.CIDR `env:"SERVICE_NETWORKS" envDefault:"100.64.0.0/13"`
	PodNetworks     []network.CIDR `env:"POD_NETWORKS" envDefault:"100.96.0.0/11"`
	NodeNetworks    []network.CIDR `env:"NODE_NETWORKS"`
}

func (v PathController) PrimaryIPFamily() string {
	return strings.Split(v.IPFamilies, ",")[0]
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
	if err := validateVPNNetworkCIDR(cfg.VPNNetwork); err != nil {
		return PathController{}, err
	}

	log.Info("config parsed", "config", cfg)
	return cfg, nil
}
