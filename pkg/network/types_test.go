// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("types", func() {
	Describe("CIDR", func() {
		Describe("Equal", func() {
			It("returns true for equal CIDRs", func() {
				a := ParseIPNetIgnoreError("10.10.0.0/24")
				b := ParseIPNetIgnoreError("10.10.0.0/24")

				Expect(a.Equal(b)).To(BeTrue())
			})

			It("returns false for different CIDRs", func() {
				a := ParseIPNetIgnoreError("10.10.0.0/24")
				b := ParseIPNetIgnoreError("10.10.0.0/25")

				Expect(a.Equal(b)).To(BeFalse())
			})
		})

		Describe("UnmarshalText", func() {
			It("accepts empty input", func() {
				cidr := CIDR{}

				Expect(cidr.UnmarshalText([]byte(""))).To(Succeed())
				Expect(cidr.String()).To(Equal(""))
			})

			It("parses a valid CIDR", func() {
				cidr := CIDR{}

				Expect(cidr.UnmarshalText([]byte("10.20.0.0/16"))).To(Succeed())
				Expect(cidr.String()).To(Equal("10.20.0.0/16"))
			})

			It("returns an error for invalid CIDR input", func() {
				cidr := CIDR{}

				Expect(cidr.UnmarshalText([]byte("invalid"))).To(HaveOccurred())
			})
		})

		Describe("String", func() {
			It("returns empty string for zero-value CIDR", func() {
				Expect((CIDR{}).String()).To(Equal(""))
			})

			It("returns canonical cidr string", func() {
				cidr := ParseIPNetIgnoreError("192.168.1.0/24")

				Expect(cidr.String()).To(Equal("192.168.1.0/24"))
			})
		})

		Describe("ToIPNet", func() {
			It("returns an equivalent net.IPNet", func() {
				cidr := ParseIPNetIgnoreError("10.0.0.0/8")

				ipNet := cidr.ToIPNet()
				Expect(ipNet).NotTo(BeNil())
				Expect(ipNet.IP.Equal(cidr.IP)).To(BeTrue())
				Expect(ipNet.Mask.String()).To(Equal(cidr.Mask.String()))
			})
		})

		Describe("IsIPv4", func() {
			It("returns true for IPv4 CIDR", func() {
				cidr := ParseIPNetIgnoreError("10.0.0.0/24")
				Expect(cidr.IsIPv4()).To(BeTrue())
			})

			It("returns false for IPv6 CIDR", func() {
				cidr := ParseIPNetIgnoreError("2001:db8::/64")
				Expect(cidr.IsIPv4()).To(BeFalse())
			})
		})

		Describe("CountHosts", func() {
			It("returns host count for subnet", func() {
				cidr := ParseIPNetIgnoreError("10.0.0.0/24")
				Expect(cidr.CountHosts()).To(Equal(254))
			})

			It("returns host count for IPv6 subnet", func() {
				cidr := ParseIPNetIgnoreError("2001:db8::/126")
				Expect(cidr.CountHosts()).To(Equal(2))
			})

			It("returns zero for prefixes without host addresses", func() {
				cidr := ParseIPNetIgnoreError("10.0.0.1/32")
				Expect(cidr.CountHosts()).To(Equal(0))
			})

		})
	})

	Describe("ParseIPNet", func() {
		It("parses valid CIDR", func() {
			cidr, err := ParseIPNet("172.16.0.0/12")

			Expect(err).NotTo(HaveOccurred())
			Expect(cidr.String()).To(Equal("172.16.0.0/12"))
		})

		It("parses valid IPv6 CIDR", func() {
			cidr, err := ParseIPNet("2001:db8::/64")

			Expect(err).NotTo(HaveOccurred())
			Expect(cidr.String()).To(Equal("2001:db8::/64"))
		})

		It("returns error for invalid input", func() {
			_, err := ParseIPNet("invalid")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ParseIPNetIgnoreError", func() {
		It("returns parsed CIDR for valid input", func() {
			cidr := ParseIPNetIgnoreError("10.0.0.0/24")
			Expect(cidr.String()).To(Equal("10.0.0.0/24"))
		})

		It("returns zero-value CIDR for invalid input", func() {
			cidr := ParseIPNetIgnoreError("invalid")
			Expect(cidr.String()).To(Equal(""))
		})
	})

	Describe("GetByIPFamily", func() {
		var cidrs []CIDR

		BeforeEach(func() {
			cidrs = []CIDR{
				ParseIPNetIgnoreError("10.0.0.0/24"),
				ParseIPNetIgnoreError("192.168.1.0/24"),
				ParseIPNetIgnoreError("2001:db8::/64"),
			}
		})

		It("returns only IPv4 cidrs", func() {
			result := GetByIPFamily(cidrs, IPv4Family)

			Expect(result).To(HaveLen(2))
			for _, cidr := range result {
				Expect(cidr.IP.To4()).NotTo(BeNil())
			}
		})

		It("returns only IPv6 cidrs", func() {
			result := GetByIPFamily(cidrs, IPv6Family)

			Expect(result).To(HaveLen(1))
			Expect(result[0].IP.To4()).To(BeNil())
		})

		It("returns empty list for unknown family", func() {
			result := GetByIPFamily(cidrs, "unknown")
			Expect(result).To(BeEmpty())
		})
	})

	Describe("Overlap", func() {
		It("returns true for overlapping networks", func() {
			a := ParseIPNetIgnoreError("10.0.0.0/24")
			b := ParseIPNetIgnoreError("10.0.0.128/25")

			Expect(Overlap(a, b)).To(BeTrue())
		})

		It("returns false for non-overlapping networks", func() {
			a := ParseIPNetIgnoreError("10.0.0.0/24")
			b := ParseIPNetIgnoreError("10.0.1.0/24")

			Expect(Overlap(a, b)).To(BeFalse())
		})

		It("returns true for overlapping IPv6 networks", func() {
			a := ParseIPNetIgnoreError("2001:db8::/64")
			b := ParseIPNetIgnoreError("2001:db8::8000/65")

			Expect(Overlap(a, b)).To(BeTrue())
		})

		It("returns false for non-overlapping IPv6 networks", func() {
			a := ParseIPNetIgnoreError("2001:db8::/64")
			b := ParseIPNetIgnoreError("2001:db8:1::/64")

			Expect(Overlap(a, b)).To(BeFalse())
		})
	})

	Describe("OverLapAny", func() {
		It("returns true when any network overlaps", func() {
			nw := ParseIPNetIgnoreError("10.0.0.0/24")
			otherA := ParseIPNetIgnoreError("10.0.1.0/24")
			otherB := ParseIPNetIgnoreError("10.0.0.128/25")

			Expect(OverLapAny(nw, otherA, otherB)).To(BeTrue())
		})

		It("returns false when no networks overlap", func() {
			nw := ParseIPNetIgnoreError("10.0.0.0/24")
			otherA := ParseIPNetIgnoreError("10.0.1.0/24")
			otherB := ParseIPNetIgnoreError("10.0.2.0/24")

			Expect(OverLapAny(nw, otherA, otherB)).To(BeFalse())
		})

		It("returns true when any IPv6 network overlaps", func() {
			nw := ParseIPNetIgnoreError("2001:db8::/64")
			otherA := ParseIPNetIgnoreError("2001:db8:1::/64")
			otherB := ParseIPNetIgnoreError("2001:db8::8000/65")

			Expect(OverLapAny(nw, otherA, otherB)).To(BeTrue())
		})

		It("returns false when no IPv6 networks overlap", func() {
			nw := ParseIPNetIgnoreError("2001:db8::/64")
			otherA := ParseIPNetIgnoreError("2001:db8:1::/64")
			otherB := ParseIPNetIgnoreError("2001:db8:2::/64")

			Expect(OverLapAny(nw, otherA, otherB)).To(BeFalse())
		})

		It("returns false when no candidate networks are provided", func() {
			nw := CIDR{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)}
			Expect(OverLapAny(nw)).To(BeFalse())
		})
	})
})
