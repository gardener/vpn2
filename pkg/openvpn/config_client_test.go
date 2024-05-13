package openvpn

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("#ClientConfig", func() {

	Describe("#GenerateClientConfig", func() {
		Context("ipv4 non HA running in seed config", func() {
			cfg := ClientValues{
				Endpoint:       "123.123.0.0",
				VPNClientIndex: -1,
				IPFamilies:     "IPv4",
				OpenVPNPort:    1143,
				IsShootClient:  false,
			}
			content, err := generateClientConfig(cfg)
			It("does not error creating the template", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("generated config contain check", func() {
				It("proto tcp4-client", func() {
					Expect(content).To(ContainSubstring(`proto tcp4-client`))
				})

				It("tls config", func() {
					Expect(content).To(ContainSubstring(`key /srv/secrets/vpn-client/tls.key
cert /srv/secrets/vpn-client/tls.crt
ca /srv/secrets/vpn-client/ca.crt`))
				})
			})
		})

		Context("ipv4 non HA running in shoot config", func() {
			cfg := ClientValues{
				Endpoint:          "123.123.0.0",
				VPNClientIndex:    -1,
				IPFamilies:        "IPv4",
				OpenVPNPort:       1143,
				ReversedVPNHeader: "invalid-host",
				IsShootClient:     true,
			}

			content, err := generateClientConfig(cfg)
			It("does not error creating the template", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("generated config contain check", func() {
				It("proto tcp4-client", func() {
					Expect(content).To(ContainSubstring(`proto tcp4-client`))
				})
				It("tls config", func() {
					Expect(content).To(ContainSubstring(`
key /srv/secrets/vpn-client/tls.key
cert /srv/secrets/vpn-client/tls.crt
ca /srv/secrets/vpn-client/ca.crt
`))
				})
				It("has http proxy options", func() {
					Expect(content).To(ContainSubstring(`
http-proxy 123.123.0.0 1143
http-proxy-option CUSTOM-HEADER Reversed-VPN invalid-host`))
				})
			})

		})

		Context("ipv4 HA config", func() {
			cfg := ClientValues{
				Endpoint:          "123.123.0.0",
				VPNClientIndex:    0,
				IPFamilies:        "IPv4",
				OpenVPNPort:       1143,
				ReversedVPNHeader: "invalid-host",
				IsShootClient:     true,
			}

			content, err := generateClientConfig(cfg)
			It("does not error creating the template", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("generated config contain check", func() {
				It("proto tcp4-client", func() {
					Expect(content).To(ContainSubstring(`proto tcp4-client`))
				})
				It("tls config", func() {
					Expect(content).To(ContainSubstring(`
key /srv/secrets/vpn-client-0/tls.key
cert /srv/secrets/vpn-client-0/tls.crt
ca /srv/secrets/vpn-client-0/ca.crt
`))
				})
			})
		})

		Context("ipv6 non HA config", func() {
			cfg := ClientValues{
				Endpoint:       "123.123.0.0",
				VPNClientIndex: -1,
				IPFamilies:     "IPv6",
				OpenVPNPort:    1143,
			}

			content, err := generateClientConfig(cfg)
			It("does not error creating the template", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("generated config contain check", func() {
				It("proto tcp6-client", func() {
					Expect(content).To(ContainSubstring(`proto tcp6-client`))
				})
			})
		})
	})
})
