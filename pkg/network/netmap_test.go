// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NetmapIP", func() {
	type testCase struct {
		description string
		inputIP     string
		subnet      string
		expected    string
		expectErr   bool
		errorMsg    string
	}

	DescribeTable("NetmapIP function",
		func(tc testCase) {
			result, err := NetmapIP(tc.inputIP, tc.subnet)
			if tc.expectErr {
				Expect(err).To(HaveOccurred())
				if tc.errorMsg != "" {
					Expect(err.Error()).To(ContainSubstring(tc.errorMsg))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(tc.expected))
			}
		},
		Entry("should map IPv4 address correctly", testCase{
			description: "valid IPv4 mapping",
			inputIP:     "192.168.1.100",
			subnet:      "10.0.0.0/8",
			expected:    "10.168.1.100",
		}),
		Entry("should preserve host portion when mapping", testCase{
			description: "preserve host portion",
			inputIP:     "172.16.5.10",
			subnet:      "192.168.0.0/16",
			expected:    "192.168.5.10",
		}),
		Entry("should handle /24 subnet correctly", testCase{
			description: "handle /24 subnet",
			inputIP:     "10.0.1.5",
			subnet:      "172.16.5.0/24",
			expected:    "172.16.5.5",
		}),
		Entry("should fail with invalid IP", testCase{
			description: "invalid IP address",
			inputIP:     "invalid-ip",
			subnet:      "10.0.0.0/8",
			expectErr:   true,
			errorMsg:    "failed to parse ip",
		}),
		Entry("should fail with invalid subnet", testCase{
			description: "invalid subnet",
			inputIP:     "192.168.1.1",
			subnet:      "invalid-subnet",
			expectErr:   true,
			errorMsg:    "failed to parse subnet",
		}),
		Entry("should fail with IPv6 address", testCase{
			description: "IPv6 not supported",
			inputIP:     "2001:db8::1",
			subnet:      "10.0.0.0/8",
			expectErr:   true,
			errorMsg:    "only IPv4 is supported",
		}),
		Entry("should fail with IPv6 subnet", testCase{
			description: "IPv6 subnet not supported",
			inputIP:     "192.168.1.1",
			subnet:      "2001:db8::/32",
			expectErr:   true,
			errorMsg:    "only IPv4 is supported",
		}),
	)

})

var _ = Describe("netmapIP", func() {
	It("should map IPv4 correctly", func() {
		ip := net.ParseIP("192.168.1.100")
		_, subnet, _ := net.ParseCIDR("10.0.0.0/8")
		mapped, err := netmapIP(ip, *subnet)
		Expect(err).NotTo(HaveOccurred())
		Expect(mapped.String()).To(Equal("10.168.1.100"))
	})
	It("should fail for IPv6", func() {
		ip := net.ParseIP("2001:db8::1")
		_, subnet, _ := net.ParseCIDR("10.0.0.0/8")
		_, err := netmapIP(ip, *subnet)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("subnetSplit", func() {
	It("should split /8 into 256 /16s", func() {
		_, parent, _ := net.ParseCIDR("242.0.0.0/8")
		subnets, err := subnetSplit(parent, 16)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(subnets)).To(Equal(256))
		Expect(subnets[0].String()).To(Equal("242.0.0.0/16"))
		Expect(subnets[255].String()).To(Equal("242.255.0.0/16"))
	})
	It("should split /24 into 256 /32s", func() {
		_, parent, _ := net.ParseCIDR("192.168.1.0/24")
		subnets, err := subnetSplit(parent, 32)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(subnets)).To(Equal(256))
		Expect(subnets[0].String()).To(Equal("192.168.1.0/32"))
		Expect(subnets[255].String()).To(Equal("192.168.1.255/32"))
	})
	It("should fail for invalid prefix", func() {
		_, parent, _ := net.ParseCIDR("10.0.0.0/8")
		_, err := subnetSplit(parent, 4)
		Expect(err).To(HaveOccurred())
	})
	It("should fail for IPv6", func() {
		_, parent, _ := net.ParseCIDR("2001:db8::/32")
		_, err := subnetSplit(parent, 40)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("netmapSubnet", func() {
	It("should map subnet correctly", func() {
		_, src, _ := net.ParseCIDR("10.1.2.0/24")
		_, dst, _ := net.ParseCIDR("242.1.2.0/24")
		mapped, err := netmapSubnet(*src, *dst)
		Expect(err).NotTo(HaveOccurred())
		Expect(mapped.IP.String()).To(Equal("242.1.2.0"))
		Expect(mapped.Mask.String()).To(Equal(dst.Mask.String()))
	})
	It("should fail for IPv6", func() {
		_, src, _ := net.ParseCIDR("2001:db8::/32")
		_, dst, _ := net.ParseCIDR("242.1.2.0/24")
		_, err := netmapSubnet(*src, *dst)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("NetmapSubnets", func() {
	It("should map multiple subnets non-overlapping", func() {
		srcs := []string{"10.1.0.0/16", "10.2.0.0/16", "10.3.1.0/24", "10.4.2.240/28"}
		dst := "242.0.0.0/8"
		mapped, err := NetmapSubnets(srcs, dst)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(mapped)).To(Equal(4))
		// Verify the correct mapping
		expectedMapping := map[string]string{
			"10.1.0.0/16":   "242.0.0.0/16",
			"10.2.0.0/16":   "242.1.0.0/16",
			"10.3.1.0/24":   "242.2.0.0/24",
			"10.4.2.240/28": "242.2.1.0/28",
		}
		Expect(mapped).To(Equal(expectedMapping))
		// Verify none of the mapped networks overlap
		for _, dst := range mapped {
			for _, otherDst := range mapped {
				if dst != otherDst {
					Expect(Overlap(ParseIPNetIgnoreError(dst), ParseIPNetIgnoreError(otherDst))).To(BeFalse(), "Mapped networks should not overlap")
				}
			}
		}

	})
	It("should fail if not enough space", func() {
		srcs := []string{"10.1.0.0/9", "10.2.0.0/9", "10.3.0.0/9"}
		dst := "242.0.0.0/8"
		_, err := NetmapSubnets(srcs, dst)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("NetmapCIDRs", func() {
	It("should map CIDRs", func() {
		srcs := []CIDR{
			ParseIPNetIgnoreError("10.1.0.0/16"),
			ParseIPNetIgnoreError("10.2.0.0/24"),
		}
		dst := ParseIPNetIgnoreError("242.0.0.0/8")
		mapped, err := NetmapCIDRs(srcs, dst)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(mapped)).To(Equal(2))

		// Convert to strings for easier equality check
		mappedStr := make(map[string]string, len(mapped))
		for src, dst := range mapped {
			mappedStr[src.String()] = dst.String()
		}

		// Verify the correct mapping
		expectedMapping := map[string]string{
			"10.1.0.0/16": "242.0.0.0/16",
			"10.2.0.0/24": "242.1.0.0/24",
		}

		Expect(mappedStr).To(Equal(expectedMapping))
	})
	It("should fail if not enough space", func() {
		srcs := []CIDR{
			ParseIPNetIgnoreError("10.1.0.0/8"),
			ParseIPNetIgnoreError("10.2.0.0/8"),
		}
		dst := ParseIPNetIgnoreError("242.0.0.0/8")
		_, err := NetmapCIDRs(srcs, dst)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not enough space in 242.0.0.0/8 to fit all source subnets"))
	})
})

var _ = Describe("ShootNetworksForNetmap", func() {
	It("should map pod, service, and node networks", func() {
		pods := []CIDR{ParseIPNetIgnoreError("10.1.0.0/16")}
		services := []CIDR{ParseIPNetIgnoreError("10.2.0.0/16")}
		nodes := []CIDR{ParseIPNetIgnoreError("10.3.0.0/24")}
		podMap, svcMap, nodeMap, err := ShootNetworksForNetmap(pods, services, nodes)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(podMap)).To(Equal(1))
		Expect(len(svcMap)).To(Equal(1))
		Expect(len(nodeMap)).To(Equal(1))
	})
	It("should fail if mapping fails", func() {
		pods := []CIDR{ParseIPNetIgnoreError("10.1.0.0/8"), ParseIPNetIgnoreError("10.2.0.0/8")}
		services := []CIDR{}
		nodes := []CIDR{}
		_, _, _, err := ShootNetworksForNetmap(pods, services, nodes)
		Expect(err).To(HaveOccurred())
	})
	It("should map multiple and dual-stack pod, service, and node networks", func() {
		pods := []CIDR{
			ParseIPNetIgnoreError("10.1.0.0/16"),
			ParseIPNetIgnoreError("10.2.0.0/24"),
			ParseIPNetIgnoreError("fd00:1::/64"),
		}
		services := []CIDR{
			ParseIPNetIgnoreError("10.3.0.0/16"),
			ParseIPNetIgnoreError("fd00:2::/112"),
		}
		nodes := []CIDR{
			ParseIPNetIgnoreError("10.219.45.0/26"),
			ParseIPNetIgnoreError("10.219.45.64/26"),
			ParseIPNetIgnoreError("10.96.0.0/11"),
			ParseIPNetIgnoreError("10.128.0.0/12"),
			ParseIPNetIgnoreError("fd00:3::/120"),
		}
		podMap, svcMap, nodeMap, err := ShootNetworksForNetmap(pods, services, nodes)
		Expect(err).NotTo(HaveOccurred())
		// Only IPv4 mappings are expected
		Expect(len(podMap)).To(Equal(2))
		Expect(len(svcMap)).To(Equal(1))
		Expect(len(nodeMap)).To(Equal(4))
		// Verify the correct mapping
		Expect(podMap).To(HaveKeyWithValue(
			new(ParseIPNetIgnoreError("10.1.0.0/16")),
			ParseIPNetIgnoreError("244.0.0.0/16"),
		))
		Expect(podMap).To(HaveKeyWithValue(
			new(ParseIPNetIgnoreError("10.2.0.0/24")),
			ParseIPNetIgnoreError("244.1.0.0/24"),
		))
		Expect(svcMap).To(HaveKeyWithValue(
			new(ParseIPNetIgnoreError("10.3.0.0/16")),
			ParseIPNetIgnoreError("243.0.0.0/16"),
		))
		Expect(nodeMap).To(HaveKeyWithValue(
			new(ParseIPNetIgnoreError("10.96.0.0/11")),
			ParseIPNetIgnoreError("242.0.0.0/11"),
		))
		Expect(nodeMap).To(HaveKeyWithValue(
			new(ParseIPNetIgnoreError("10.128.0.0/12")),
			ParseIPNetIgnoreError("242.32.0.0/12"),
		))
		Expect(nodeMap).To(HaveKeyWithValue(
			new(ParseIPNetIgnoreError("10.219.45.0/26")),
			ParseIPNetIgnoreError("242.48.0.0/26"),
		))
		Expect(nodeMap).To(HaveKeyWithValue(
			new(ParseIPNetIgnoreError("10.219.45.64/26")),
			ParseIPNetIgnoreError("242.48.0.64/26"),
		))
		// Verify none of the mapped networks overlap
		for _, m := range []map[*CIDR]CIDR{podMap, svcMap, nodeMap} {
			for _, dst := range m {
				for _, otherDst := range m {
					if !dst.Equal(otherDst) {
						Expect(Overlap(dst, otherDst)).To(BeFalse(), "Mapped networks should not overlap")
					}
				}
			}
		}
	})
	It("should handle only IPv6 networks gracefully (no IPv4 mappings)", func() {
		pods := []CIDR{ParseIPNetIgnoreError("fd00:1::/64")}
		services := []CIDR{ParseIPNetIgnoreError("fd00:2::/112")}
		nodes := []CIDR{ParseIPNetIgnoreError("fd00:3::/120")}
		podMap, svcMap, nodeMap, err := ShootNetworksForNetmap(pods, services, nodes)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(podMap)).To(Equal(0))
		Expect(len(svcMap)).To(Equal(0))
		Expect(len(nodeMap)).To(Equal(0))
	})
	It("should map dual-stack networks and ignore IPv6", func() {
		pods := []CIDR{
			ParseIPNetIgnoreError("10.10.0.0/16"),
			ParseIPNetIgnoreError("fd00:10::/64"),
		}
		services := []CIDR{
			ParseIPNetIgnoreError("10.20.0.0/16"),
			ParseIPNetIgnoreError("fd00:20::/112"),
		}
		nodes := []CIDR{
			ParseIPNetIgnoreError("10.30.0.0/24"),
			ParseIPNetIgnoreError("fd00:30::/120"),
		}
		podMap, svcMap, nodeMap, err := ShootNetworksForNetmap(pods, services, nodes)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(podMap)).To(Equal(1))
		Expect(len(svcMap)).To(Equal(1))
		Expect(len(nodeMap)).To(Equal(1))
		// Verify the correct mapping
		Expect(podMap).To(HaveKeyWithValue(
			new(ParseIPNetIgnoreError("10.10.0.0/16")),
			ParseIPNetIgnoreError("244.0.0.0/16"),
		))
		Expect(svcMap).To(HaveKeyWithValue(
			new(ParseIPNetIgnoreError("10.20.0.0/16")),
			ParseIPNetIgnoreError("243.0.0.0/16"),
		))
		Expect(nodeMap).To(HaveKeyWithValue(
			new(ParseIPNetIgnoreError("10.30.0.0/24")),
			ParseIPNetIgnoreError("242.0.0.0/24"),
		))
	})
	It("should not fail even on very small shoot networks", func() {
		pods := []CIDR{ParseIPNetIgnoreError("10.1.0.1/30")}
		services := []CIDR{ParseIPNetIgnoreError("10.2.0.2/31")}
		nodes := []CIDR{ParseIPNetIgnoreError("10.3.0.15/32"), ParseIPNetIgnoreError("100.1.12.3/32")}
		podMap, svcMap, nodeMap, err := ShootNetworksForNetmap(pods, services, nodes)
		Expect(err).To(Not(HaveOccurred()))
		// Verify the correct mapping
		Expect(podMap).To(HaveKeyWithValue(
			new(ParseIPNetIgnoreError("10.1.0.0/30")),
			ParseIPNetIgnoreError("244.0.0.0/30"),
		))
		Expect(svcMap).To(HaveKeyWithValue(
			new(ParseIPNetIgnoreError("10.2.0.2/31")),
			ParseIPNetIgnoreError("243.0.0.0/31"),
		))
		Expect(nodeMap).To(HaveKeyWithValue(
			new(ParseIPNetIgnoreError("10.3.0.15/32")),
			ParseIPNetIgnoreError("242.0.0.0/32"),
		))
		Expect(nodeMap).To(HaveKeyWithValue(
			new(ParseIPNetIgnoreError("100.1.12.3/32")),
			ParseIPNetIgnoreError("242.0.0.1/32"),
		))
	})
})
