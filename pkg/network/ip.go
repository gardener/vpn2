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

const (
	addrLen   = 128
	bondBits  = 122
	bondStart = 192
)

func GetBondAddressAndTargetsShootClient(vpnNetwork *net.IPNet, vpnClientIndex int) (*net.IPNet, []net.IP) {
	clientIP := ClientIP(vpnNetwork, vpnClientIndex)

	shootSubnet := &net.IPNet{
		IP:   clientIP,
		Mask: net.CIDRMask(bondBits, addrLen),
	}

	target := slices.Clone(clientIP)
	target[len(target)-1] = byte(bondStart + 1)

	return shootSubnet, append([]net.IP{}, target)
}

func GetBondAddressAndTargetsSeedClient(acquiredIP net.IP, vpnNetwork *net.IPNet, haVPNClients int) (*net.IPNet, []net.IP) {
	subnet := &net.IPNet{
		IP:   acquiredIP,
		Mask: net.CIDRMask(bondBits, addrLen),
	}

	targets := make([]net.IP, 0, haVPNClients)
	for i := range haVPNClients {
		targets = append(targets, ClientIP(vpnNetwork, i))
	}
	return subnet, targets
}

func ClientIP(vpnNetwork *net.IPNet, index int) net.IP {
	newIP := slices.Clone(vpnNetwork.IP.To16())
	newIP[len(newIP)-1] = byte(bondStart + 2 + index)
	return newIP
}

func ClientIndexFromClientIP(clientIP net.IP) int {
	return int(clientIP[len(clientIP)-1]) - bondStart - 2
}

func BondIP6TunnelLinkName(index int) string {
	return fmt.Sprintf("%s-ip6tnl%d", constants.BondDevice, index)
}
