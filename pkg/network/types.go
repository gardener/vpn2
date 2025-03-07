// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"fmt"
	"net"
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
