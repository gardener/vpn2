// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"net"

	"github.com/vishvananda/netlink"
)

// DeleteNeighborEntry deletes all entries from the neighbor table that match device and ip address.
func DeleteNeighborEntry(ip net.IP, device string) error {

	link, err := netlink.LinkByName(device)
	if err != nil {
		return err
	}

	neighbors, err := netlink.NeighList(link.Attrs().Index, netlink.FAMILY_ALL)
	if err != nil {
		return err
	}

	for _, n := range neighbors {
		if n.IP.Equal(ip) {
			if err := netlink.NeighDel(&n); err != nil {
				return err
			}
		}
	}

	return nil
}
