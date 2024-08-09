// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"

	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
)

func validateVPNNetworkCIDR(cidr network.CIDR, ipFamily string) error {
	length, _ := cidr.Mask.Size()
	switch ipFamily {
	case constants.IPv4Family:
		if length != 120 {
			return fmt.Errorf("ipv4 setup needs ipv6 vpn network with /120 subnet mask, got %d", length)
		}
	case constants.IPv6Family:
		if length != 120 {
			return fmt.Errorf("ipv6 setup needs vpn network to have /120 subnet mask, got %d", length)
		}
	default:
		return fmt.Errorf("unknown ipFamily: %s", ipFamily)
	}
	return nil
}
