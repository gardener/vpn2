// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"fmt"
	"math/big"
	"net"
)

const (
	// IPv4Family represents the IPv4 address family.
	IPv4Family = "IPv4"
	// IPv6Family represents the IPv6 address family.
	IPv6Family = "IPv6"
)

type CIDR net.IPNet

func (c CIDR) Equal(other CIDR) bool {
	return c.IP.Equal(other.IP) && c.Mask.String() == other.Mask.String()
}

func (c *CIDR) UnmarshalText(text []byte) error {
	// empty strings are allowed
	if string(text) == "" {
		return nil
	}
	_, net, err := net.ParseCIDR(string(text))
	if err != nil {
		return err
	}
	*c = CIDR(*net)
	return nil
}

func (c CIDR) String() string {
	if len(c.IP) == 0 {
		return ""
	}
	ones, _ := c.Mask.Size()
	return fmt.Sprintf("%s/%d", c.IP, ones)
}

func (c CIDR) ToIPNet() *net.IPNet {
	netw := net.IPNet(c)
	return &netw
}

func (c *CIDR) IsIPv4() bool {
	return c.IP.To4() != nil
}

// CountHosts returns the number of addressable host IPs in a CIDR
func (c *CIDR) CountHosts() *big.Int {
	ones, bits := c.Mask.Size()
	if ones < 0 || bits <= 0 || ones > bits {
		return big.NewInt(0)
	}

	// total = 2^(hostBits)
	hostBits := bits - ones
	total := new(big.Int).Lsh(big.NewInt(1), uint(hostBits))

	// IPv4: subtract network and broadcast for traditional subnets.
	// /31 and /32 are special cases where no subtraction is applied.
	if bits == 32 && ones <= 30 {
		if total.Cmp(big.NewInt(2)) >= 0 {
			return new(big.Int).Sub(total, big.NewInt(2))
		}
		return big.NewInt(0)
	}

	return total
}

// ParseIPNet parses a CIDR string and returns a network.CIDR (for user-provided values)
func ParseIPNet(cidr string) (CIDR, error) {
	_, prefix, err := net.ParseCIDR(cidr)
	if err != nil {
		return CIDR{}, err
	}
	return CIDR(*prefix), nil
}

// ParseIPNetIgnoreError parses a CIDR string and ignores any error (for testing and constants)
func ParseIPNetIgnoreError(cidr string) CIDR {
	parsed, _ := ParseIPNet(cidr)
	return parsed
}

// GetByIPFamily returns a list of CIDRs that belong to the given IP family.
func GetByIPFamily(cidrs []CIDR, ipFamily string) []CIDR {
	var result []CIDR
	for _, nw := range cidrs {
		switch ipFamily {
		case IPv4Family:
			if nw.IP.To4() != nil {
				result = append(result, nw)
			}
		case IPv6Family:
			if nw.IP.To4() == nil {
				result = append(result, nw)
			}
		}
	}
	return result
}

// Overlap checks if two IP networks overlap.
func Overlap(a, b CIDR) bool {
	return a.ToIPNet().Contains(b.IP) || b.ToIPNet().Contains(a.IP)
}

// OverLapAny checks if any of the given IP networks otherNws overlap with nw.
func OverLapAny(nw CIDR, otherNws ...CIDR) bool {
	for _, otherNw := range otherNws {
		if Overlap(nw, otherNw) {
			return true
		}
	}
	return false
}
