// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"strconv"
	"strings"
	"time"

	"github.com/caarlos0/env/v10"
	"github.com/gardener/vpn2/pkg/network"
)

type VPNClient struct {
	TCP struct {
		KeepAliveTime     int64 `env:"KEEPALIVE_TIME" envDefault:"7200"`
		KeepAliveInterval int64 `env:"KEEPALIVE_INTVL" envDefault:"75"`
		KeepAliveProbes   int64 `env:"KEEPALIVE_PROBES" envDefault:"9"`
	} `envPrefix:"TCP_"`
	IPFamilies        string       `env:"IP_FAMILIES" envDefault:"IPv4"`
	Endpoint          string       `env:"ENDPOINT"`
	OpenVPNPort       int          `env:"OPENVPN_PORT" envDefault:"8132"`
	VPNNetwork        network.CIDR `env:"VPN_NETWORK"`
	SeedPodNetwork    network.CIDR `env:"SEED_POD_NETWORK"`
	IsShootClient     bool         `env:"IS_SHOOT_CLIENT"`
	PodName           string       `env:"POD_NAME"`
	Namespace         string       `env:"NAMESPACE"`
	VPNServerIndex    string       `env:"VPN_SERVER_INDEX"`
	VPNClientIndex    int
	IsHA              bool          `env:"IS_HA"`
	ReversedVPNHeader string        `env:"REVERSED_VPN_HEADER" envDefault:"invalid-host"`
	HAVPNClients      int           `env:"HA_VPN_CLIENTS"`
	HAVPNServers      int           `env:"HA_VPN_SERVERS"`
	StartIndex        int           `env:"START_INDEX" envDefault:"200"`
	EndIndex          int           `env:"END_INDEX" envDefault:"254"`
	PodLabelSelector  string        `env:"POD_LABEL_SELECTOR" envDefault:"app=kubernetes,role=apiserver"`
	WaitTime          time.Duration `env:"WAIT_TIME" envDefault:"2s"`
}

func (v VPNClient) PrimaryIPFamily() string {
	return strings.Split(v.IPFamilies, ",")[0]
}

func GetVPNClientConfig() (VPNClient, error) {
	cfg := VPNClient{}
	if err := env.Parse(&cfg); err != nil {
		return cfg, err
	}
	if cfg.VPNNetwork.String() == "" {
		var err error
		cfg.VPNNetwork, err = getVPNNetworkDefault()
		if err != nil {
			return VPNClient{}, err
		}
	}
	if err := validateVPNNetworkCIDR(cfg.VPNNetwork, cfg.PrimaryIPFamily()); err != nil {
		return VPNClient{}, err
	}
	cfg.VPNClientIndex = -1
	if cfg.PodName != "" {
		podNameSlice := strings.Split(cfg.PodName, "-")
		clientIndex, err := strconv.Atoi(podNameSlice[len(podNameSlice)-1])
		if err == nil {
			cfg.VPNClientIndex = clientIndex
		}
	}
	return cfg, nil
}
