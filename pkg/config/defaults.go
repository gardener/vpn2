// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"net"

	"github.com/gardener/vpn2/pkg/network"
)

var ErrorInvalidIPFamily = errors.New("invalid IPFamily")

func getVPNNetworkDefault(ipFamily string) (network.CIDR, error) {
	switch ipFamily {
	case IPv4Family:
		_, cidr, err := net.ParseCIDR(defaultIPV4VpnNetwork)
		if err != nil {
			return network.CIDR{}, err
		}
		return network.CIDR(*cidr), nil
	case IPv6Family:
		_, cidr, err := net.ParseCIDR(defaultIPV6VpnNetwork)
		if err != nil {
			return network.CIDR{}, err
		}
		return network.CIDR(*cidr), nil
	default:
		return network.CIDR{}, ErrorInvalidIPFamily
	}
}
