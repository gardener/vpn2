// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

import (
	"net"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	IPv4Family = "IPv4"
	IPv6Family = "IPv6"

	// BondDevice is the name of the bond device used for the HA deployment.
	BondDevice = "bond0"
	// TapDevice is the name of the tap device used for the HA VPN.
	TapDevice = "tap0"
	// TunnelDevice is the name of the tunnel device used for non-HA VPN.
	TunnelDevice = "tun0"
	// VPNNetworkMask is the required prefix size for the VPN network.
	VPNNetworkMask = 96

	ShootPodNetworkMapped     = constants.ReservedShootPodNetworkMappedRange
	ShootServiceNetworkMapped = constants.ReservedShootServiceNetworkMappedRange
	ShootNodeNetworkMapped    = constants.ReservedShootNodeNetworkMappedRange
	SeedPodNetworkMapped      = constants.ReservedSeedPodNetworkMappedRange

	EnvoyVPNGroupId = constants.EnvoyVPNGroupId
)

// DefaultVPNNetwork is the default IPv6 transfer network used by VPN.
var DefaultVPNNetwork net.IPNet

func init() {
	_, nw, err := net.ParseCIDR(constants.DefaultVPNRangeV6)
	if err != nil {
		panic(err)
	}
	DefaultVPNNetwork = *nw
}
