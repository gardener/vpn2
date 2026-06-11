// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

import (
	"net"
	"time"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	IPv4Family = "IPv4"
	IPv6Family = "IPv6"

	// TunnelMTUOverhead is the number of bytes subtracted from the underlying interface MTU
	// to derive the OpenVPN tun-mtu value (IPv6 header + TCP header + OpenVPN framing).
	TunnelMTUOverhead = 130
	// MinimumMTU is the smallest possible MTU that can still transport IPv6 packets
	MinimumMTU = 1280
	// BondDevice is the name of the bond device used for the HA deployment.
	BondDevice = "bond0"
	// TapDevice is the name of the tap device used for the HA VPN.
	TapDevice = "tap0"
	// TunnelDevice is the name of the tunnel device used for non-HA VPN.
	TunnelDevice = "tun0"
	// VPNNetworkMask is the required prefix size for the VPN network.
	VPNNetworkMask = 96

	BondingModeActiveBackup = "active-backup"
	BondingModeBalanceRR    = "balance-rr"

	ShootPodNetworkMapped     = constants.ReservedShootPodNetworkMappedRange
	ShootServiceNetworkMapped = constants.ReservedShootServiceNetworkMappedRange
	ShootNodeNetworkMapped    = constants.ReservedShootNodeNetworkMappedRange
	SeedPodNetworkMapped      = constants.ReservedSeedPodNetworkMappedRange

	// RoutesPerClientMax is the upper limit for route table entries. One entry estimated at ~50 bytes of memory.
	RoutesPerClientMax = 1 << 20 // 1 million routes (~50MB) per client.
	RoutesPerClientMin = 256     // OpenVPN default

	PathControllerUpdateInterval  = 2 * time.Second
	TunnelControllerUpdateTimeout = 2 * PathControllerUpdateInterval

	EnvoyVPNGroupId = constants.EnvoyVPNGroupId
)

// BondingModes are the supported bonding modes for the HA VPN.
var BondingModes = []string{BondingModeActiveBackup, BondingModeBalanceRR}

// DefaultVPNNetwork is the default IPv6 transfer network used by VPN.
var DefaultVPNNetwork net.IPNet

func init() {
	_, nw, err := net.ParseCIDR(constants.DefaultVPNRangeV6)
	if err != nil {
		panic(err)
	}
	DefaultVPNNetwork = *nw
}
