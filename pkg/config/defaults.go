// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
