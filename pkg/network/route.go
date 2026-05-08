// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package network

import (
	"fmt"
	"net"

	"github.com/go-logr/logr"
	"github.com/vishvananda/netlink"
)

func getDefaultRoute() (*netlink.Route, error) {
	_, defaultIPv4, _ := net.ParseCIDR("0.0.0.0/0")
	_, defaultIPv6, _ := net.ParseCIDR("::/0")

	routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("failed to list network routes: %w", err)
	}

	var defaultRoute *netlink.Route
	for _, route := range routes {
		if route.Dst != nil {
			if route.Dst.String() == defaultIPv4.String() || route.Dst.String() == defaultIPv6.String() {
				defaultRoute = &route
				break
			}
		}
	}

	if defaultRoute == nil {
		return nil, fmt.Errorf("failed to find default route")
	}

	return defaultRoute, nil
}


func routeForNetwork(net *net.IPNet, device netlink.Link) netlink.Route {
	// ip route replace $net dev $device
	route := netlink.Route{
		Dst:       net,
		LinkIndex: device.Attrs().Index,
	}
	return route
}

func ReplaceRoute(log logr.Logger, ipnet *net.IPNet, dev netlink.Link) error {
	route := routeForNetwork(ipnet, dev)
	log.Info("replacing route", "route", route, "ipnet", ipnet)
	if err := netlink.RouteReplace(&route); err != nil {
		return fmt.Errorf("error replacing route for %s: %w", ipnet, err)
	}
	return nil
}
