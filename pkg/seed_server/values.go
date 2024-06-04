// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed_server

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

func BuildValues(cfg config.SeedServer) (openvpn.SeedServerValues, error) {
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

	switch cfg.IPFamilies {
	case config.IPv4Family:
		if len(cfg.VPNNetwork.IP) != 4 {
			return v, fmt.Errorf("vpn network prefix is not v4 although v4 address family was specified")
		}
		if ones, _ := cfg.VPNNetwork.Mask.Size(); ones != 24 {
			return v, fmt.Errorf("invalid prefixlength of vpn network prefix, must be /24, vpn network: %s", cfg.VPNNetwork)
		}
		vpnNetworkBytes := copyIP(cfg.VPNNetwork.IP)
		switch v.IsHA {
		case true:
			vpnNetworkBytes[3] = byte(v.VPNIndex * 64)
			v.OpenVPNNetwork = network.CIDR{
				IP:   copyIP(vpnNetworkBytes),
				Mask: net.CIDRMask(26, 32),
			}
			vpnNetworkBytes[3] = byte(v.VPNIndex*64 + 8)
			v.IPv4PoolStartIP = vpnNetworkBytes.String()
			vpnNetworkBytes[3] = byte(v.VPNIndex*64 + 62)
			v.IPv4PoolEndIP = vpnNetworkBytes.String()
		case false:
			v.OpenVPNNetwork = cfg.VPNNetwork
			vpnNetworkBytes[3] = byte(10)
			v.IPv4PoolStartIP = vpnNetworkBytes.String()
			vpnNetworkBytes[3] = byte(254)
			v.IPv4PoolEndIP = vpnNetworkBytes.String()
		}

	case config.IPv6Family:
		if len(cfg.VPNNetwork.IP) != 16 {
			return v, fmt.Errorf("vpn network prefix is not v6 although v6 address family was specified")
		}
		if ones, _ := cfg.VPNNetwork.Mask.Size(); ones != 120 {
			return v, fmt.Errorf("invalid prefixlength of vpn network prefix, must be /120, vpn network: %s", cfg.VPNNetwork)
		}
		if v.IsHA {
			return v, fmt.Errorf("error: the highly-available VPN setup is only supported for IPv4 single-stack shoots but IPv6 address family was specified")
		}
		v.OpenVPNNetwork = cfg.VPNNetwork

	default:
		return v, fmt.Errorf("no valid IP address family, ip address family: %s", cfg.IPFamilies)
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
