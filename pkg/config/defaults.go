// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"net"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/vpn2/pkg/network"
)

func getVPNNetworkDefault() (network.CIDR, error) {
	// Always use IPv6 ULA for the VPN transfer network
	_, cidr, err := net.ParseCIDR(constants.DefaultVPNRangeV6)
	if err != nil {
		return network.CIDR{}, err
	}
	return network.CIDR(*cidr), nil
}
