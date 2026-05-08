// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"fmt"

	"github.com/vishvananda/netlink"

	"github.com/gardener/vpn2/pkg/constants"
)

// GetDefaultMTU returns the MTU of the default route of the pod.
func GetDefaultMTU() (int, error) {

	defaultRoute, err := getDefaultRoute()
	if err != nil {
		return 0, err
	}

	// Get route interface
	defaultInterface, err := netlink.LinkByIndex(defaultRoute.LinkIndex)
	if err != nil {
		return 0, fmt.Errorf("failed to find default route interface: %w", err)
	}

	return defaultInterface.Attrs().MTU, nil
}

// DetectTunnelMTU returns the MTU for the VPN tunnel device by finding the
// MTU of the default route device (i.e. eth0 in a container) and subtracting
// the given overhead for VPN encapsulation.
func DetectTunnelMTU(overhead int) (int, error) {
	defaultMTU, err := GetDefaultMTU()

	if err != nil {
		return 0, fmt.Errorf("failed to detect tunnel MTU: %w", err)
	}

	tunnelMTU := defaultMTU - overhead

	// Make sure we never go below IPv6 viability
	if tunnelMTU < constants.MinimumMTU {
		tunnelMTU = constants.MinimumMTU
	}

	return tunnelMTU, nil
}
