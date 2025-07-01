// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpn_client

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/coreos/go-iptables/iptables"
	"github.com/go-logr/logr"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
)

func SetIPTableRules(log logr.Logger, cfg config.VPNClient) error {
	forwardDevice := constants.TunnelDevice
	if cfg.VPNServerIndex != "" {
		// we don't know the name of the bond0ip6tnl devices ahead of time, so we use a wildcard
		forwardDevice = fmt.Sprintf("%s+", constants.BondDevice)
	}

	// In HA mode, we only set up double NAT rules if there is an overlap between the seed pod network and the shoot networks.
	overlap := network.OverLapAny(cfg.SeedPodNetwork, slices.Concat(cfg.ShootPodNetworks, cfg.ShootServiceNetworks, cfg.ShootNodeNetworks)...)

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
				err = ipTable.AppendUnique("filter", "FORWARD", "--in-interface", forwardDevice, "-j", "ACCEPT")
				if err != nil {
					return err
				}
				if !cfg.IsHA || cfg.IsHA && overlap {
					log.Info("setting up double NAT IPv4 iptables rules (shoot)")
					ipv4PodNetworkMappings, ipv4ServiceNetworkMappings, ipv4NodeNetworkMappings, err := network.ShootNetworksForNetmap(cfg.ShootPodNetworks, cfg.ShootServiceNetworks, cfg.ShootNodeNetworks)
					if err != nil {
						return err
					}

					for src, dst := range ipv4PodNetworkMappings {
						err = ipTable.AppendUnique("nat", "PREROUTING", "--in-interface", forwardDevice, "-d", dst.String(), "-j", "NETMAP", "--to", src.String())
						if err != nil {
							return err
						}
						err = ipTable.AppendUnique("nat", "POSTROUTING", "--out-interface", forwardDevice, "-s", src.String(), "-j", "NETMAP", "--to", dst.String())
						if err != nil {
							return err
						}
					}
					for src, dst := range ipv4ServiceNetworkMappings {
						err = ipTable.AppendUnique("nat", "PREROUTING", "--in-interface", forwardDevice, "-d", dst.String(), "-j", "NETMAP", "--to", src.String())
						if err != nil {
							return err
						}
						err = ipTable.AppendUnique("nat", "POSTROUTING", "--out-interface", forwardDevice, "-s", src.String(), "-j", "NETMAP", "--to", dst.String())
						if err != nil {
							return err
						}
					}
					for src, dst := range ipv4NodeNetworkMappings {
						err = ipTable.AppendUnique("nat", "PREROUTING", "--in-interface", forwardDevice, "-d", dst.String(), "-j", "NETMAP", "--to", src.String())
						if err != nil {
							return err
						}
						err = ipTable.AppendUnique("nat", "POSTROUTING", "--out-interface", forwardDevice, "-s", src.String(), "-j", "NETMAP", "--to", dst.String())
						if err != nil {
							return err
						}
					}
				}
			}

			err = ipTable.AppendUnique("filter", "FORWARD", "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--clamp-mss-to-pmtu")
			if err != nil {
				return err
			}

			err = ipTable.AppendUnique("nat", "POSTROUTING", "--out-interface", "eth0", "-j", "MASQUERADE")
			if err != nil {
				return err
			}
		} else {
			if protocol == iptables.ProtocolIPv4 {
				// Seed client only exists in HA VPN mode, so we don't need to check for IsHA
				if overlap {
					log.Info("setting up double NAT IPv4 iptables rules (seed)")
					ipTable, err := network.NewIPTables(log, iptables.ProtocolIPv4)
					if err != nil {
						return err
					}

					ipv4PodNetworkMappings, ipv4ServiceNetworkMappings, ipv4NodeNetworkMappings, err := network.ShootNetworksForNetmap(cfg.ShootPodNetworks, cfg.ShootServiceNetworks, cfg.ShootNodeNetworks)
					if err != nil {
						return err
					}

					for src, dst := range ipv4PodNetworkMappings {
						err = ipTable.AppendUnique("nat", "OUTPUT", "-m", "owner", "--gid-owner", strconv.Itoa(constants.EnvoyVPNGroupId), "-d", src.String(), "-j", "NETMAP", "--to", dst.String())
						if err != nil {
							return err
						}
					}
					for src, dst := range ipv4ServiceNetworkMappings {
						err = ipTable.AppendUnique("nat", "OUTPUT", "-m", "owner", "--gid-owner", strconv.Itoa(constants.EnvoyVPNGroupId), "-d", src.String(), "-j", "NETMAP", "--to", dst.String())
						if err != nil {
							return err
						}
					}
					for src, dst := range ipv4NodeNetworkMappings {
						err = ipTable.AppendUnique("nat", "OUTPUT", "-m", "owner", "--gid-owner", strconv.Itoa(constants.EnvoyVPNGroupId), "-d", src.String(), "-j", "NETMAP", "--to", dst.String())
						if err != nil {
							return err
						}
					}

					if cfg.SeedPodNetwork.IsIPv4() {
						err = ipTable.AppendUnique("nat", "PREROUTING", "--in-interface", forwardDevice, "-d", constants.SeedPodNetworkMapped, "-j", "NETMAP", "--to", cfg.SeedPodNetwork.String())
						if err != nil {
							return err
						}
						err = ipTable.AppendUnique("nat", "POSTROUTING", "--out-interface", forwardDevice, "-s", cfg.SeedPodNetwork.String(), "-j", "NETMAP", "--to", constants.SeedPodNetworkMapped)
						if err != nil {
							return err
						}
					}
				}
			}
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
