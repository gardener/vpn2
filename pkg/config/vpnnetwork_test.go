// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"net"
	"testing"

	"github.com/gardener/vpn2/pkg/network"
)

func Test_ValidateVPNNetworkCIDR(t *testing.T) {
	tt := []struct {
		name        string
		networkCIDR string
		wantError   bool
	}{
		{
			name:        "valid cidr",
			networkCIDR: "fd8f:6d53:b97a:1::/112",
		},

		{
			name:        "ipv4 cidr",
			networkCIDR: "192.168.0.0/24",
			wantError:   true,
		},

		{
			name:        "ipv6 valid subnet mask",
			networkCIDR: "fd8f:6d53:b97a:1::/112",
		},

		{
			name:        "ipv4 invalid subnet mask",
			networkCIDR: "fd8f:6d53:b97a:1::/121",
			wantError:   true,
		},
	}
	for _, testcase := range tt {
		t.Run(testcase.name, func(t *testing.T) {

			_, n, err := net.ParseCIDR(testcase.networkCIDR)
			if err != nil {
				t.Fatal("could not parse CIDR from testcase")
			}

			err = validateVPNNetworkCIDR(network.CIDR(*n))
			if testcase.wantError && err == nil {
				t.Fatal("want error, got nil")
			}
			if err != nil && !testcase.wantError {
				t.Fatalf("got unwanted err: %s", err)
			}
		})
	}
}
