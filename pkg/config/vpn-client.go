// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/caarlos0/env/v10"

	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
)

type VPNClient struct {
	IPFamilies           []string       `env:"IP_FAMILIES" envDefault:"IPv4"`
	Endpoint             string         `env:"ENDPOINT"`
	OpenVPNPort          uint           `env:"OPENVPN_PORT" envDefault:"8132"`
	VPNNetwork           network.CIDR   `env:"VPN_NETWORK"`
	SeedPodNetwork       network.CIDR   `env:"SEED_POD_NETWORK"`
	ShootServiceNetworks []network.CIDR `env:"SHOOT_SERVICE_NETWORKS"`
	ShootPodNetworks     []network.CIDR `env:"SHOOT_POD_NETWORKS"`
	ShootNodeNetworks    []network.CIDR `env:"SHOOT_NODE_NETWORKS"`
	IsShootClient        bool           `env:"IS_SHOOT_CLIENT"`
	PodName              string         `env:"POD_NAME"`
	Namespace            string         `env:"NAMESPACE"`
	VPNServerIndex       string         `env:"VPN_SERVER_INDEX"`
	VPNClientIndex       int
	IsHA                 bool          `env:"IS_HA"`
	ReversedVPNHeader    string        `env:"REVERSED_VPN_HEADER" envDefault:"invalid-host"`
	ReversedVPNHeaderKey string        `env:"REVERSED_VPN_HEADER_KEY" envDefault:"Reversed-VPN"`
	HAVPNClients         uint          `env:"HA_VPN_CLIENTS"`
	HAVPNServers         uint          `env:"HA_VPN_SERVERS"`
	PodLabelSelector     string        `env:"POD_LABEL_SELECTOR" envDefault:"app=kubernetes,role=apiserver"`
	WaitTime             time.Duration `env:"WAIT_TIME" envDefault:"2s"`
}

func (v VPNClient) PrimaryIPFamily() string {
	return v.IPFamilies[0]
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
	if err := validateVPNNetworkCIDR(cfg.VPNNetwork); err != nil {
		return VPNClient{}, err
	}
	cfg.VPNClientIndex = -1

	if cfg.IsHA {
		if cfg.IsShootClient {
			if cfg.PodName == "" {
				return VPNClient{}, fmt.Errorf("IS_HA and IS_SHOOT_CLIENT are set to true but POD_NAME is not set")
			}
		}
		if cfg.HAVPNServers > 0 && cfg.HAVPNClients == 0 {
			return VPNClient{}, fmt.Errorf("HA_VPN_SERVERS is set to %d but HA_VPN_CLIENTS is set to 0", cfg.HAVPNServers)
		}
		if cfg.HAVPNClients > 0 && cfg.HAVPNServers == 0 {
			return VPNClient{}, fmt.Errorf("HA_VPN_CLIENTS is set to %d but HA_VPN_SERVERS is set to 0", cfg.HAVPNClients)
		}
	}

	if len(cfg.IPFamilies) < 1 || len(cfg.IPFamilies) > 2 {
		return VPNClient{}, fmt.Errorf("IP_FAMILIES must contain at least one but no more than 2 elements")
	}

	for _, ipFamily := range cfg.IPFamilies {
		if ipFamily != network.IPv4Family && ipFamily != constants.IPv6Family {
			return VPNClient{}, fmt.Errorf("IP_FAMILIES can only contain %s or %s, found value %q", network.IPv4Family, constants.IPv6Family, ipFamily)
		}
	}

	// Remove ip family duplicates
	slices.Sort(cfg.IPFamilies)
	cfg.IPFamilies = slices.Compact(cfg.IPFamilies)

	if cfg.WaitTime < 0 {
		return VPNClient{}, fmt.Errorf("WAIT_TIME must not be negative")
	}

	if cfg.PodName != "" {
		podNameSlice := strings.Split(cfg.PodName, "-")
		clientIndex, err := strconv.Atoi(podNameSlice[len(podNameSlice)-1])
		if err == nil {
			cfg.VPNClientIndex = clientIndex
		}
	}
	return cfg, nil
}
