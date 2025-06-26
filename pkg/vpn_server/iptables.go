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
		log.Info("setting up double NAT IPv4 iptables rules")
		ipTable, err := network.NewIPTables(log, iptables.ProtocolIPv4)
		if err != nil {
			return err
		}

		ipv4PodNetworks, ipv4ServiceNetworks, ipv4NodeNetworks, err := network.ShootNetworksForNetmap(cfg.ShootPodNetworks, cfg.ShootServiceNetworks, cfg.ShootNodeNetworks)
		if err != nil {
			return err
		}

		for _, nw := range ipv4PodNetworks {
			err = ipTable.AppendUnique("nat", "OUTPUT", "-m", "owner", "--gid-owner", strconv.Itoa(constants.EnvoyVPNGroupId), "-d", nw.String(), "-j", "NETMAP", "--to", constants.ShootPodNetworkMapped)
			if err != nil {
				return err
			}
		}
		for _, nw := range ipv4ServiceNetworks {
			err = ipTable.AppendUnique("nat", "OUTPUT", "-m", "owner", "--gid-owner", strconv.Itoa(constants.EnvoyVPNGroupId), "-d", nw.String(), "-j", "NETMAP", "--to", constants.ShootServiceNetworkMapped)
			if err != nil {
				return err
			}
		}
		for _, nw := range ipv4NodeNetworks {
			err = ipTable.AppendUnique("nat", "OUTPUT", "-m", "owner", "--gid-owner", strconv.Itoa(constants.EnvoyVPNGroupId), "-d", nw.String(), "-j", "NETMAP", "--to", constants.ShootNodeNetworkMapped)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
