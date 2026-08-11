// SPDX-FileCopyrightText: Contributors to the Gardener project
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

// DetectFragmentMTU returns the fragment size for UDP fragments in OpenVPN based on the
// MTU of the default route device (i.e. eth0 in a container) by subtracting
// the given overhead for UDP Proxy encapsulation via UDPM (UDP-Mux) protocol.
func DetectFragmentMTU(udpmVersion string, isHA bool) (int, error) {
	defaultMTU, err := GetDefaultMTU()

	if err != nil {
		return 0, fmt.Errorf("failed to detect fragment MTU: %w", err)
	}

	overhead := 0
	switch udpmVersion {
	case "v1":
		overhead += constants.UDPProxyOverheadV1
	case "v2":
		overhead += constants.UDPProxyOverheadV2
	default:
		return 0, fmt.Errorf("invalid UDPM version: %s", udpmVersion)
	}
	if isHA {
		overhead += constants.UDProxyHAVPNOverhead
	}

	fragmentMTU := defaultMTU - overhead

	return fragmentMTU, nil
}
