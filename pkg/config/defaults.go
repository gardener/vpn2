// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"net"

	"github.com/gardener/vpn2/pkg/network"
)

func getVPNNetworkDefault() (network.CIDR, error) {
	// Always use ipv6 ULA for the VPN transfer network
	_, cidr, err := net.ParseCIDR(defaultIPV6VpnNetwork)
	if err != nil {
		return network.CIDR{}, err
	}
	return network.CIDR(*cidr), nil
}
