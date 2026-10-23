// SPDX-FileCopyrightText: Contributors to the Gardener project
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

	// ShootMarkChain is the name of the iptables mangle chain that assigns fwmarks to
	// shoot-destined connections and saves/restores them via CONNMARK for per-connection persistence.
	ShootMarkChain = "SHOOT_MARK"
	// RoutingTableBaseID is the base ID for client-specific routing tables. i.e. vpn-shoot-0 uses table 100, vpn-shoot-1 uses table 101, etc.
	// This is well below the main-table (254) and local-table (255) IDs.
	RoutingTableBaseID = 100
	// FwmarkBase is the base fwmark value for per-shoot-client traffic classification.
	// Client N is assigned fwmark FwmarkBase+N. Mark 100 matches table 100, mark 101 matches table 101, etc.
	// ip rule shows marks in hex, so 100 = 0x64, 101 = 0x65, nf_conntrack shows the real values.
	FwmarkBase = 100
	// FwmarkRulePriorityBase is the base priority for the per-shoot-client fwmark ip rules.
	// Client N gets priority FwmarkRulePriorityBase+N. Lower numeric priority = higher precedence
	// in Linux policy routing; each rule needs a distinct priority. 100 is well below the main-table rule (priority 32766).
	FwmarkRulePriorityBase = 100
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
