// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package network

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"

	"github.com/gardener/vpn2/pkg/constants"
)

var _ = Describe("MTU", Serial, func() {
	Describe("GetDefaultMTU", func() {
		It("returns the MTU of the default route interface", func() {
			mtu, err := GetDefaultMTU()
			Expect(err).NotTo(HaveOccurred())
			Expect(mtu).To(BeNumerically(">", 0))
		})

		It("returns an error if no default route was found", func() {
			defaultRoute, err := getDefaultRoute()
			Expect(err).NotTo(HaveOccurred())

			// Temporarily remove the default route
			err = netlink.RouteDel(defaultRoute)
			Expect(err).NotTo(HaveOccurred())

			_, err = GetDefaultMTU()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to find default route"))

			// Restore the default route
			err = netlink.RouteAdd(defaultRoute)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("DetectTunnelMTU", func() {
		It("subtracts the overhead from the default MTU", func() {
			defaultMTU, err := GetDefaultMTU()
			Expect(err).NotTo(HaveOccurred())

			overhead := 100
			expected := defaultMTU - overhead
			if expected < constants.MinimumMTU {
				expected = constants.MinimumMTU
			}

			mtu, err := DetectTunnelMTU(overhead)
			Expect(err).NotTo(HaveOccurred())
			Expect(mtu).To(Equal(expected))
		})

		It("never returns less than the minimum MTU", func() {
			defaultMTU, err := GetDefaultMTU()
			Expect(err).NotTo(HaveOccurred())

			overhead := defaultMTU - 100
			mtu, err := DetectTunnelMTU(overhead)
			Expect(err).NotTo(HaveOccurred())
			Expect(mtu).To(Equal(constants.MinimumMTU))
		})
	})

	Describe("DetectFragmentMTU", func() {
		It("subtracts v1 overhead for udpm v1 non-HA", func() {
			defaultMTU, err := GetDefaultMTU()
			Expect(err).NotTo(HaveOccurred())

			mtu, err := DetectFragmentMTU("v1", false)
			Expect(err).NotTo(HaveOccurred())
			Expect(mtu).To(Equal(defaultMTU - constants.UDPProxyOverheadV1))
		})

		It("subtracts v2 overhead for udpm v2 non-HA", func() {
			defaultMTU, err := GetDefaultMTU()
			Expect(err).NotTo(HaveOccurred())

			mtu, err := DetectFragmentMTU("v2", false)
			Expect(err).NotTo(HaveOccurred())
			Expect(mtu).To(Equal(defaultMTU - constants.UDPProxyOverheadV2))
		})

		It("adds HA overhead on top of v1 overhead for udpm v1 HA", func() {
			defaultMTU, err := GetDefaultMTU()
			Expect(err).NotTo(HaveOccurred())

			mtu, err := DetectFragmentMTU("v1", true)
			Expect(err).NotTo(HaveOccurred())
			Expect(mtu).To(Equal(defaultMTU - constants.UDPProxyOverheadV1 - constants.UDProxyHAVPNOverhead))
		})

		It("adds HA overhead on top of v2 overhead for udpm v2 HA", func() {
			defaultMTU, err := GetDefaultMTU()
			Expect(err).NotTo(HaveOccurred())

			mtu, err := DetectFragmentMTU("v2", true)
			Expect(err).NotTo(HaveOccurred())
			Expect(mtu).To(Equal(defaultMTU - constants.UDPProxyOverheadV2 - constants.UDProxyHAVPNOverhead))
		})

		It("returns an error for unknown udpm version", func() {
			_, err := DetectFragmentMTU("v99", false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid UDPM version"))

			_, err = DetectFragmentMTU("v99", true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid UDPM version"))
		})
	})
})
