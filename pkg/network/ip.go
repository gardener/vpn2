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
// The last 64 bits of the /64 VPN network CIDR are structured using bytes 8–11 as a discriminator
// and bytes 12–15 as a 32-bit payload:
//
// Assuming the VPN network is using the default CIDR `fd8f:6d53:b97a:1::/64`:
//   - b8=0xaa, b9-b11=0x00: seed clients — b12-b15 = full 32-bit FNV hash of the pod name
//     - e.g. `fd8f:6d53:b97a:1:aa00::4657:570d/96`
//   - b8=0xbb, b9-b12=0x00: shoot clients — b13=0xb, b14=0x00, b15 = client index (8-bit)
//     - shoot client 0: `fd8f:6d53:b97a:1:bb00::b:0/96`
//     - shoot client 1: `fd8f:6d53:b97a:1:bb00::b:1/96`
//   - b8=0xff, b9-b11=0x00: VPN tunnel subnets — b12=vpnIndex, b13-b15=tunnel host
//     - subnet for VPN index 0: `fd8f:6d53:b97a:1:ff00::100:0/112`
//     - subnet for VPN index 1: `fd8f:6d53:b97a:1:ff00::101:0/112`

const (
	addrLen             = 128
	bondPrefixSize      = 96 // /96 per-type subnet: 4-byte marker (b8-b11) + 4-byte payload (b12-b15)
	vpnTunnelPrefixSize = 112
	seedClientMarker    = 0xaa // b8 value for seed client addresses
	shootClientMarker   = 0xbb // b8 value for shoot client addresses
	tunnelMarker        = 0xff // b8 value for VPN tunnel subnet addresses
	bondStartShoot      = 0xb  // avoids anycast address with shoot client 0 (fd8f:6d53:b97a:1:bb00::/96)
	startIndexSeed      = 1
	endIndexSeed        = 0xffffffff
)

func BondingShootClientAddress(vpnNetwork *net.IPNet, vpnClientIndex int) *net.IPNet {
	ip := BondingShootClientIP(vpnNetwork, vpnClientIndex)
	return BondingAddressForClient(ip)
}

// BondingShootClientSubnet returns the /96 subnet used by all shoot clients.
func BondingShootClientSubnet(vpnNetwork *net.IPNet) *net.IPNet {
	return BondingShootClientAddress(vpnNetwork, 0)
}

// BondingSeedClientSubnet returns the /96 subnet used by all seed clients.
func BondingSeedClientSubnet(vpnNetwork *net.IPNet) *net.IPNet {
	base, _, _ := BondingSeedClientRange(vpnNetwork.IP)
	return BondingAddressForClient(base)
}

func BondingSeedClientAddress(vpnNetwork *net.IPNet, podName string) *net.IPNet {
	base := slices.Clone(vpnNetwork.IP.To16())
	// Seed clients: b8=seedClientMarker, b9-b11=0x00, b12-b15 = full 32-bit FNV hash of pod name.
	base[8] = byte(seedClientMarker)
	base[9] = 0
	base[10] = 0
	base[11] = 0

	h := fnv.New32a()
	_, _ = h.Write([]byte(podName))
	sum32 := h.Sum32()

	base[12] = byte((sum32 >> 24) & 0xFF)
	base[13] = byte((sum32 >> 16) & 0xFF)
	base[14] = byte((sum32 >> 8) & 0xFF)
	base[15] = byte(sum32 & 0xFF)

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
	ip[8] = byte(shootClientMarker)
	ip[9] = 0
	ip[10] = 0
	ip[11] = 0
	ip[12] = 0
	ip[13] = byte(bondStartShoot)
	ip[14] = 0
	ip[15] = byte(index & 0xFF)
	return ip
}

func BondingSeedClientRange(vpnNetworkIP net.IP) (base net.IP, startIndex, endIndex int) {
	base = slices.Clone(vpnNetworkIP.To16())
	base[8] = byte(seedClientMarker)
	base[9] = 0
	base[10] = 0
	base[11] = 0
	base[12] = 0
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
	base[8] = byte(tunnelMarker)
	base[9] = 0
	base[10] = 0
	base[11] = 0
	base[12] = 1
	base[13] = byte(vpnIndex & 0xFF)
	base[14] = 0
	base[15] = 0

	return CIDR{
		IP:   base,
		Mask: net.CIDRMask(vpnTunnelPrefixSize, addrLen),
	}
}
