// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
)

func getVPNNetworkDefault() (network.CIDR, error) {
	return network.CIDR(constants.DefaultVPNNetwork), nil
}
