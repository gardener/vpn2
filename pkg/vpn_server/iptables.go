package vpn_server

import (
	"fmt"
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

		ipv4PodNetworks := network.GetByIPFamily(cfg.PodNetworks, network.IPv4Family)
		if len(ipv4PodNetworks) > 1 {
			return fmt.Errorf("exactly one IPv4 pod network is supported. IPv4 pod networks: %s", ipv4PodNetworks)
		}
		ipv4ServiceNetworks := network.GetByIPFamily(cfg.ServiceNetworks, network.IPv4Family)
		if len(ipv4ServiceNetworks) > 1 {
			return fmt.Errorf("exactly one IPv4 service network is supported. IPv4 service networks: %s", ipv4ServiceNetworks)
		}
		ipv4NodeNetworks := network.GetByIPFamily(cfg.NodeNetworks, network.IPv4Family)
		if len(ipv4NodeNetworks) > 1 {
			return fmt.Errorf("exactly one IPv4 node network is supported. IPv4 node networks: %s", ipv4NodeNetworks)
		}

		for _, nw := range ipv4PodNetworks {
			err = ipTable.AppendUnique("nat", "OUTPUT", "-m", "owner", "--uid-owner", strconv.Itoa(constants.EnvoyNonRootUserId), "-d", nw.String(), "-j", "NETMAP", "--to", constants.ShootPodNetworkMapped)
			if err != nil {
				return err
			}
		}
		for _, nw := range ipv4ServiceNetworks {
			err = ipTable.AppendUnique("nat", "OUTPUT", "-m", "owner", "--uid-owner", strconv.Itoa(constants.EnvoyNonRootUserId), "-d", nw.String(), "-j", "NETMAP", "--to", constants.ShootServiceNetworkMapped)
			if err != nil {
				return err
			}
		}
		for _, nw := range ipv4NodeNetworks {
			err = ipTable.AppendUnique("nat", "OUTPUT", "-m", "owner", "--uid-owner", strconv.Itoa(constants.EnvoyNonRootUserId), "-d", nw.String(), "-j", "NETMAP", "--to", constants.ShootNodeNetworkMapped)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
