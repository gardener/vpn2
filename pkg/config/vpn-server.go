// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/caarlos0/env/v10"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/go-logr/logr"
)

type VPNServer struct {
	IPFamilies     string       `env:"IP_FAMILIES" envDefault:"IPv4"`
	ServiceNetwork network.CIDR `env:"SERVICE_NETWORK" envDefault:"100.64.0.0/13"`
	PodNetwork     network.CIDR `env:"POD_NETWORK" envDefault:"100.96.0.0/11"`
	NodeNetwork    network.CIDR `env:"NODE_NETWORK"`
	VPNNetwork     network.CIDR `env:"VPN_NETWORK"`
	PodName        string       `env:"POD_NAME"`
	StatusPath     string       `env:"OPENVPN_STATUS_PATH"`
	IsHA           bool         `env:"IS_HA"`
	HAVPNClients   int          `env:"HA_VPN_CLIENTS"`
	LocalNodeIP    string       `env:"LOCAL_NODE_IP" envDefault:"255.255.255.255"`
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
	log.Info("config parsed", "config", cfg)
	return cfg, nil
}
