// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpn_server_test

import (
	"slices"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/gardener/vpn2/pkg/vpn_server"
)

func TestVPNServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VPNServer Suite")
}

var _ = Describe("BuildValues", func() {
	var cfg config.VPNServer

	BeforeEach(func() {
		cfg = config.VPNServer{
			StatusPath:  "/srv/status/openvpn.status",
			LocalNodeIP: "10.10.10.10",
		}
	})

	Describe("VPN_NETWORK", func() {
		Context("when VPN_NETWORK is not set", func() {
			It("should return an error", func() {
				_, err := vpn_server.BuildValues(cfg)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("VPN_NETWORK must be set"))
			})
		})
		Context("when VPN_NETWORK is not a IPv6 CIDR", func() {
			BeforeEach(func() {
				cfg.VPNNetwork = network.ParseIPNetIgnoreError("192.168.0.0/24")
			})

			It("should return an error", func() {
				_, err := vpn_server.BuildValues(cfg)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("VPN_NETWORK must be a IPv6 CIDR: " + cfg.VPNNetwork.String()))
			})
		})
		Context("when VPN_NETWORK is not a /96 IPv6 CIDR", func() {
			BeforeEach(func() {
				cfg.VPNNetwork = network.ParseIPNetIgnoreError("2001:db8::/64")
			})

			It("should return an error", func() {
				_, err := vpn_server.BuildValues(cfg)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("invalid prefix length for VPN_NETWORK, must be /96, vpn network: " + cfg.VPNNetwork.String()))
			})
		})
		Context("when VPN_NETWORK is a /96 IPv6 CIDR", func() {
			BeforeEach(func() {
				cfg.VPNNetwork = network.ParseIPNetIgnoreError("2001:db8::/96")
			})

			It("should pass", func() {
				_, err := vpn_server.BuildValues(cfg)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("HA", func() {
		BeforeEach(func() {
			cfg.VPNNetwork = network.ParseIPNetIgnoreError("2001:db8::/96")
		})
		Context("when IS_HA is set", func() {
			BeforeEach(func() {
				cfg.IsHA = true
				cfg.HAVPNClients = 3
			})
			It("should fail due to missing pod name", func() {
				_, err := vpn_server.BuildValues(cfg)
				Expect(err).To(MatchError("IS_HA flag in config does not match HA info from pod name: IS_HA = true, POD_NAME = "))
			})
			Context("when pod name is set non-stateful", func() {
				BeforeEach(func() {
					cfg.PodName = "vpn-seed-server-5d99b56fcb-2h58x"
				})
				It("should fail", func() {
					_, err := vpn_server.BuildValues(cfg)
					Expect(err).To(MatchError("IS_HA flag in config does not match HA info from pod name: IS_HA = true, POD_NAME = vpn-seed-server-5d99b56fcb-2h58x"))
				})
			})
			Context("when pod name is set", func() {
				BeforeEach(func() {
					cfg.PodName = "vpn-seed-server-1"
				})
				It("should pass", func() {
					_, err := vpn_server.BuildValues(cfg)
					Expect(err).ToNot(HaveOccurred())
				})
				Context("when networks are set (v4)", func() {
					var v4NetworksMapped []network.CIDR
					BeforeEach(func() {
						cfg.ServiceNetworks = []network.CIDR{network.ParseIPNetIgnoreError("100.80.0.0/16")}
						cfg.PodNetworks = []network.CIDR{network.ParseIPNetIgnoreError("100.81.0.0/16")}
						cfg.NodeNetworks = []network.CIDR{network.ParseIPNetIgnoreError("100.82.0.0/16")}
						v4NetworksMapped = []network.CIDR{
							network.ParseIPNetIgnoreError(constants.ShootNodeNetworkMapped),
							network.ParseIPNetIgnoreError(constants.ShootServiceNetworkMapped),
							network.ParseIPNetIgnoreError(constants.ShootPodNetworkMapped),
						}

					})
					It("should configure HA VPN correctly (v4)", func() {
						v, err := vpn_server.BuildValues(cfg)
						Expect(err).ToNot(HaveOccurred())
						Expect(v.IsHA).To(BeTrue())
						Expect(v.VPNIndex).To(Equal(1))
						Expect(v.Device).To(Equal(constants.TapDevice))
						Expect(v.HAVPNClients).To(Equal(3))
						Expect(v.OpenVPNNetwork).To(Equal(network.HAVPNTunnelNetwork(cfg.VPNNetwork.IP, 1)))
						Expect(v.ShootNetworks).To(ConsistOf(v4NetworksMapped))
						Expect(v.ShootNetworksV4).To(Equal(v.ShootNetworks))
						Expect(v.ShootNetworksV6).To(BeEmpty())
					})
					It("should remove duplicates", func() {
						cfg.ServiceNetworks = append(cfg.ServiceNetworks, cfg.ServiceNetworks...)
						cfg.PodNetworks = append(cfg.PodNetworks, cfg.PodNetworks...)
						cfg.NodeNetworks = append(cfg.NodeNetworks, cfg.NodeNetworks...)
						v, err := vpn_server.BuildValues(cfg)
						Expect(err).ToNot(HaveOccurred())
						Expect(v.ShootNetworks).To(HaveLen(3))
					})
					Context("when networks are set (v4 + v6)", func() {
						var v6Networks []network.CIDR
						BeforeEach(func() {
							v6Services := network.ParseIPNetIgnoreError("2001:db8:1::/64")
							v6Pods := network.ParseIPNetIgnoreError("2001:db8:2::/64")
							v6Nodes := network.ParseIPNetIgnoreError("2001:db8:3::/64")
							cfg.ServiceNetworks = append(cfg.ServiceNetworks, v6Services)
							cfg.PodNetworks = append(cfg.PodNetworks, v6Pods)
							cfg.NodeNetworks = append(cfg.NodeNetworks, v6Nodes)
							v6Networks = append(v6Networks, v6Services, v6Pods, v6Nodes)
						})
						It("should configure HA VPN correctly (v4 + v6)", func() {
							v, err := vpn_server.BuildValues(cfg)
							Expect(err).ToNot(HaveOccurred())
							Expect(v.IsHA).To(BeTrue())
							Expect(v.VPNIndex).To(Equal(1))
							Expect(v.Device).To(Equal(constants.TapDevice))
							Expect(v.HAVPNClients).To(Equal(3))
							Expect(v.OpenVPNNetwork).To(Equal(network.HAVPNTunnelNetwork(cfg.VPNNetwork.IP, 1)))
							Expect(v.ShootNetworks).To(ConsistOf(slices.Concat(v4NetworksMapped, v6Networks)))
							Expect(v.ShootNetworksV4).To(Equal(v4NetworksMapped))
							Expect(v.ShootNetworksV6).To(Equal(v6Networks))
						})
						It("should remove duplicates", func() {
							cfg.ServiceNetworks = append(cfg.ServiceNetworks, cfg.ServiceNetworks...)
							cfg.PodNetworks = append(cfg.PodNetworks, cfg.PodNetworks...)
							cfg.NodeNetworks = append(cfg.NodeNetworks, cfg.NodeNetworks...)
							v, err := vpn_server.BuildValues(cfg)
							Expect(err).ToNot(HaveOccurred())
							Expect(v.ShootNetworks).To(HaveLen(6))
						})
					})
				})
				Context("when networks are set (v6)", func() {
					var v6Networks []network.CIDR
					BeforeEach(func() {
						cfg.ServiceNetworks = []network.CIDR{network.ParseIPNetIgnoreError("2001:db8:1::/64")}
						cfg.PodNetworks = []network.CIDR{network.ParseIPNetIgnoreError("2001:db8:2::/64")}
						cfg.NodeNetworks = []network.CIDR{network.ParseIPNetIgnoreError("2001:db8:3::/64")}
						v6Networks = slices.Concat(cfg.ServiceNetworks, cfg.PodNetworks, cfg.NodeNetworks)
					})
					It("should configure HA VPN correctly (v6)", func() {
						v, err := vpn_server.BuildValues(cfg)
						Expect(err).ToNot(HaveOccurred())
						Expect(v.IsHA).To(BeTrue())
						Expect(v.VPNIndex).To(Equal(1))
						Expect(v.Device).To(Equal(constants.TapDevice))
						Expect(v.HAVPNClients).To(Equal(3))
						Expect(v.OpenVPNNetwork).To(Equal(network.HAVPNTunnelNetwork(cfg.VPNNetwork.IP, 1)))
						Expect(v.ShootNetworks).To(ConsistOf(v6Networks))
						Expect(v.ShootNetworksV6).To(Equal(v.ShootNetworks))
						Expect(v.ShootNetworksV4).To(BeEmpty())
					})
					It("should remove duplicates", func() {
						cfg.ServiceNetworks = append(cfg.ServiceNetworks, cfg.ServiceNetworks...)
						cfg.PodNetworks = append(cfg.PodNetworks, cfg.PodNetworks...)
						cfg.NodeNetworks = append(cfg.NodeNetworks, cfg.NodeNetworks...)
						v, err := vpn_server.BuildValues(cfg)
						Expect(err).ToNot(HaveOccurred())
						Expect(v.ShootNetworks).To(HaveLen(3))
					})
				})
			})
		})
	})

	Describe("non-HA", func() {
		BeforeEach(func() {
			cfg.VPNNetwork = network.ParseIPNetIgnoreError("2001:db8::/96")
		})
		Context("when IS_HA is not set", func() {
			It("should be false", func() {
				v, err := vpn_server.BuildValues(cfg)
				Expect(err).ToNot(HaveOccurred())
				Expect(v.IsHA).To(BeFalse())
			})
		})
		Context("when IS_HA is set to false", func() {
			BeforeEach(func() {
				cfg.IsHA = false
			})
			It("should be false", func() {
				v, err := vpn_server.BuildValues(cfg)
				Expect(err).ToNot(HaveOccurred())
				Expect(v.IsHA).To(BeFalse())
			})
			Context("when pod name is set stateful", func() {
				BeforeEach(func() {
					cfg.PodName = "vpn-seed-server-0"
				})
				It("should fail", func() {
					_, err := vpn_server.BuildValues(cfg)
					Expect(err).To(MatchError("IS_HA flag in config does not match HA info from pod name: IS_HA = false, POD_NAME = vpn-seed-server-0"))
				})
			})
			Context("when pod name is set non-stateful", func() {
				BeforeEach(func() {
					cfg.PodName = "vpn-seed-server-5d99b56fcb-2h58x"
				})
				It("should pass", func() {
					_, err := vpn_server.BuildValues(cfg)
					Expect(err).ToNot(HaveOccurred())
				})
				Context("when networks are set (v4)", func() {
					var v4NetworksMapped []network.CIDR
					BeforeEach(func() {
						cfg.ServiceNetworks = []network.CIDR{network.ParseIPNetIgnoreError("100.80.0.0/16")}
						cfg.PodNetworks = []network.CIDR{network.ParseIPNetIgnoreError("100.81.0.0/16")}
						cfg.NodeNetworks = []network.CIDR{network.ParseIPNetIgnoreError("100.82.0.0/16")}
						v4NetworksMapped = []network.CIDR{
							network.ParseIPNetIgnoreError(constants.ShootNodeNetworkMapped),
							network.ParseIPNetIgnoreError(constants.ShootServiceNetworkMapped),
							network.ParseIPNetIgnoreError(constants.ShootPodNetworkMapped),
						}
					})
					It("should configure HA VPN correctly (v4)", func() {
						v, err := vpn_server.BuildValues(cfg)
						Expect(err).ToNot(HaveOccurred())
						Expect(v.IsHA).To(BeFalse())
						Expect(v.VPNIndex).To(Equal(0))
						Expect(v.Device).To(Equal(constants.TunnelDevice))
						Expect(v.HAVPNClients).To(Equal(-1))
						Expect(v.OpenVPNNetwork).To(Equal(cfg.VPNNetwork))
						Expect(v.ShootNetworks).To(ConsistOf(v4NetworksMapped))
						Expect(v.ShootNetworksV4).To(Equal(v.ShootNetworks))
						Expect(v.ShootNetworksV6).To(BeEmpty())
					})
					It("should remove duplicates", func() {
						cfg.ServiceNetworks = append(cfg.ServiceNetworks, cfg.ServiceNetworks...)
						cfg.PodNetworks = append(cfg.PodNetworks, cfg.PodNetworks...)
						cfg.NodeNetworks = append(cfg.NodeNetworks, cfg.NodeNetworks...)
						v, err := vpn_server.BuildValues(cfg)
						Expect(err).ToNot(HaveOccurred())
						Expect(v.ShootNetworks).To(HaveLen(3))
					})
					Context("when networks are set (v4 + v6)", func() {
						var v6Networks []network.CIDR
						BeforeEach(func() {
							v6Services := network.ParseIPNetIgnoreError("2001:db8:1::/64")
							v6Pods := network.ParseIPNetIgnoreError("2001:db8:2::/64")
							v6Nodes := network.ParseIPNetIgnoreError("2001:db8:3::/64")
							cfg.ServiceNetworks = append(cfg.ServiceNetworks, v6Services)
							cfg.PodNetworks = append(cfg.PodNetworks, v6Pods)
							cfg.NodeNetworks = append(cfg.NodeNetworks, v6Nodes)
							v6Networks = append(v6Networks, v6Services, v6Pods, v6Nodes)
						})
						It("should configure HA VPN correctly (v4 + v6)", func() {
							v, err := vpn_server.BuildValues(cfg)
							Expect(err).ToNot(HaveOccurred())
							Expect(v.IsHA).To(BeFalse())
							Expect(v.VPNIndex).To(Equal(0))
							Expect(v.Device).To(Equal(constants.TunnelDevice))
							Expect(v.HAVPNClients).To(Equal(-1))
							Expect(v.OpenVPNNetwork).To(Equal(cfg.VPNNetwork))
							Expect(v.ShootNetworks).To(ConsistOf(slices.Concat(v4NetworksMapped, v6Networks)))
							Expect(v.ShootNetworksV4).To(Equal(v4NetworksMapped))
							Expect(v.ShootNetworksV6).To(Equal(v6Networks))
						})
						It("should remove duplicates", func() {
							cfg.ServiceNetworks = append(cfg.ServiceNetworks, cfg.ServiceNetworks...)
							cfg.PodNetworks = append(cfg.PodNetworks, cfg.PodNetworks...)
							cfg.NodeNetworks = append(cfg.NodeNetworks, cfg.NodeNetworks...)
							v, err := vpn_server.BuildValues(cfg)
							Expect(err).ToNot(HaveOccurred())
							Expect(v.ShootNetworks).To(HaveLen(6))
						})
					})
				})
				Context("when networks are set (v6)", func() {
					var v6Networks []network.CIDR
					BeforeEach(func() {
						cfg.ServiceNetworks = []network.CIDR{network.ParseIPNetIgnoreError("2001:db8:1::/64")}
						cfg.PodNetworks = []network.CIDR{network.ParseIPNetIgnoreError("2001:db8:2::/64")}
						cfg.NodeNetworks = []network.CIDR{network.ParseIPNetIgnoreError("2001:db8:3::/64")}
						v6Networks = slices.Concat(cfg.ServiceNetworks, cfg.PodNetworks, cfg.NodeNetworks)
					})
					It("should configure HA VPN correctly (v6)", func() {
						v, err := vpn_server.BuildValues(cfg)
						Expect(err).ToNot(HaveOccurred())
						Expect(v.IsHA).To(BeFalse())
						Expect(v.VPNIndex).To(Equal(0))
						Expect(v.Device).To(Equal(constants.TunnelDevice))
						Expect(v.HAVPNClients).To(Equal(-1))
						Expect(v.OpenVPNNetwork).To(Equal(cfg.VPNNetwork))
						Expect(v.ShootNetworks).To(ConsistOf(v6Networks))
						Expect(v.ShootNetworksV6).To(Equal(v.ShootNetworks))
						Expect(v.ShootNetworksV4).To(BeEmpty())
					})
					It("should remove duplicates", func() {
						cfg.ServiceNetworks = append(cfg.ServiceNetworks, cfg.ServiceNetworks...)
						cfg.PodNetworks = append(cfg.PodNetworks, cfg.PodNetworks...)
						cfg.NodeNetworks = append(cfg.NodeNetworks, cfg.NodeNetworks...)
						v, err := vpn_server.BuildValues(cfg)
						Expect(err).ToNot(HaveOccurred())
						Expect(v.ShootNetworks).To(HaveLen(3))
					})
				})
			})
		})
	})
})
