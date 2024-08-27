// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpn_client

import (
	"github.com/coreos/go-iptables/iptables"
	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/go-logr/logr"
)

func SetIPTableRules(log logr.Logger, cfg config.VPNClient) error {
	forwardDevice := "tun0"
	if cfg.VPNServerIndex != "" {
		forwardDevice = constants.BondDevice
	}

	protocol := iptables.ProtocolIPv4
	if cfg.PrimaryIPFamily() == constants.IPv6Family {
		protocol = iptables.ProtocolIPv6
	}
	ipTable, err := network.NewIPTables(log, protocol)
	if err != nil {
		return err
	}

	if cfg.IsShootClient {
		if protocol == iptables.ProtocolIPv4 {
			err = ipTable.Append("filter", "FORWARD", "--in-interface", forwardDevice, "-j", "ACCEPT")
			if err != nil {
				return err
			}
		}

		err = ipTable.Append("nat", "POSTROUTING", "--out-interface", "eth0", "-j", "MASQUERADE")
		if err != nil {
			return err
		}
	} else {
		if protocol == iptables.ProtocolIPv6 {
			// allow icmp6 for Neighbor Discovery Protocol
			err = ipTable.AppendUnique("filter", "INPUT", "-i", forwardDevice, "-p", "icmpv6", "-j", "ACCEPT")
			if err != nil {
				return err
			}
		}
		err = ipTable.AppendUnique("filter", "INPUT", "-m", "state", "--state", "RELATED,ESTABLISHED", "-i", forwardDevice, "-j", "ACCEPT")
		if err != nil {
			return err
		}
		err = ipTable.AppendUnique("filter", "INPUT", "-i", forwardDevice, "-j", "DROP")
		if err != nil {
			return err
		}
	}
	return nil
}
