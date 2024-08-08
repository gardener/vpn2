// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package openvpn

import (
	"net"

	"github.com/gardener/vpn2/pkg/network"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("#SeedServerConfig", func() {
	var (
		cfgIPv4 SeedServerValues
		cfgIPv6 SeedServerValues

		prepareIPv4HA = func() {
			cfgIPv4.IsHA = true
			cfgIPv4.Device = "tap0"
			cfgIPv4.OpenVPNNetwork = parseIPNet("fd8f:6d53:b97a:7777::/120")
			cfgIPv4.StatusPath = "/srv/status/openvpn.status"
		}
	)

	BeforeEach(func() {
		cfgIPv4 = SeedServerValues{
			Device:         "tun0",
			OpenVPNNetwork: parseIPNet("fd8f:6d53:b97a:7777::/120"),
			IsHA:           false,
			ShootNetworks: []network.CIDR{
				parseIPNet("100.64.0.0/13"),
				parseIPNet("100.96.0.0/11"),
				parseIPNet("10.0.1.0/24"),
			},
			IPFamily: "IPv4",
		}
		cfgIPv6 = SeedServerValues{
			Device:         "tun0",
			OpenVPNNetwork: parseIPNet("fd8f:6d53:b97a:7777::/120"),
			IsHA:           false,
			ShootNetworks: []network.CIDR{
				parseIPNet("2001:db8:1::/48"),
				parseIPNet("2001:db8:2::/48"),
				parseIPNet("2001:db8:3::/48"),
			},
			IPFamily: "IPv6",
		}
	})

	Describe("#GenerateOpenVPNConfig", func() {
		It("should generate correct openvpn.config for IPv4 default values", func() {
			content, err := generateSeedServerConfig(cfgIPv4)
			Expect(err).NotTo(HaveOccurred())

			Expect(content).To(ContainSubstring(`tls-auth "/srv/secrets/tlsauth/vpn.tlsauth" 0
`))
			Expect(content).To(ContainSubstring(`proto tcp4-server

server-ipv6 fd8f:6d53:b97a:7777::/120
`))

			Expect(content).To(ContainSubstring(`dev tun0
`))

			Expect(content).To(ContainSubstring(`
script-security 2
up "/bin/vpn-server firewall --mode up --device tun0 --shoot-network=100.64.0.0/13 --shoot-network=100.96.0.0/11 --shoot-network=10.0.1.0/24"
down "/bin/vpn-server firewall --mode down --device tun0"`))
		})

		It("should generate correct openvpn.config for IPv4 default values with HA", func() {
			prepareIPv4HA()
			content, err := generateSeedServerConfig(cfgIPv4)
			Expect(err).NotTo(HaveOccurred())

			Expect(content).To(ContainSubstring(`tls-auth "/srv/secrets/tlsauth/vpn.tlsauth" 0
`))
			Expect(content).To(ContainSubstring(`proto tcp4-server

server-ipv6 fd8f:6d53:b97a:7777::/120
`))

			Expect(content).To(ContainSubstring(`
client-to-client
duplicate-cn
`))

			Expect(content).To(ContainSubstring(`
dev tap0
`))

			Expect(content).To(ContainSubstring(`
script-security 2
up "/bin/vpn-server firewall --mode up --device tap0 --shoot-network=100.64.0.0/13 --shoot-network=100.96.0.0/11 --shoot-network=10.0.1.0/24"
down "/bin/vpn-server firewall --mode down --device tap0"`))

			Expect(content).To(ContainSubstring(`
status /srv/status/openvpn.status 15
status-version 2`))
		})

		It("should generate correct openvpn.config for IPv6 default values", func() {
			content, err := generateSeedServerConfig(cfgIPv6)
			Expect(err).NotTo(HaveOccurred())

			Expect(content).To(ContainSubstring(`tls-auth "/srv/secrets/tlsauth/vpn.tlsauth" 0
`))
			Expect(content).To(ContainSubstring(`proto tcp6-server

server-ipv6 fd8f:6d53:b97a:7777::/120
`))
			Expect(content).To(ContainSubstring(`
dev tun0
`))
			Expect(content).To(ContainSubstring(`
script-security 2
up "/bin/vpn-server firewall --mode up --device tun0 --shoot-network=2001:db8:1::/48 --shoot-network=2001:db8:2::/48 --shoot-network=2001:db8:3::/48"
down "/bin/vpn-server firewall --mode down --device tun0"`))
		})
	})

	Describe("#GenerateVPNShootClient", func() {
		It("should generate correct vpn-shoot-client for IPv4 default values", func() {
			content, err := generateConfigForClientFromServer(cfgIPv4)
			Expect(err).NotTo(HaveOccurred())

			Expect(content).To(Equal(`
iroute 100.64.0.0 255.248.0.0
iroute 100.96.0.0 255.224.0.0
iroute 10.0.1.0 255.255.255.0
`))
		})

		It("should generate correct vpn-shoot-client for IPv4 default values with HA", func() {
			prepareIPv4HA()
			content, err := generateConfigForClientFromServer(cfgIPv4)
			Expect(err).NotTo(HaveOccurred())

			Expect(content).To(Equal(`
iroute 100.64.0.0 255.248.0.0
iroute 100.96.0.0 255.224.0.0
iroute 10.0.1.0 255.255.255.0
`))
		})

		It("should generate correct vpn-shoot-client for IPv6 default values", func() {
			content, err := generateConfigForClientFromServer(cfgIPv6)
			Expect(err).NotTo(HaveOccurred())

			Expect(content).To(Equal(`
iroute-ipv6 2001:db8:1::/48
iroute-ipv6 2001:db8:2::/48
iroute-ipv6 2001:db8:3::/48
`))
		})
	})
})

func parseIPNet(cidr string) network.CIDR {
	_, prefix, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	return network.CIDR(*prefix)
}
