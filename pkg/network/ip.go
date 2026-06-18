// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"fmt"
	"hash/fnv"
	"net"
	"slices"

	"github.com/gardener/vpn2/pkg/constants"
)

// The High-Availability VPN divides the VPN network in several subnets.
// The last 32 bits of the /96 VPN network CIDR are structured using byte 12 (b12) as a discriminator:
//
// Assuming the VPN network is using the default CIDR `fd8f:6d53:b97a:1::0/96`:
//   - b12 = 0xaa:        seed clients — b13+b14+b15 are derived from a 24-bit hash of the pod name
//     - e.g. `fd8f:6d53:b97a:1::aa46:570d/104`
//   - b12 = 0xbb:        shoot clients — b13=0x00, b14=0x00, b15=client index
//     - shoot client 0: `fd8f:6d53:b97a:1::bb00:0`
//     - shoot client 1: `fd8f:6d53:b97a:1::bb00:1`
//   - b12 = 0xff:        VPN tunnel subnets — b13=vpnIndex, b14+b15=tunnel host
//     - subnet for VPN index 0: `fd8f:6d53:b97a:1::ff00:0/112`
//     - subnet for VPN index 1: `fd8f:6d53:b97a:1::ff01:0/112`

const (
	addrLen             = 128
	bondPrefixSize      = 104
	vpnTunnelPrefixSize = 112
	seedClientMarker    = 0xaa // b12 value for seed client addresses
	shootClientMarker   = 0xbb // b12 value for shoot client addresses
	tunnelMarker        = 0xff // b12 value for VPN tunnel subnet addresses
	startIndexSeed      = 1
	endIndexSeed        = 0xffffff
)

func BondingShootClientAddress(vpnNetwork *net.IPNet, vpnClientIndex int) *net.IPNet {
	ip := BondingShootClientIP(vpnNetwork, vpnClientIndex)
	return BondingAddressForClient(ip)
}

// BondingShootClientSubnet returns the /104 subnet used by all shoot clients.
func BondingShootClientSubnet(vpnNetwork *net.IPNet) *net.IPNet {
	return BondingShootClientAddress(vpnNetwork, 0)
}

// BondingSeedClientSubnet returns the /104 subnet used by all seed clients.
func BondingSeedClientSubnet(vpnNetwork *net.IPNet) *net.IPNet {
	base, _, _ := BondingSeedClientRange(vpnNetwork.IP)
	return BondingAddressForClient(base)
}

func BondingSeedClientAddress(vpnNetwork *net.IPNet, podName string) *net.IPNet {
	base := slices.Clone(vpnNetwork.IP.To16())
	// Seed clients: b12=seedClientMarker, b13+b14+b15 derived from a 24-bit hash of the pod name.
	base[12] = byte(seedClientMarker)

	h := fnv.New32a()
	_, _ = h.Write([]byte(podName))
	// XOR-fold all 32 bits into 24 to avoid discarding entropy from the top byte.
	sum32 := h.Sum32()
	sum24 := (sum32 & 0xFFFFFF) ^ (sum32 >> 24)

	base[13] = byte((sum24 >> 16) & 0xFF)
	base[14] = byte((sum24 >> 8) & 0xFF)
	base[15] = byte(sum24 & 0xFF)

	return BondingAddressForClient(base)
}

func BondingAddressForClient(ip net.IP) *net.IPNet {
	return &net.IPNet{
		IP:   ip,
		Mask: net.CIDRMask(bondPrefixSize, addrLen),
	}
}

func AllBondingShootClientIPs(vpnNetwork *net.IPNet, haVPNClients int) []net.IP {
	ips := make([]net.IP, haVPNClients)
	for i := range haVPNClients {
		ips[i] = BondingShootClientIP(vpnNetwork, i)
	}
	return ips
}

func BondingShootClientIP(vpnNetwork *net.IPNet, index int) net.IP {
	ip := slices.Clone(vpnNetwork.IP.To16())
	ip[12] = byte(shootClientMarker)
	ip[13] = 0
	ip[14] = 0
	ip[15] = byte(index & 0xFF)
	return ip
}

func BondingSeedClientRange(vpnNetworkIP net.IP) (base net.IP, startIndex, endIndex int) {
	base = slices.Clone(vpnNetworkIP.To16())
	base[12] = byte(seedClientMarker)
	base[13] = 0
	base[14] = 0
	base[15] = 0
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
	base[15] = 0
	base[14] = 0
	base[13] = byte(vpnIndex & 0xFF)
	base[12] = byte(tunnelMarker)

	return CIDR{
		IP:   base,
		Mask: net.CIDRMask(vpnTunnelPrefixSize, addrLen),
	}
}
