// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpn_server

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/gardener/vpn2/pkg/openvpn"
)

func BuildValues(cfg config.VPNServer) (openvpn.SeedServerValues, error) {
	v := openvpn.SeedServerValues{
		StatusPath: cfg.StatusPath,
	}

	if cfg.VPNNetwork.IP == nil {
		return v, fmt.Errorf("VPN_NETWORK must be set")
	}
	if cfg.VPNNetwork.IsIPv4() {
		return v, fmt.Errorf("VPN_NETWORK must be a IPv6 CIDR: %s", cfg.VPNNetwork)
	}
	if ones, _ := cfg.VPNNetwork.Mask.Size(); ones != constants.VPNNetworkMask {
		return v, fmt.Errorf("invalid prefix length for VPN_NETWORK, must be /%d, vpn network: %s", constants.VPNNetworkMask, cfg.VPNNetwork)
	}

	v.IsHA, v.VPNIndex = getHAInfo(cfg)

	if v.IsHA != cfg.IsHA {
		return v, fmt.Errorf("IS_HA flag in config does not match HA info from pod name: IS_HA = %t, POD_NAME = %s", cfg.IsHA, cfg.PodName)
	}

	switch v.IsHA {
	case true:
		v.Device = constants.TapDevice
		v.HAVPNClients = cfg.HAVPNClients
		v.OpenVPNNetwork = network.HAVPNTunnelNetwork(cfg.VPNNetwork.IP, v.VPNIndex)

		//v.ShootNetworks = append(v.ShootNetworks, cfg.ServiceNetworks...)
		//v.ShootNetworks = append(v.ShootNetworks, cfg.PodNetworks...)
		//if len(cfg.NodeNetworks) != 0 && cfg.NodeNetworks[0].String() != "" {
		//	v.ShootNetworks = append(v.ShootNetworks, cfg.NodeNetworks...)
		//}
	case false:
		v.Device = constants.TunnelDevice
		v.HAVPNClients = -1
		v.OpenVPNNetwork = cfg.VPNNetwork
	}

	v.SeedPodNetwork = cfg.SeedPodNetwork
	// IPv4 networks are mapped to 240/4, IPv6 networks are kept as is
	for _, serviceNetwork := range cfg.ServiceNetworks {
		if serviceNetwork.IP.To4() != nil {
			v.ShootNetworks = append(v.ShootNetworks, network.ParseIPNetIgnoreError(constants.ShootServiceNetworkMapped))
		} else {
			v.ShootNetworks = append(v.ShootNetworks, serviceNetwork)
		}
	}
	for _, podNetwork := range cfg.PodNetworks {
		if podNetwork.IP.To4() != nil {
			v.ShootNetworks = append(v.ShootNetworks, network.ParseIPNetIgnoreError(constants.ShootPodNetworkMapped))
		} else {
			v.ShootNetworks = append(v.ShootNetworks, podNetwork)
		}
	}
	for _, nodeNetwork := range cfg.NodeNetworks {
		if nodeNetwork.IP.To4() != nil {
			v.ShootNetworks = append(v.ShootNetworks, network.ParseIPNetIgnoreError(constants.ShootNodeNetworkMapped))
		} else {
			v.ShootNetworks = append(v.ShootNetworks, nodeNetwork)
		}
	}
	//}

	// remove possible duplicates. sort, then compact.
	slices.SortFunc(v.ShootNetworks, func(a, b network.CIDR) int {
		return strings.Compare(a.String(), b.String())
	})
	v.ShootNetworks = slices.CompactFunc(v.ShootNetworks, func(a network.CIDR, b network.CIDR) bool {
		return a.Equal(b)
	})

	for _, shootNetwork := range v.ShootNetworks {
		if shootNetwork.IP.To4() != nil {
			v.ShootNetworksV4 = append(v.ShootNetworksV4, shootNetwork)
		} else {
			v.ShootNetworksV6 = append(v.ShootNetworksV6, shootNetwork)
		}
	}

	return v, nil
}

func getHAInfo(cfg config.VPNServer) (bool, int) {
	podName := cfg.PodName
	if podName == "" {
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
