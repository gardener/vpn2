// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"fmt"
	"net"
)

// Netmap maps the given IP address to a new IP address in the provided subnet.
func Netmap(ip string, subnet string) (string, error) {
	srcIp := net.ParseIP(ip)
	if srcIp == nil {
		return "", fmt.Errorf("failed to parse ip: %v", ip)
	}
	_, subnetMask, err := net.ParseCIDR(subnet)
	if err != nil {
		return "", fmt.Errorf("failed to parse subnet: %v", err)
	}
	mappedIP, err := netmap(srcIp, *subnetMask)
	if err != nil {
		return "", err
	}
	return mappedIP.String(), nil
}

func netmap(ip net.IP, subnet net.IPNet) (net.IP, error) {
	// Ensure the IP is in the correct format (IPv4 or IPv6)
	ipv4 := ip.To4()
	subnetBase := subnet.IP.To4()
	if ipv4 == nil || subnetBase == nil {
		return nil, fmt.Errorf("failed to map ip %s to subnet %s. only IPv4 is supported for mapping", ip.String(), subnet.String())
	}

	// Apply the subnet mask to get the base IP address
	mappedIP := make(net.IP, len(ipv4))
	for i := range ipv4 {
		mappedIP[i] = (ipv4[i] & ^subnet.Mask[i]) | subnetBase[i]
	}

	return mappedIP, nil
}
