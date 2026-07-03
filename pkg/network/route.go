// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package network

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

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

// runIP executes the iproute2 "ip" command. Nexthop objects and routes that reference a nexthop
// group cannot be expressed via the netlink library (v1.3.1), so we drive them through iproute2.
func runIP(args ...string) error {
	out, err := exec.Command("ip", args...).CombinedOutput() // #nosec: G204 -- No user-provided input
	if err != nil {
		return fmt.Errorf("command 'ip %s' failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ReplaceDeviceNexthop creates or updates a device-scoped nexthop object with the given id for the
// given link. Device nexthops are address-family specific: pass ipv6=false for the IPv4 nexthop and
// ipv6=true for the IPv6 nexthop.
func ReplaceDeviceNexthop(id int, linkName string, ipv6 bool) error {
	var args []string
	if ipv6 {
		args = append(args, "-6")
	}
	args = append(args, "nexthop", "replace", "id", strconv.Itoa(id), "dev", linkName)
	return runIP(args...)
}

// ReplaceResilientNexthopGroup creates or updates a resilient ECMP nexthop group with the given id
// over the given member nexthop ids. The group inherits its address family from its members, so no
// family flag is passed. Resilient groups minimize flow disruption when members go down and come
// back: only buckets that have been idle for idleTimer seconds are migrated to a recovered member,
// and unbalancedTimer=0 disables force-migration of active buckets for rebalancing.
func ReplaceResilientNexthopGroup(groupID int, memberIDs []int, buckets, idleTimer, unbalancedTimer int) error {
	ids := make([]string, len(memberIDs))
	for i, id := range memberIDs {
		ids[i] = strconv.Itoa(id)
	}
	return runIP(
		"nexthop", "replace", "id", strconv.Itoa(groupID),
		"group", strings.Join(ids, "/"),
		"type", "resilient",
		"buckets", strconv.Itoa(buckets),
		"idle_timer", strconv.Itoa(idleTimer),
		"unbalanced_timer", strconv.Itoa(unbalancedTimer),
	)
}

// ReplaceRouteViaNexthopGroup installs or updates a route to dst that forwards via the given
// nexthop group id.
func ReplaceRouteViaNexthopGroup(dst *net.IPNet, groupID int) error {
	var args []string
	if dst.IP.To4() == nil {
		args = append(args, "-6")
	}
	args = append(args, "route", "replace", dst.String(), "nhid", strconv.Itoa(groupID))
	return runIP(args...)
}
