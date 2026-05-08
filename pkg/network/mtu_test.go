// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
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
})
