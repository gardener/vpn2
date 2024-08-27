// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"

	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
)

func validateVPNNetworkCIDR(cidr network.CIDR) error {
	length, _ := cidr.Mask.Size()
	if length != constants.VPNNetworkMask {
		return fmt.Errorf("vpn network needs to have /%d subnet mask, got /%d", constants.VPNNetworkMask, length)
	}
	return nil
}
