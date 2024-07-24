// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpn_client

import (
	"github.com/coreos/go-iptables/iptables"
	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/go-logr/logr"
)

func SetIPTableRules(log logr.Logger, cfg config.VPNClient) error {
	forwardDevice := "tun0"
	if cfg.VPNServerIndex != "" {
		forwardDevice = "bond0"
	}

	protocol := iptables.ProtocolIPv4
	if cfg.IPFamilies == "IPv6" {
		protocol = iptables.ProtocolIPv6
	}
	iptable, err := network.NewIPTables(log, protocol)
	if err != nil {
		return err
	}

	if cfg.IsShootClient {
		if cfg.IPFamilies == "IPv4" {
			err = iptable.Append("filter", "FORWARD", "--in-interface", forwardDevice, "-j", "ACCEPT")
			if err != nil {
				return err
			}
		}

		err = iptable.Append("nat", "POSTROUTING", "--out-interface", "eth0", "-j", "MASQUERADE")
		if err != nil {
			return err
		}
	} else {
		if cfg.IPFamilies == "IPv6" {
			// allow icmp6 for Neighbor Discovery Protocol
			err = iptable.AppendUnique("filter", "INPUT", "-i", forwardDevice, "-p", "icmpv6", "-j", "ACCEPT")
			if err != nil {
				return err
			}
		}
		err = iptable.AppendUnique("filter", "INPUT", "-m", "state", "--state", "RELATED,ESTABLISHED", "-i", forwardDevice, "-j", "ACCEPT")
		if err != nil {
			return err
		}
		err = iptable.AppendUnique("filter", "INPUT", "-i", forwardDevice, "-j", "DROP")
		if err != nil {
			return err
		}
	}
	return nil
}
