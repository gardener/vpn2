// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpn_client

import (
	"strings"

	"github.com/coreos/go-iptables/iptables"
	"github.com/go-logr/logr"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
)

func SetIPTableRules(log logr.Logger, cfg config.VPNClient) error {
	forwardDevice := constants.TunnelDevice
	if cfg.VPNServerIndex != "" {
		forwardDevice = constants.BondDevice
	}

	for _, family := range strings.Split(cfg.IPFamilies, ",") {
		protocol := iptables.ProtocolIPv4
		if family == constants.IPv6Family {
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
				if !cfg.IsHA {
					err = ipTable.AppendUnique("nat", "PREROUTING", "--in-interface", forwardDevice, "-d", constants.ShootPodNetworkMapped, "-j", "NETMAP", "--to", cfg.ShootPodNetwork.String())
					if err != nil {
						return err
					}
					err = ipTable.AppendUnique("nat", "POSTROUTING", "--out-interface", forwardDevice, "-s", cfg.ShootPodNetwork.String(), "-j", "NETMAP", "--to", constants.ShootPodNetworkMapped)
					if err != nil {
						return err
					}
					err = ipTable.AppendUnique("nat", "PREROUTING", "--in-interface", forwardDevice, "-d", constants.ShootSvcNetworkMapped, "-j", "NETMAP", "--to", cfg.ShootServiceNetwork.String())
					if err != nil {
						return err
					}
					err = ipTable.AppendUnique("nat", "POSTROUTING", "--out-interface", forwardDevice, "-s", cfg.ShootServiceNetwork.String(), "-j", "NETMAP", "--to", constants.ShootSvcNetworkMapped)
					if err != nil {
						return err
					}
					err = ipTable.AppendUnique("nat", "PREROUTING", "--in-interface", forwardDevice, "-d", constants.ShootNodeNetworkMapped, "-j", "NETMAP", "--to", cfg.ShootNodeNetwork.String())
					if err != nil {
						return err
					}
					err = ipTable.AppendUnique("nat", "POSTROUTING", "--out-interface", forwardDevice, "-s", cfg.ShootNodeNetwork.String(), "-j", "NETMAP", "--to", constants.ShootNodeNetworkMapped)
					if err != nil {
						return err
					}
				}
			}

			err = ipTable.Append("filter", "FORWARD", "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--clamp-mss-to-pmtu")
			if err != nil {
				return err
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
	}
	return nil
}
