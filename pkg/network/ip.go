// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"fmt"
	"net"
	"slices"

	"github.com/gardener/vpn2/pkg/constants"
)

// The High-Availability VPN divides the VPN network in several subnets.
// Assuming the VPN network is using the default CIDR `fd8f:6d53:b97a:1::0/112`, these subnets are used:
// - For the underlying VPN tunnel for each VPN server (the VPN index)
//   - subnet for VPN index 0: `fd8f:6d53:b97a:1::0/120`
//   - subnet for VPN index 1: `fd8f:6d53:b97a:1::100/120`
// - subnet for the bonding network: `fd8f:6d53:b97a:1::a00/119`
//   - IP of shoot client 0: `fd8f:6d53:b97a:1::b00`
//   - IP of shoot client 1: `fd8f:6d53:b97a:1::b01`
//   - IPs of seed clients are in the range `fd8f:6d53:b97a:1::a01` to `fd8f:6d53:b97a:1::aff`

const (
	addrLen             = 128
	bondPrefixSize      = 119
	vpnTunnelPrefixSize = 120
	bondStartSeed       = 0xa
	bondStartShoot      = 0xb
	startIndexSeed      = 1
	endIndexSeed        = 0xff
)

func BondingShootClientAddress(vpnNetwork *net.IPNet, vpnClientIndex int) *net.IPNet {
	ip := BondingShootClientIP(vpnNetwork, vpnClientIndex)
	return BondingAddressForClient(ip)
}

func BondingAddressForClient(ip net.IP) *net.IPNet {
	return &net.IPNet{
		IP:   ip,
		Mask: net.CIDRMask(bondPrefixSize, addrLen),
	}
}

func AllBondingShootClientIPs(vpnNetwork *net.IPNet, haVPNClients int) []net.IP {
	ips := make([]net.IP, haVPNClients)
	for i := 0; i < haVPNClients; i++ {
		ips[i] = BondingShootClientIP(vpnNetwork, i)
	}
	return ips
}

func BondingShootClientIP(vpnNetwork *net.IPNet, index int) net.IP {
	ip := slices.Clone(vpnNetwork.IP.To16())
	ip[len(ip)-1] = byte(index)
	ip[len(ip)-2] = byte(bondStartShoot)
	return ip
}

func BondingSeedClientRange(vpnNetworkIP net.IP) (base net.IP, startIndex, endIndex int) {
	base = slices.Clone(vpnNetworkIP.To16())
	base[len(base)-1] = 0
	base[len(base)-2] = byte(bondStartSeed)
	startIndex = startIndexSeed
	endIndex = endIndexSeed
	return
}

func ClientIndexFromBondingShootClientIP(clientIP net.IP) int {
	return int(clientIP[len(clientIP)-1])
}

func BondIP6TunnelLinkName(index int) string {
	return fmt.Sprintf("%s-ip6tnl%d", constants.BondDevice, index)
}

func HAVPNTunnelNetwork(vpnNetworkIP net.IP, vpnIndex int) CIDR {
	base := slices.Clone(vpnNetworkIP.To16())
	base[len(base)-1] = 0
	base[len(base)-2] = byte(vpnIndex)

	return CIDR{
		IP:   base,
		Mask: net.CIDRMask(vpnTunnelPrefixSize, addrLen),
	}
}
