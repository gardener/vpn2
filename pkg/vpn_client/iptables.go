// SPDX-FileCopyrightText: Contributors to the Gardener project
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

// SetMarkRules installs mangle OUTPUT rules that distribute new outbound connections to shoot
// networks across shoot clients by assigning per-connection fwmarks. Only traffic destined for
// shootNetworks is marked; all other traffic is unaffected. The mark is persisted in conntrack so
// all packets of an established connection carry the same mark without re-evaluation. Rules are
// idempotent (AppendUnique). numClients must equal the number of ip6tnl tunnel devices created by
// setup.
func SetMarkRules(log logr.Logger, shootNetworks []network.CIDR, numClients int) error {
	for _, protocol := range []iptables.Protocol{iptables.ProtocolIPv4, iptables.ProtocolIPv6} {
		ipt, err := network.NewIPTables(log, protocol)
		if err != nil {
			return err
		}

		// Ensure the chain exists; ignore the error if it already does.
		_ = ipt.NewChain("mangle", constants.ShootMarkChain)

		// Assign marks to new connections using random distribution. Rules are guarded by
		// "-m mark --mark 0" so only the first matching rule fires: once a mark is set,
		// subsequent rules are skipped. For N clients, the i-th rule fires with probability
		// 1/(N-i) given that no earlier rule matched, so the overall distribution is uniform.
		for i := range numClients {
			mark := strconv.Itoa(constants.FwmarkBase + i)
			remaining := numClients - i
			// Only fire if this connection has not been marked yet by a previous rule.
			args := []string{"-m", "conntrack", "--ctstate", "NEW", "-m", "mark", "--mark", "0"}
			if remaining > 1 {
				prob := fmt.Sprintf("%f", 1.0/float64(remaining))
				args = append(args, "-m", "statistic", "--mode", "random", "--probability", prob)
			}
			args = append(args, "-j", "MARK", "--set-mark", mark)
			if err := ipt.AppendUnique("mangle", constants.ShootMarkChain, args...); err != nil {
				return fmt.Errorf("error adding mark rule for client %d: %w", i, err)
			}
		}
		// Persist the mark into conntrack so subsequent packets of the same connection reuse it.
		if err := ipt.AppendUnique("mangle", constants.ShootMarkChain,
			"-m", "conntrack", "--ctstate", "NEW",
			"-j", "CONNMARK", "--save-mark",
		); err != nil {
			return fmt.Errorf("error adding CONNMARK save rule: %w", err)
		}
		// Restore the mark on established/related packets so they follow the same table as the SYN.
		if err := ipt.AppendUnique("mangle", constants.ShootMarkChain,
			"-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED",
			"-j", "CONNMARK", "--restore-mark",
		); err != nil {
			return fmt.Errorf("error adding CONNMARK restore rule: %w", err)
		}

		// Jump into the chain only for shoot-destined traffic so seed and other traffic is unaffected.
		isIPv6 := protocol == iptables.ProtocolIPv6
		for _, cidr := range shootNetworks {
			networkIsIPv6 := cidr.IP.To4() == nil
			if networkIsIPv6 != isIPv6 {
				continue
			}
			if err := ipt.AppendUnique("mangle", "OUTPUT",
				"-d", cidr.String(),
				"-j", constants.ShootMarkChain,
			); err != nil {
				return fmt.Errorf("error adding OUTPUT jump rule for %s: %w", cidr, err)
			}
		}
	}
	return nil
}

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

			err = ipTable.AppendUnique("mangle", "POSTROUTING", "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--clamp-mss-to-pmtu")
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
