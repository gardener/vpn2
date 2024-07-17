// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpn_server

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/gardener/vpn2/pkg/openvpn"
)

func BuildValues(cfg config.VPNServer) (openvpn.SeedServerValues, error) {
	v := openvpn.SeedServerValues{
		IPFamilies: cfg.IPFamilies,
		StatusPath: cfg.StatusPath,
	}

	v.ShootNetworks = append(v.ShootNetworks, cfg.ServiceNetwork)

	v.ShootNetworks = append(v.ShootNetworks, cfg.PodNetwork)

	if cfg.NodeNetwork.String() != "" {
		v.ShootNetworks = append(v.ShootNetworks, cfg.NodeNetwork)
	}

	v.IsHA, v.VPNIndex = getHAInfo()

	switch v.IsHA {
	case true:
		v.Device = "tap0"
		v.HAVPNClients = cfg.HAVPNClients
	case false:
		v.Device = "tun0"
		v.HAVPNClients = -1
	}

	if len(cfg.VPNNetwork.IP) != 16 {
		return v, fmt.Errorf("vpn network prefix must be v6")
	}
	if ones, _ := cfg.VPNNetwork.Mask.Size(); ones != 120 {
		return v, fmt.Errorf("invalid prefixlength of vpn network prefix, must be /120, vpn network: %s", cfg.VPNNetwork)
	}

	switch v.IsHA {
	case false:
		v.OpenVPNNetwork = cfg.VPNNetwork
		v.OpenVPNNetworkPool = network.CIDR{
			IP:   copyIP(cfg.VPNNetwork.IP),
			Mask: net.CIDRMask(121, 128),
		}
		v.OpenVPNNetworkPool.IP[15] += 128
	case true:
		v.OpenVPNNetwork = network.CIDR{
			IP:   copyIP(cfg.VPNNetwork.IP),
			Mask: net.CIDRMask(122, 128),
		}
		v.OpenVPNNetwork.IP[15] += byte(64 * v.VPNIndex)
		v.OpenVPNNetworkPool = network.CIDR{
			IP:   copyIP(v.OpenVPNNetwork.IP),
			Mask: net.CIDRMask(123, 128),
		}
		v.OpenVPNNetworkPool.IP[15] += 32
	}

	return v, nil
}

func copyIP(ip net.IP) net.IP {
	new := make(net.IP, len(ip))
	copy(new, ip)
	return new
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
