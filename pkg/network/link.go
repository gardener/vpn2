// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package network

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
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
		if _, ok := errors.AsType[netlink.LinkNotFoundError](err); ok {
			return nil
		}
		return fmt.Errorf("failed to get link %s for deletion: %w", name, err)
	}

	if err = netlink.LinkDel(link); err != nil {
		return fmt.Errorf("failed to delete link %s: %w", name, err)
	}
	return nil
}

// CreateTunnel creates an ip6tnl tunnel to allow IPv4 and IPv6 packages over IPv6 and sets it up.
func CreateTunnel(linkName string, local, remote net.IP) error {
	tunnel := &netlink.Ip6tnl{
		LinkAttrs: netlink.LinkAttrs{
			Name: linkName,
		},
		Local:  local,
		Remote: remote,
	}
	if err := netlink.LinkAdd(tunnel); err != nil {
		return fmt.Errorf("failed to add link %s: %w", linkName, err)
	}
	if err := netlink.LinkSetUp(tunnel); err != nil {
		return fmt.Errorf("failed to set up link %s: %w", linkName, err)
	}
	return nil
}

// GetLinkIPAddressesByName gets the IP addresses for the given link name and scope (`ScopeLink` or `ScopeUniversal`).
func GetLinkIPAddressesByName(name string, scope int) ([]net.IP, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get link %s: %w", name, err)
	}
	return getLinkIPAddresses(link, scope)
}

// getLinkIPAddresses gets the IP addresses for the given link and scope (`ScopeLink` or `ScopeUniversal`).
func getLinkIPAddresses(link netlink.Link, scope int) ([]net.IP, error) {
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

// GetLinkIPAddrForIP gets the netlink.Addr for the given link name and IP address.
func GetLinkIPAddrForIP(name string, ip net.IP) (*netlink.Addr, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get link %s: %w", name, err)
	}
	addrs, err := netlink.AddrList(link, familyAll)
	if err != nil {
		return nil, fmt.Errorf("failed to list addresses of link %s: %w", link.Attrs().Name, err)
	}
	for _, addr := range addrs {
		if addr.IP.Equal(ip) {
			return &addr, nil
		}
	}
	return nil, fmt.Errorf("no address %s found on link %s", ip.String(), name)
}

// IPAddrFlagsToString converts IP address flags to a human-readable string.
func IPAddrFlagsToString(flags int) string {
	flagsStr := strings.Builder{}

	flagTypes := map[int]string{
		unix.IFA_F_SECONDARY:      "Secondary",
		unix.IFA_F_NODAD:          "Nodad",
		unix.IFA_F_HOMEADDRESS:    "Home",
		unix.IFA_F_DEPRECATED:     "Deprecated",
		unix.IFA_F_OPTIMISTIC:     "Optimistic",
		unix.IFA_F_DADFAILED:      "Dadfailed",
		unix.IFA_F_TENTATIVE:      "Tentative",
		unix.IFA_F_PERMANENT:      "Permanent",
		unix.IFA_F_MANAGETEMPADDR: "Managetempaddr",
		unix.IFA_F_NOPREFIXROUTE:  "Noprefixroute",
		unix.IFA_F_MCAUTOJOIN:     "Mcautojoin",
		unix.IFA_F_STABLE_PRIVACY: "Stable_privacy",
	}

	for flag, name := range flagTypes {
		if flags&flag != 0 {
			flagsStr.WriteString(name + " ")
		}
	}

	return strings.TrimSpace(flagsStr.String())
}
