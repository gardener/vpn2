package vpn_server

import (
	"strconv"

	"github.com/coreos/go-iptables/iptables"
	"github.com/go-logr/logr"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
)

func SetIPTableRules(log logr.Logger, cfg config.VPNServer) error {
	if !cfg.IsHA {
		log.Info("setting up double NAT IPv4 iptables rules (seed-server)")
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
	}
	return nil
}
