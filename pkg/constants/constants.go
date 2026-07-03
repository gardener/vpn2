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

	// ManagementPort is the default port for the OpenVPN management interface.
	ManagementPort = 7505
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

	WatchdogWindowSize = 20
	WatchdogThreshold  = 10
	WatchdogCooldown   = 20

	EnvoyVPNGroupId = constants.EnvoyVPNGroupId

	// ECMPHashPolicyL3 is the value for Layer 3 ECMP hash policy. It uses source and target IPs.
	ECMPHashPolicyL3 = "0"
	// ECMPHashPolicyL4 is the value for Layer 4 ECMP hash policy. It uses source and target ports on top of L3.
	ECMPHashPolicyL4 = "1"

	// ResilientNexthopBuckets is the number of buckets in the resilient ECMP nexthop groups. Each
	// bucket maps a set of flows to one shoot client's ip6tnl device. More buckets give finer-grained
	// flow distribution (fewer flows move together) at a small memory cost.
	ResilientNexthopBuckets = 1024
	// ResilientNexthopIdleTimer is the time in seconds a bucket must be idle (no forwarded packets)
	// before it may be reassigned to a recovered/added nexthop. Larger values keep idle connections
	// pinned to their current device for longer when a shoot client recovers.
	ResilientNexthopIdleTimer = 60
	// ResilientNexthopUnbalancedTimer is the time in seconds a group may stay unbalanced before the
	// kernel force-migrates even active buckets to rebalance load. 0 disables forced rebalancing so
	// active connections are never moved off their device for load-balancing reasons.
	ResilientNexthopUnbalancedTimer = 0

	// NexthopGroupIDIPv4 and NexthopGroupIDIPv6 are the (network-namespace-local) IDs of the
	// resilient ECMP nexthop groups used by the IPv4 and IPv6 shoot-network routes respectively.
	NexthopGroupIDIPv4 = 400
	NexthopGroupIDIPv6 = 600
	// NexthopDeviceBaseIPv4 and NexthopDeviceBaseIPv6 are the base IDs for the per-shoot-client device
	// nexthop objects. The actual ID is the base plus the client index.
	NexthopDeviceBaseIPv4 = 4000
	NexthopDeviceBaseIPv6 = 6000
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
