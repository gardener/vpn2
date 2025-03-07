// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package openvpn

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/vpn2/pkg/network"
)

var _ = Describe("#SeedServerConfig", func() {
	var (
		cfgIPv4      SeedServerValues
		cfgIPv6      SeedServerValues
		cfgDualStack SeedServerValues

		prepareIPv4HA = func() {
			cfgIPv4.IsHA = true
			cfgIPv4.Device = "tap0"
			cfgIPv4.OpenVPNNetwork = network.ParseIPNet("fd8f:6d53:b97a:7777::/96")
			cfgIPv4.StatusPath = "/srv/status/openvpn.status"
		}
	)

	BeforeEach(func() {
		cfgIPv4 = SeedServerValues{
			Device:         "tun0",
			OpenVPNNetwork: network.ParseIPNet("fd8f:6d53:b97a:7777::/96"),
			IsHA:           false,
			ShootNetworks: []network.CIDR{
				network.ParseIPNet("100.64.0.0/13"),
				network.ParseIPNet("100.96.0.0/11"),
				network.ParseIPNet("10.0.1.0/24"),
			},
			ShootNetworksV4: []network.CIDR{
				network.ParseIPNet("100.64.0.0/13"),
				network.ParseIPNet("100.96.0.0/11"),
				network.ParseIPNet("10.0.1.0/24"),
			},
			SeedPodNetworkV4: network.ParseIPNet("100.64.0.0/12"),
		}
		cfgIPv6 = SeedServerValues{
			Device:         "tun0",
			OpenVPNNetwork: network.ParseIPNet("fd8f:6d53:b97a:7777::/96"),
			IsHA:           false,
			ShootNetworks: []network.CIDR{
				network.ParseIPNet("2001:db8:1::/48"),
				network.ParseIPNet("2001:db8:2::/48"),
				network.ParseIPNet("2001:db8:3::/48"),
			},
			ShootNetworksV6: []network.CIDR{
				network.ParseIPNet("2001:db8:1::/48"),
				network.ParseIPNet("2001:db8:2::/48"),
				network.ParseIPNet("2001:db8:3::/48"),
			},
			SeedPodNetworkV4: network.ParseIPNet("100.64.0.0/12"),
		}
		cfgDualStack = SeedServerValues{
			Device:         "tun0",
			OpenVPNNetwork: network.ParseIPNet("fd8f:6d53:b97a:7777::/96"),
			IsHA:           false,
			ShootNetworks: []network.CIDR{
				network.ParseIPNet("100.64.0.0/13"),
				network.ParseIPNet("100.96.0.0/11"),
				network.ParseIPNet("10.0.1.0/24"),
				network.ParseIPNet("2001:db8:1::/48"),
				network.ParseIPNet("2001:db8:2::/48"),
				network.ParseIPNet("2001:db8:3::/48"),
			},
			ShootNetworksV4: []network.CIDR{
				network.ParseIPNet("100.64.0.0/13"),
				network.ParseIPNet("100.96.0.0/11"),
				network.ParseIPNet("10.0.1.0/24"),
			},
			ShootNetworksV6: []network.CIDR{
				network.ParseIPNet("2001:db8:1::/48"),
				network.ParseIPNet("2001:db8:2::/48"),
				network.ParseIPNet("2001:db8:3::/48"),
			},
			SeedPodNetworkV4: network.ParseIPNet("100.64.0.0/12"),
		}
	})

	Describe("#GenerateOpenVPNConfig", func() {
		It("should generate correct openvpn.config for IPv4 default values", func() {
			content, err := generateSeedServerConfig(cfgIPv4)
			Expect(err).NotTo(HaveOccurred())

			Expect(content).To(ContainSubstring(`tls-auth "/srv/secrets/tlsauth/vpn.tlsauth" 0
`))
			Expect(content).To(ContainSubstring(`proto tcp6-server

server-ipv6 fd8f:6d53:b97a:7777::/96
`))

			Expect(content).To(ContainSubstring(`dev tun0
`))

			Expect(content).To(ContainSubstring(`
script-security 2
up "/bin/vpn-server firewall --mode up --device tun0 --shoot-network=100.64.0.0/13,100.96.0.0/11,10.0.1.0/24 --seed-pod-network-v4=100.64.0.0/12"
down "/bin/vpn-server firewall --mode down --device tun0"`))
			Expect(content).To(HaveNoLineLongerThan(OpenVPNConfigMaxLineLength))
		})

		It("should generate correct openvpn.config for IPv4 default values with HA", func() {
			prepareIPv4HA()
			content, err := generateSeedServerConfig(cfgIPv4)
			Expect(err).NotTo(HaveOccurred())

			Expect(content).To(ContainSubstring(`tls-auth "/srv/secrets/tlsauth/vpn.tlsauth" 0
`))
			Expect(content).To(ContainSubstring(`proto tcp6-server

server-ipv6 fd8f:6d53:b97a:7777::/96
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
up "/bin/vpn-server firewall --mode up --device tap0 --shoot-network=100.64.0.0/13,100.96.0.0/11,10.0.1.0/24 --seed-pod-network-v4=100.64.0.0/12"
down "/bin/vpn-server firewall --mode down --device tap0"`))

			Expect(content).To(ContainSubstring(`
status /srv/status/openvpn.status 15
status-version 2`))
			Expect(content).To(HaveNoLineLongerThan(OpenVPNConfigMaxLineLength))
		})

		It("should generate correct openvpn.config for IPv6 default values", func() {
			content, err := generateSeedServerConfig(cfgIPv6)
			Expect(err).NotTo(HaveOccurred())

			Expect(content).To(ContainSubstring(`tls-auth "/srv/secrets/tlsauth/vpn.tlsauth" 0
`))
			Expect(content).To(ContainSubstring(`proto tcp6-server

server-ipv6 fd8f:6d53:b97a:7777::/96
`))
			Expect(content).To(ContainSubstring(`
dev tun0
`))
			Expect(content).To(ContainSubstring(`
script-security 2
up "/bin/vpn-server firewall --mode up --device tun0 --shoot-network=2001:db8:1::/48,2001:db8:2::/48,2001:db8:3::/48 --seed-pod-network-v4=100.64.0.0/12"
down "/bin/vpn-server firewall --mode down --device tun0"`))
			Expect(content).To(HaveNoLineLongerThan(OpenVPNConfigMaxLineLength))
		})

		It("should generate correct openvpn.config for dual stack values", func() {
			content, err := generateSeedServerConfig(cfgDualStack)
			Expect(err).NotTo(HaveOccurred())

			Expect(content).To(ContainSubstring(`tls-auth "/srv/secrets/tlsauth/vpn.tlsauth" 0
`))
			Expect(content).To(ContainSubstring(`proto tcp6-server

server-ipv6 fd8f:6d53:b97a:7777::/96
`))
			Expect(content).To(ContainSubstring(`
dev tun0
`))
			Expect(content).To(ContainSubstring(`
script-security 2
up "/bin/vpn-server firewall --mode up --device tun0 --shoot-network=100.64.0.0/13,100.96.0.0/11,10.0.1.0/24,2001:db8:1::/48,2001:db8:2::/48,2001:db8:3::/48 --seed-pod-network-v4=100.64.0.0/12"
down "/bin/vpn-server firewall --mode down --device tun0"`))
			Expect(content).To(HaveNoLineLongerThan(OpenVPNConfigMaxLineLength))
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

		It("should generate correct vpn-shoot-client for dual stack values", func() {
			content, err := generateConfigForClientFromServer(cfgDualStack)
			Expect(err).NotTo(HaveOccurred())

			Expect(content).To(Equal(`
iroute 100.64.0.0 255.248.0.0
iroute 100.96.0.0 255.224.0.0
iroute 10.0.1.0 255.255.255.0
iroute-ipv6 2001:db8:1::/48
iroute-ipv6 2001:db8:2::/48
iroute-ipv6 2001:db8:3::/48
`))
		})
	})
})
