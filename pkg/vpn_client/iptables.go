// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpn_client

import (
	"fmt"

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

	for _, family := range cfg.IPFamilies {
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
					log.Info("setting up double NAT IPv4 iptables rules")
					ipv4PodNetworks := network.GetByIPFamily(cfg.ShootPodNetworks, network.IPv4Family)
					if len(ipv4PodNetworks) > 1 {
						return fmt.Errorf("exactly one IPv4 pod network is supported. IPv4 pod networks: %s", ipv4PodNetworks)
					}
					ipv4ServiceNetworks := network.GetByIPFamily(cfg.ShootServiceNetworks, network.IPv4Family)
					if len(ipv4ServiceNetworks) > 1 {
						return fmt.Errorf("exactly one IPv4 service network is supported. IPv4 service networks: %s", ipv4ServiceNetworks)
					}
					ipv4NodeNetworks := network.GetByIPFamily(cfg.ShootNodeNetworks, network.IPv4Family)
					if len(ipv4NodeNetworks) > 1 {
						return fmt.Errorf("exactly one IPv4 node network is supported. IPv4 node networks: %s", ipv4NodeNetworks)
					}

					for _, nw := range ipv4PodNetworks {
						err = ipTable.AppendUnique("nat", "PREROUTING", "--in-interface", forwardDevice, "-d", constants.ShootPodNetworkMapped, "-j", "NETMAP", "--to", nw.String())
						if err != nil {
							return err
						}
						err = ipTable.AppendUnique("nat", "POSTROUTING", "--out-interface", forwardDevice, "-s", nw.String(), "-j", "NETMAP", "--to", constants.ShootPodNetworkMapped)
						if err != nil {
							return err
						}
					}
					for _, nw := range ipv4ServiceNetworks {
						err = ipTable.AppendUnique("nat", "PREROUTING", "--in-interface", forwardDevice, "-d", constants.ShootServiceNetworkMapped, "-j", "NETMAP", "--to", nw.String())
						if err != nil {
							return err
						}
						err = ipTable.AppendUnique("nat", "POSTROUTING", "--out-interface", forwardDevice, "-s", nw.String(), "-j", "NETMAP", "--to", constants.ShootServiceNetworkMapped)
						if err != nil {
							return err
						}
					}
					for _, nw := range ipv4NodeNetworks {
						err = ipTable.AppendUnique("nat", "PREROUTING", "--in-interface", forwardDevice, "-d", constants.ShootNodeNetworkMapped, "-j", "NETMAP", "--to", nw.String())
						if err != nil {
							return err
						}
						err = ipTable.AppendUnique("nat", "POSTROUTING", "--out-interface", forwardDevice, "-s", nw.String(), "-j", "NETMAP", "--to", constants.ShootNodeNetworkMapped)
						if err != nil {
							return err
						}
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
