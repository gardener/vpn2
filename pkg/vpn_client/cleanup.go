// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpn_client

import (
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/gardener/vpn2/pkg/openvpn"
)

func Cleanup(log logr.Logger, values openvpn.ClientValues) error {
	log.Info("Cleaning up VPN client resources")

	// Remove any IPv6 address assigned to the VPN device. It will be reassigned by openvpn on the next connection.
	tuntap, err := netlink.LinkByName(values.Device)
	if err != nil {
		if errors.As(err, &netlink.LinkNotFoundError{}) {
			log.Info("VPN device not found, nothing to clean up", "device", values.Device)
			return nil
		}
		return fmt.Errorf("failed to get link %s: %w", values.Device, err)
	}

	if err := netlink.LinkSetDown(tuntap); err != nil {
		return fmt.Errorf("failed to set link %s down: %w", values.Device, err)
	}

	addr, err := netlink.AddrList(tuntap, unix.AF_INET6)
	if err != nil {
		return fmt.Errorf("failed to list addresses for link %s: %w", values.Device, err)
	}
	for _, a := range addr {
		if err := netlink.AddrDel(tuntap, &a); err != nil {
			return fmt.Errorf("failed to delete address %s from link %s: %w", a.String(), values.Device, err)
		}
	}

	return nil
}
