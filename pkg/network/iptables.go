// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/coreos/go-iptables/iptables"
	"github.com/go-logr/logr"
)

// NewIPTables wraps the creation of IPTables to patch the path to the correct implementation binary.
// It has been introduced to avoid the risk that the command doesn't work due to missing kernel modules.
func NewIPTables(log logr.Logger, proto iptables.Protocol) (*iptables.IPTables, error) {
	for _, suffix := range []string{"legacy", "nft"} {
		if path := "/usr/sbin/iptables-" + suffix; iptablesWorks(path) {
			log.Info("using iptables backend " + suffix)
			return iptables.New(iptables.IPFamily(proto), iptables.Path(adjustPath(path, proto)))
		}
	}

	return nil, fmt.Errorf("could not find iptables backend")
}

func adjustPath(path string, proto iptables.Protocol) string {
	if proto == iptables.ProtocolIPv6 {
		return strings.ReplaceAll(path, "iptables-", "ip6tables-")
	}
	return path
}

func iptablesWorks(path string) bool {
	// check both iptables and ip6tables
	return exec.Command(path, "-L").Run() == nil && exec.Command(adjustPath(path, iptables.ProtocolIPv6), "-L").Run() == nil // #nosec: G204 -- Command line is completely static "/usr/sbin/(iptables|ip6tables)-(legacy|nft) -L".
}

// ShootNetworksForNetmap verifies that there is exactly one IPv4 pod, service, and node network in the provided lists.
// This is needed until more than one network is supported in the netmap iptables rules and more can be defined in the shoot spec.
// The function returns the IPv4 networks or an error if the requirements are not met.
func ShootNetworksForNetmap(ShootPodNetworks, ShootServiceNetworks, ShootNodeNetworks []CIDR) (ipv4PodNetworks []CIDR, ipv4ServiceNetworks []CIDR, ipv4NodeNetworks []CIDR, err error) {
	ipv4PodNetworks = GetByIPFamily(ShootPodNetworks, IPv4Family)
	if len(ipv4PodNetworks) > 1 {
		return nil, nil, nil, fmt.Errorf("exactly one IPv4 pod network is supported. IPv4 pod networks: %s", ipv4PodNetworks)
	}
	ipv4ServiceNetworks = GetByIPFamily(ShootServiceNetworks, IPv4Family)
	if len(ipv4ServiceNetworks) > 1 {
		return nil, nil, nil, fmt.Errorf("exactly one IPv4 service network is supported. IPv4 service networks: %s", ipv4ServiceNetworks)
	}
	ipv4NodeNetworks = GetByIPFamily(ShootNodeNetworks, IPv4Family)
	if len(ipv4NodeNetworks) > 1 {
		return nil, nil, nil, fmt.Errorf("exactly one IPv4 node network is supported. IPv4 node networks: %s", ipv4NodeNetworks)
	}
	return ipv4PodNetworks, ipv4ServiceNetworks, ipv4NodeNetworks, nil
}
