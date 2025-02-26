// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpn_server

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/gardener/vpn2/pkg/openvpn"
)

func BuildValues(cfg config.VPNServer) (openvpn.SeedServerValues, error) {
	v := openvpn.SeedServerValues{
		StatusPath: cfg.StatusPath,
	}

	if len(cfg.VPNNetwork.IP) != 16 {
		return v, fmt.Errorf("VPN_NETWORK must be a IPv6 CIDR: %s", cfg.VPNNetwork)
	}
	if ones, _ := cfg.VPNNetwork.Mask.Size(); ones != constants.VPNNetworkMask {
		return v, fmt.Errorf("invalid prefix length for VPN_NETWORK, must be /%d, vpn network: %s", constants.VPNNetworkMask, cfg.VPNNetwork)
	}

	v.IsHA, v.VPNIndex = getHAInfo()

	switch v.IsHA {
	case true:
		v.Device = constants.TapDevice
		v.HAVPNClients = cfg.HAVPNClients
		v.OpenVPNNetwork = network.HAVPNTunnelNetwork(cfg.VPNNetwork.IP, v.VPNIndex)

		v.ShootNetworks = append(v.ShootNetworks, cfg.ServiceNetworks...)
		v.ShootNetworks = append(v.ShootNetworks, cfg.PodNetworks...)
		if len(cfg.NodeNetworks) != 0 && cfg.NodeNetworks[0].String() != "" {
			v.ShootNetworks = append(v.ShootNetworks, cfg.NodeNetworks...)
		}

	case false:
		v.Device = constants.TunnelDevice
		v.HAVPNClients = -1
		v.OpenVPNNetwork = cfg.VPNNetwork

		v.SeedPodNetwork = cfg.SeedPodNetwork
		v.ShootNetworks = append(v.ShootNetworks, network.ParseIPNet(constants.ShootNodeNetworkMapped))
		v.ShootNetworks = append(v.ShootNetworks, network.ParseIPNet(constants.ShootSvcNetworkMapped))
		v.ShootNetworks = append(v.ShootNetworks, network.ParseIPNet(constants.ShootPodNetworkMapped))
	}

	for _, shootNetwork := range v.ShootNetworks {
		if shootNetwork.IP.To4() != nil {
			v.ShootNetworksV4 = append(v.ShootNetworksV4, shootNetwork)
		} else {
			v.ShootNetworksV6 = append(v.ShootNetworksV6, shootNetwork)
		}
	}

	return v, nil
}

func getHAInfo() (bool, int) {
	podName, ok := os.LookupEnv("POD_NAME")
	if !ok {
		return false, 0
	}

	re := regexp.MustCompile(`.*-([0-2])$`)
	matches := re.FindStringSubmatch(podName)
	if len(matches) > 1 {
		index, _ := strconv.Atoi(matches[1])
		return true, index
	}
	return false, 0
}
