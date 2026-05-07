package network

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"

	"github.com/gardener/vpn2/pkg/constants"
)

// GetDefaultMTU returns the MTU of the default route of the pod.
func GetDefaultMTU() (int, error) {

	// Get default route
	_, defaultIPv4, _ := net.ParseCIDR("0.0.0.0/0")
	_, defaultIPv6, _ := net.ParseCIDR("::/0")

	routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return 0, fmt.Errorf("failed to list network routes: %w", err)
	}

	var defaultRoute *netlink.Route
	for _, route := range routes {
		if route.Dst != nil && route.Dst.String() == defaultIPv4.String() || route.Dst.String() == defaultIPv6.String() {
			defaultRoute = &route
			break
		}
	}

	if defaultRoute == nil {
		return 0, fmt.Errorf("failed to find default route: %w", err)
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
