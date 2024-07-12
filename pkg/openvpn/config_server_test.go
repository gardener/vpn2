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
			cfgIPv4.OpenVPNNetwork = parseIPNet("192.168.123.0/26")
			cfgIPv4.IPv4PoolStartIP = "192.168.123.8"
			cfgIPv4.IPv4PoolEndIP = "192.168.123.62"
			cfgIPv4.StatusPath = "/srv/status/openvpn.status"
		}
	)

	BeforeEach(func() {
		cfgIPv4 = SeedServerValues{
			Device:          "tun0",
			IPv4PoolStartIP: "192.168.123.10",
			IPv4PoolEndIP:   "192.168.123.254",
			OpenVPNNetwork:  parseIPNet("192.168.123.0/24"),
			IsHA:            false,
			ShootNetworks: []network.CIDR{
				parseIPNet("100.64.0.0/13"),
				parseIPNet("100.96.0.0/11"),
				parseIPNet("10.0.1.0/24"),
			},
			IPFamilies: "IPv4",
		}
		cfgIPv6 = SeedServerValues{
			Device:         "tun0",
			OpenVPNNetwork: parseIPNet("2001:db8:10::/48"),
			IsHA:           false,
			ShootNetworks: []network.CIDR{
				parseIPNet("2001:db8:1::/48"),
				parseIPNet("2001:db8:2::/48"),
				parseIPNet("2001:db8:3::/48"),
			},
			IPFamilies: "IPv6",
		}
	})

	Describe("#GenerateOpenVPNConfig", func() {
		It("should generate correct openvpn.config for IPv4 default values", func() {
			content, err := generateSeedServerConfig(cfgIPv4)
			Expect(err).NotTo(HaveOccurred())

			Expect(content).To(ContainSubstring(`tls-auth "/srv/secrets/tlsauth/vpn.tlsauth" 0
`))
			Expect(content).To(ContainSubstring(`proto tcp4-server
server 192.168.123.0 255.255.255.0 nopool
ifconfig-pool 192.168.123.10 192.168.123.254
`))
			Expect(content).To(ContainSubstring(`
route 100.64.0.0 255.248.0.0
route 100.96.0.0 255.224.0.0
route 10.0.1.0 255.255.255.0
`))

			Expect(content).To(ContainSubstring(`dev tun0
`))

			Expect(content).To(ContainSubstring(`
script-security 2
up "/bin/seed-server firewall --mode up --device tun0"
down "/bin/seed-server firewall --mode down --device tun0"`))
		})

		It("should generate correct openvpn.config for IPv4 default values with HA", func() {
			prepareIPv4HA()
			content, err := generateSeedServerConfig(cfgIPv4)
			Expect(err).NotTo(HaveOccurred())

			Expect(content).To(ContainSubstring(`tls-auth "/srv/secrets/tlsauth/vpn.tlsauth" 0
`))
			Expect(content).To(ContainSubstring(`proto tcp4-server
server 192.168.123.0 255.255.255.192 nopool
ifconfig-pool 192.168.123.8 192.168.123.62
`))
			Expect(content).To(ContainSubstring(`
route 100.64.0.0 255.248.0.0
route 100.96.0.0 255.224.0.0
route 10.0.1.0 255.255.255.0
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
up "/bin/seed-server firewall --mode up --device tap0"
down "/bin/seed-server firewall --mode down --device tap0"`))

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
server-ipv6 2001:db8:10::/48
`))
			Expect(content).To(ContainSubstring(`
route-ipv6 2001:db8:1::/48
route-ipv6 2001:db8:2::/48
route-ipv6 2001:db8:3::/48
`))
			Expect(content).To(ContainSubstring(`
dev tun0
`))
			Expect(content).To(ContainSubstring(`
script-security 2
up "/bin/seed-server firewall --mode up --device tun0"
down "/bin/seed-server firewall --mode down --device tun0"`))
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

	Describe("#GenerateVPNShootClientHA", func() {
		It("should generate correct vpn-shoot-client for IPv4 HA", func() {
			prepareIPv4HA()
			content, err := generateConfigForClientHAFromServer(cfgIPv4, "192.168.123.2")
			Expect(err).NotTo(HaveOccurred())

			Expect(content).To(Equal(`ifconfig-push 192.168.123.2 255.255.255.192
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
