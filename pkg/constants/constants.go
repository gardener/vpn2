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
	// VPNNetworkPrefixSize is the required prefix size for the VPN network.
	VPNNetworkMask = 96
)

// DefaultVPNNetwork is the default IPv6 transfer network used by VPN.
var DefaultVPNNetwork net.IPNet

func init() {
	// TODO (Martin Weindel) if Gardener is updated to have /96 prefix size instead of /120, adjust this code accordingly
	// because of circular dependencies, this is postponed after release of VPN2
	ip, _, err := net.ParseCIDR(constants.DefaultVPNRangeV6)
	if err != nil {
		panic(err)
	}
	DefaultVPNNetwork = net.IPNet{
		IP:   ip,
		Mask: net.CIDRMask(VPNNetworkMask, 128),
	}
}
