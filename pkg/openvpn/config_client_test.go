// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

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
				IPFamily:       "IPv4",
				OpenVPNPort:    1143,
				IsShootClient:  false,
				Protocol:       "tcp",
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
				Endpoint:             "123.123.0.0",
				VPNClientIndex:       -1,
				IPFamily:             "IPv4",
				OpenVPNPort:          1143,
				ReversedVPNHeader:    "invalid-host",
				ReversedVPNHeaderKey: "Reversed-VPN",
				IsShootClient:        true,
				SeedPodNetwork:       "10.123.0.0/19",
				Protocol:             "tcp",
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
				It("adds route for seed pod network", func() {
					Expect(content).To(ContainSubstring(`
script-security 2
up "/bin/sh -c '/sbin/ip route replace 10.123.0.0/19 dev $1' -- "
`))
				})
			})
		})

		Context("ipv4 non-HA shoot client config", func() {
			cfg := ClientValues{
				Endpoint:          "123.123.0.0",
				VPNClientIndex:    0,
				IPFamily:          "IPv4",
				OpenVPNPort:       1143,
				ReversedVPNHeader: "invalid-host",
				IsShootClient:     true,
				SeedPodNetwork:    "10.123.0.0/19",
				Protocol:          "tcp",
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

				It("adds route for seed pod network", func() {
					Expect(content).To(ContainSubstring(`
script-security 2
up "/bin/sh -c '/sbin/ip route replace 10.123.0.0/19 dev $1' -- "
`))
				})
			})
		})

		Context("ipv4 HA shoot client config", func() {
			cfg := ClientValues{
				Endpoint:          "123.123.0.0",
				VPNClientIndex:    0,
				IPFamily:          "IPv4",
				OpenVPNPort:       1143,
				ReversedVPNHeader: "invalid-host",
				IsShootClient:     true,
				IsHA:              true,
				SeedPodNetwork:    "10.123.0.0/19",
				Protocol:          "tcp",
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
				IPFamily:       "IPv6",
				OpenVPNPort:    1143,
				Protocol:       "tcp",
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

		Context("dual-stack non HA config", func() {
			cfg := ClientValues{
				Endpoint:       "123.123.0.0",
				VPNClientIndex: -1,
				IPFamily:       "IPv6",
				OpenVPNPort:    1143,
				IsDualStack:    true,
				Protocol:       "tcp",
			}

			content, err := generateClientConfig(cfg)
			It("does not error creating the template", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("generated config contain check", func() {
				It("proto tcp6-client", func() {
					Expect(content).To(ContainSubstring(`proto tcp-client`))
				})
			})
		})

		Context("ipv4 non HA config with udp protocol", func() {
			cfg := ClientValues{
				Endpoint:       "123.123.0.0",
				VPNClientIndex: -1,
				IPFamily:       "IPv4",
				OpenVPNPort:    1143,
				Protocol:       "udp",
				FragmentMTU:    1400,
			}

			content, err := generateClientConfig(cfg)
			It("does not error creating the template", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("generated config contain check", func() {
				It("proto udp4", func() {
					Expect(content).To(ContainSubstring(`proto udp4`))
				})
				It("has fragment and buffer options", func() {
					Expect(content).To(ContainSubstring(`fragment 1400 mtu`))
					Expect(content).To(ContainSubstring(`sndbuf 16777216`))
					Expect(content).To(ContainSubstring(`rcvbuf 16777216`))
				})
				It("does not have tcp-nodelay", func() {
					Expect(content).NotTo(ContainSubstring(`tcp-nodelay`))
				})
			})
		})

		Context("ipv6 non HA config with udp protocol", func() {
			cfg := ClientValues{
				Endpoint:       "123.123.0.0",
				VPNClientIndex: -1,
				IPFamily:       "IPv6",
				OpenVPNPort:    1143,
				Protocol:       "udp",
				FragmentMTU:    1400,
			}

			content, err := generateClientConfig(cfg)
			It("does not error creating the template", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("generated config contain check", func() {
				It("proto udp6", func() {
					Expect(content).To(ContainSubstring(`proto udp6`))
				})
			})
		})

		Context("dual-stack non HA config with udp protocol", func() {
			cfg := ClientValues{
				Endpoint:       "123.123.0.0",
				VPNClientIndex: -1,
				IPFamily:       "IPv6",
				OpenVPNPort:    1143,
				IsDualStack:    true,
				Protocol:       "udp",
				FragmentMTU:    1400,
			}

			content, err := generateClientConfig(cfg)
			It("does not error creating the template", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("generated config contain check", func() {
				It("proto udp", func() {
					Expect(content).To(ContainSubstring(`proto udp`))
				})
			})
		})
	})
})
