// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"errors"
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

const (
	familyAll     = 0
	ScopeUniverse = 0
	ScopeLink     = 253
)

// DeleteLinkByName delete a link by name.
func DeleteLinkByName(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		var linkNotFoundError netlink.LinkNotFoundError
		if errors.As(err, &linkNotFoundError) {
			return nil
		}
		return fmt.Errorf("failed to get link %s: %w", name, err)
	}

	if err = netlink.LinkDel(link); err != nil {
		return fmt.Errorf("failed to delete link %s: %w", name, err)
	}
	return nil
}

// CreateTunnelIP6Tnl creates an ip6tnl tunnel to allow IPv4 and IPv6 packages over IPv6 and sets it up.
func CreateTunnelIP6Tnl(linkName string, local, remote net.IP) error {
	tunnel := &netlink.Ip6tnl{
		LinkAttrs: netlink.LinkAttrs{
			Name: linkName,
		},
		Local:  local,
		Remote: remote,
	}
	if err := netlink.LinkAdd(tunnel); err != nil {
		return fmt.Errorf("failed to add %s link: %w", linkName, err)
	}
	if err := netlink.LinkSetUp(tunnel); err != nil {
		return fmt.Errorf("failed to up %s link: %w", linkName, err)
	}
	return nil
}

// GetLinkIPAddressesByName gets the IP addresses for the given link name and scope (`ScopeLink` or `ScopeUniversal`).
func GetLinkIPAddressesByName(name string, scope int) ([]net.IP, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get link %s: %w", name, err)
	}
	return GetLinkIPAddresses(link, scope)
}

// GetLinkIPAddresses gets the IP addresses for the given link and scope (`ScopeLink` or `ScopeUniversal`).
func GetLinkIPAddresses(link netlink.Link, scope int) ([]net.IP, error) {
	addrs, err := netlink.AddrList(link, familyAll)
	if err != nil {
		return nil, fmt.Errorf("failed to list addresses of link %s: %w", link.Attrs().Name, err)
	}
	var ips []net.IP
	for _, addr := range addrs {
		if addr.Scope == scope {
			ips = append(ips, addr.IP)
		}
	}
	return ips, nil
}
