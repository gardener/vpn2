// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpn_client

import (
	"context"
	"fmt"
	"net"
	"os/exec"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/ippool"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/go-logr/logr"
	"github.com/vishvananda/netlink"
)

func ConfigureBonding(ctx context.Context, log logr.Logger, cfg *config.VPNClient) error {
	var addr *net.IPNet
	var targets []net.IP

	if cfg.IsShootClient {
		addr, targets = network.GetBondAddressAndTargetsShootClient(cfg.VPNNetwork.ToIPNet(), cfg.VPNClientIndex)
	} else {
		manager, err := ippool.NewPodIPPoolManager(cfg.Namespace, cfg.PodLabelSelector)
		if err != nil {
			return err
		}
		broker, err := ippool.NewIPAddressBroker(manager, cfg)
		if err != nil {
			return err
		}

		log.Info("acquiring ip address for bonding from kube-api server")
		acquiredIP, err := broker.AcquireIP(ctx)
		if err != nil {
			return fmt.Errorf("failed to acquire ip: %w", err)
		}
		ip := net.ParseIP(acquiredIP)
		if ip == nil {
			return fmt.Errorf("acquired ip %s is not a valid ipv6 nor ipv4", ip)
		}
		addr, targets = network.GetBondAddressAndTargetsSeedClient(ip, cfg.VPNNetwork.ToIPNet(), cfg.HAVPNClients)
	}

	for i := range cfg.HAVPNServers {
		linkName := fmt.Sprintf("tap%d", i)
		err := deleteLinkByName(linkName)
		if err != nil {
			return err
		}

		cmd := exec.CommandContext(ctx, "openvpn", "--mktun", "--dev", linkName)
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	// check if bond0 already exists and delete it if exists
	err := deleteLinkByName("bond0")
	if err != nil {
		return err
	}

	tab0Link, err := netlink.LinkByName("tap0")
	if err != nil {
		return fmt.Errorf("failed to get link tap0: %w", err)
	}

	// create bond0
	linkAttrs := netlink.NewLinkAttrs()
	bond := netlink.NewLinkBond(linkAttrs)
	// use bonding
	// - with active-backup mode
	// - activate ARP requests (but not used for monitoring as use_carrier=1 and arp_validate=none by default)
	// - using `primary tap0` to avoid ambiguity of selection if multiple devices are up (primary_reselect=always by default)
	// - using `num_grat_arp 5` as safeguard on switching device
	bond.Name = "bond0"
	bond.Mode = netlink.BOND_MODE_ACTIVE_BACKUP
	bond.FailOverMac = netlink.BOND_FAIL_OVER_MAC_ACTIVE
	bond.ArpInterval = 1000
	bond.ArpIpTargets = targets
	bond.ArpAllTargets = netlink.BOND_ARP_ALL_TARGETS_ANY
	bond.Primary = tab0Link.Attrs().Index
	bond.NumPeerNotif = 5

	if err = netlink.LinkAdd(bond); err != nil {
		return fmt.Errorf("failed to create bond0 link device: %w", err)
	}

	for i := range cfg.HAVPNServers {
		linkName := fmt.Sprintf("tap%d", i)

		link, err := netlink.LinkByName(linkName)
		if err != nil {
			return fmt.Errorf("failed to get link %s: %w", linkName, err)
		}

		err = netlink.LinkSetMaster(link, bond)
		if err != nil {
			return fmt.Errorf("failed to set bond0 as master for link %s: %w", linkName, err)
		}
	}

	err = netlink.LinkSetUp(bond)
	if err != nil {
		return fmt.Errorf("failed to up bond0 link: %w", err)
	}
	err = netlink.AddrAdd(bond, &netlink.Addr{IPNet: addr})
	if err != nil {
		return fmt.Errorf("failed to add address %s to bond0 link: %w", addr, err)
	}

	return nil
}

func deleteLinkByName(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return nil
		}
		return fmt.Errorf("failed to get link %s: %w", name, err)
	}

	if err = netlink.LinkDel(link); err != nil {
		return fmt.Errorf("failed to delete link %s: %w", name, err)
	}
	return nil
}
