// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/caarlos0/env/v10"
	"github.com/go-logr/logr"

	"github.com/gardener/vpn2/pkg/network"
)

type VPNServer struct {
	ServiceNetworks  []network.CIDR `env:"SERVICE_NETWORKS" envDefault:"100.64.0.0/13"`
	PodNetworks      []network.CIDR `env:"POD_NETWORKS" envDefault:"100.96.0.0/11"`
	NodeNetworks     []network.CIDR `env:"NODE_NETWORKS"`
	VPNNetwork       network.CIDR   `env:"VPN_NETWORK"`
	SeedPodNetworkV4 network.CIDR   `env:"SEED_POD_NETWORK_V4"`
	PodName          string         `env:"POD_NAME"`
	StatusPath       string         `env:"OPENVPN_STATUS_PATH"`
	IsHA             bool           `env:"IS_HA"`
	HAVPNClients     int            `env:"HA_VPN_CLIENTS"`
	LocalNodeIP      string         `env:"LOCAL_NODE_IP" envDefault:"255.255.255.255"`
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
	log.Info("config parsed", "config", cfg)
	return cfg, nil
}
