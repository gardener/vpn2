// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"

	"github.com/caarlos0/env/v10"
	"github.com/go-logr/logr"

	"github.com/gardener/vpn2/pkg/network"
)

type VPNServer struct {
	ServiceNetworks []network.CIDR `env:"SERVICE_NETWORKS" envDefault:"100.64.0.0/13"`
	PodNetworks     []network.CIDR `env:"POD_NETWORKS" envDefault:"100.96.0.0/11"`
	NodeNetworks    []network.CIDR `env:"NODE_NETWORKS"`
	VPNNetwork      network.CIDR   `env:"VPN_NETWORK"`
	SeedPodNetwork  network.CIDR   `env:"SEED_POD_NETWORK"`
	PodName         string         `env:"POD_NAME"`
	StatusPath      string         `env:"OPENVPN_STATUS_PATH"`
	IsHA            bool           `env:"IS_HA"`
	HAVPNClients    int            `env:"HA_VPN_CLIENTS"`
	LocalNodeIP     string         `env:"LOCAL_NODE_IP" envDefault:"255.255.255.255"`
}

func GetVPNServerConfig(log logr.Logger) (VPNServer, error) {
	cfg := VPNServer{}
	if err := env.Parse(&cfg); err != nil {
		return cfg, err
	}
	if cfg.VPNNetwork.String() == "" {
		var err error
		cfg.VPNNetwork, err = getVPNNetworkDefault()
		if err != nil {
			return VPNServer{}, err
		}
	}
	if err := validateVPNNetworkCIDR(cfg.VPNNetwork); err != nil {
		return VPNServer{}, err
	}

	if cfg.IsHA {
		if cfg.PodName == "" {
			return VPNServer{}, fmt.Errorf("IS_HA is set to true but POD_NAME is not set")
		}
		if cfg.HAVPNClients <= 0 {
			return VPNServer{}, fmt.Errorf("IS_HA is set to true but HA_VPN_CLIENTS is not set or invalid")
		}
		if cfg.StatusPath == "" {
			return VPNServer{}, fmt.Errorf("IS_HA is set to true but OPENVPN_STATUS_PATH is not set")
		}
	}

	log.Info("config parsed", "config", cfg)
	return cfg, nil
}
