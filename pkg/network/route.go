// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0
package network

import (
	"errors"
	"fmt"
	"net"
	"syscall"

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

func ReplaceRoute(log logr.Logger, ipnet *net.IPNet, dev netlink.Link) error {
	route := netlink.Route{
		Dst:       ipnet,
		LinkIndex: dev.Attrs().Index,
	}

	log.Info("replacing route", "route", route, "ipnet", ipnet)
	if err := netlink.RouteReplace(&route); err != nil {
		return fmt.Errorf("error replacing route for %s: %w", ipnet, err)
	}
	return nil
}

// ReplaceRouteInTable installs or replaces a route to dst that forwards via the named device in
// the given routing table. Use unix.RT_TABLE_MAIN (254) for the main routing table.
func ReplaceRouteInTable(dst *net.IPNet, linkName string, tableID int) error {
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to find link %s: %w", linkName, err)
	}
	route := &netlink.Route{
		Dst:       dst,
		LinkIndex: link.Attrs().Index,
		Table:     tableID,
	}
	if err := netlink.RouteReplace(route); err != nil {
		return fmt.Errorf("error replacing route for %s via %s table %d: %w", dst, linkName, tableID, err)
	}
	return nil
}

// AddFWmarkRule adds a policy routing rule that steers packets marked with the given fwmark into
// the given routing table. The priority must be unique among ip rules for the address family.
func AddFWmarkRule(mark, tableID, priority int, ipv6 bool) error {
	family := netlink.FAMILY_V4
	if ipv6 {
		family = netlink.FAMILY_V6
	}
	rule := netlink.NewRule()
	rule.Family = family
	rule.Mark = uint32(mark) // #nosec: G115 -- mark is a small non-negative constant.
	rule.Table = tableID
	rule.Priority = priority
	if err := netlink.RuleAdd(rule); err != nil {
		return fmt.Errorf("error adding ip rule fwmark %d table %d: %w", mark, tableID, err)
	}
	return nil
}

// AddLoopbackRoute installs a route for dst via lo in the main routing table. This is needed for
// IPv6 shoot networks when there is no default IPv6 gateway: without a main-table entry the
// initial routing lookup for locally-generated packets fails before iptables mangle OUTPUT fires,
// so the fwmark-based policy routing never gets a chance to redirect the packet. A loopback route
// (unlike a blackhole) allows the kernel to proceed to the re-route after mangle OUTPUT, where the
// fwmark set by SHOOT_MARK is evaluated against the ip rules and the packet is steered into the
// correct per-client table.
func AddLoopbackRoute(dst *net.IPNet) error {
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return fmt.Errorf("failed to find lo: %w", err)
	}
	route := &netlink.Route{
		Dst:       dst,
		LinkIndex: lo.Attrs().Index,
	}
	if err := netlink.RouteReplace(route); err != nil {
		return fmt.Errorf("error adding loopback route for %s: %w", dst, err)
	}
	return nil
}

// ReplaceFWmarkRule atomically redirects the fwmark policy rule for mark/priority to newTableID.
// It first adds the new rule, then deletes any old rule that points to a different table. This
// ensures there is no window where the mark is unroutable: a brief overlap with both rules present
// is safe because both tables are valid routing tables during the transition.
func ReplaceFWmarkRule(mark, newTableID, priority int, ipv6 bool) error {
	family := netlink.FAMILY_V4
	if ipv6 {
		family = netlink.FAMILY_V6
	}

	// Add the new rule first so packets are never left without a matching rule.
	newRule := netlink.NewRule()
	newRule.Family = family
	newRule.Mark = uint32(mark) // #nosec: G115 -- mark is a small non-negative constant.
	newRule.Table = newTableID
	newRule.Priority = priority
	if err := netlink.RuleAdd(newRule); err != nil {
		// If the exact rule already exists (same mark+priority+table), that is fine.
		if !errors.Is(err, syscall.EEXIST) {
			return fmt.Errorf("error adding ip rule fwmark %d table %d: %w", mark, newTableID, err)
		}
	}

	// Now delete any stale rules at the same priority that point to a different table.
	rules, err := netlink.RuleList(family)
	if err != nil {
		return fmt.Errorf("error listing ip rules: %w", err)
	}
	for _, r := range rules {
		if r.Mark == uint32(mark) && r.Priority == priority && r.Table != newTableID { // #nosec: G115 -- mark is a small non-negative constant.
			if err := netlink.RuleDel(&r); err != nil {
				return fmt.Errorf("error deleting stale ip rule fwmark %d table %d: %w", mark, r.Table, err)
			}
		}
	}
	return nil
}
