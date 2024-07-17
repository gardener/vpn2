// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"fmt"
)

func ValidateCIDR(cidr CIDR, ipFamily string) error {
	length, _ := cidr.Mask.Size()
	switch ipFamily {
	case "IPv4":
		if length != 120 {
			return fmt.Errorf("ipv4 setup needs ipv6 vpn network with /120 subnet mask, got %d", length)
		}
	case "IPv6":
		if length != 120 {
			return fmt.Errorf("ipv6 setup needs vpn network to have /120 subnet mask, got %d", length)
		}
	default:
		return fmt.Errorf("unknown ipFamily: %s", ipFamily)
	}
	return nil
}
