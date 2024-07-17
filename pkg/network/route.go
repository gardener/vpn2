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

func routeForNetwork(net *net.IPNet, device netlink.Link) netlink.Route {
	// ip route replace $net dev $device
	route := netlink.Route{
		Dst:       net,
		LinkIndex: device.Attrs().Index,
	}
	return route
}

func RouteReplace(log logr.Logger, ipnet *net.IPNet, dev netlink.Link) error {
	route := routeForNetwork(ipnet, dev)
	log.Info("replacing route", "route", route, "ipnet", ipnet)
	if err := netlink.RouteReplace(&route); err != nil {
		return fmt.Errorf("error replacing route for %s: %w", ipnet, err)
	}
	return nil
}
