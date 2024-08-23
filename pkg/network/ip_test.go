// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network_test

import (
	"bytes"
	"net"
	"testing"

	"github.com/gardener/vpn2/pkg/constants"
	. "github.com/gardener/vpn2/pkg/network"
)

var vpnNetwork = &constants.DefaultVPNNetwork

func Test_BondingShootClientIP(t *testing.T) {
	tt := []struct {
		name       string
		vpnNetwork *net.IPNet
		index      int
		want       net.IP
	}{
		{
			name:       "vpn-shoot-0",
			vpnNetwork: vpnNetwork,
			index:      0,
			want:       net.ParseIP("fd8f:6d53:b97a:1::b00"),
		},
		{
			name:       "vpn-shoot-1",
			vpnNetwork: vpnNetwork,
			index:      1,
			want:       net.ParseIP("fd8f:6d53:b97a:1::b01"),
		},
	}
	for _, testcase := range tt {
		t.Run(testcase.name, func(t *testing.T) {
			clientIP := BondingShootClientIP(testcase.vpnNetwork, testcase.index)
			if !clientIP.Equal(testcase.want) {
				t.Errorf("unequal shoot client ip: want: %+v, got: %+v", testcase.want, clientIP)
			}
		})
	}
}

func Test_BondingShootClientAddress(t *testing.T) {
	tt := []struct {
		name       string
		vpnNetwork *net.IPNet
		index      int
		want       net.IPNet
	}{
		{
			name:       "vpn-shoot-0",
			vpnNetwork: vpnNetwork,
			index:      0,
			want: net.IPNet{
				IP:   net.ParseIP("fd8f:6d53:b97a:1::b00"),
				Mask: net.CIDRMask(119, 128),
			},
		},
		{
			name:       "vpn-shoot-1",
			vpnNetwork: vpnNetwork,
			index:      1,
			want: net.IPNet{
				IP:   net.ParseIP("fd8f:6d53:b97a:1::b01"),
				Mask: net.CIDRMask(119, 128),
			},
		},
	}
	for _, testcase := range tt {
		t.Run(testcase.name, func(t *testing.T) {
			subnet := BondingShootClientAddress(testcase.vpnNetwork, testcase.index)

			if !bytes.Equal(subnet.Mask, testcase.want.Mask) {
				t.Errorf("unequal subnet masks: want: %s, got: %s", testcase.want.Mask, subnet.Mask)
			}

			if !subnet.IP.Equal(testcase.want.IP) {
				t.Errorf("unequal subnet ip: want: %+v, got: %+v", testcase.want.IP, subnet.IP)
			}
		})
	}
}

func Test_AllBondingShootClientIPs(t *testing.T) {
	ips := AllBondingShootClientIPs(vpnNetwork, 2)
	want := []net.IP{
		net.ParseIP("fd8f:6d53:b97a:1::b00"),
		net.ParseIP("fd8f:6d53:b97a:1::b01"),
	}
	if len(ips) != len(want) {
		t.Errorf("unequal number of ips: want: %d, got: %d", len(want), len(ips))
	}
	for i := range ips {
		if !ips[i].Equal(want[i]) {
			t.Errorf("unequal shoot client ip: want: %+v, got: %+v", want[i], ips[i])
		}
	}
}

func Test_BondingAddressForSeedClient(t *testing.T) {
	tt := []struct {
		name       string
		acquiredIP net.IP
		want       net.IPNet
	}{
		{
			name:       "kube-apiserver-1",
			acquiredIP: net.ParseIP("fd8f:6d53:b97a:1::a47"),
			want: net.IPNet{
				IP:   net.ParseIP("fd8f:6d53:b97a:1::a47"),
				Mask: net.CIDRMask(119, 128),
			},
		},
		{
			name:       "kube-apiserver-2",
			acquiredIP: net.ParseIP("fd8f:6d53:b97a:1::aef"),
			want: net.IPNet{
				IP:   net.ParseIP("fd8f:6d53:b97a:1::aef"),
				Mask: net.CIDRMask(119, 128),
			},
		},
	}
	for _, testcase := range tt {
		t.Run(testcase.name, func(t *testing.T) {
			subnet := BondingAddressForClient(testcase.acquiredIP)
			if !bytes.Equal(subnet.Mask, testcase.want.Mask) {
				t.Errorf("unequal subnet masks: want: %s, got: %s", testcase.want.Mask, subnet.Mask)
			}

			if !subnet.IP.Equal(testcase.want.IP) {
				t.Errorf("unequal subnet ip: want: %+v, got: %+v", testcase.want.IP, subnet.IP)
			}
		})
	}
}

func Test_BondingSeedClientRange(t *testing.T) {
	base, startIndex, endIndex := BondingSeedClientRange(vpnNetwork.IP)
	wantBase := net.ParseIP("fd8f:6d53:b97a:1::a00")
	if !base.Equal(wantBase) {
		t.Errorf("unequal base client ip: want: %+v, got: %+v", wantBase, base)
	}
	if startIndex != 1 {
		t.Errorf("unequal startIndex: want: %d, got: %d", 1, startIndex)
	}
	if endIndex != 0xff {
		t.Errorf("unequal endIndex: want: %d, got: %d", 0xff, endIndex)
	}
}

func Test_HAVPNTunnelNetwork(t *testing.T) {
	tt := []struct {
		name       string
		vpnNetwork *net.IPNet
		vpnIndex   int
		want       CIDR
	}{
		{
			name:       "vpn-seed-server-0",
			vpnNetwork: vpnNetwork,
			vpnIndex:   0,
			want: CIDR{
				IP:   net.ParseIP("fd8f:6d53:b97a:1::0"),
				Mask: net.CIDRMask(120, 128),
			},
		},
		{
			name:       "vpn-seed-server-1",
			vpnNetwork: vpnNetwork,
			vpnIndex:   1,
			want: CIDR{
				IP:   net.ParseIP("fd8f:6d53:b97a:1::100"),
				Mask: net.CIDRMask(120, 128),
			},
		},
	}
	for _, testcase := range tt {
		t.Run(testcase.name, func(t *testing.T) {
			subnet := HAVPNTunnelNetwork(testcase.vpnNetwork.IP, testcase.vpnIndex)
			if !subnet.IP.Equal(testcase.want.IP) {
				t.Errorf("unequal CIDR ip: want: %+v, got: %+v", testcase.want.IP, subnet.IP)
			}
			if !bytes.Equal(subnet.Mask, testcase.want.Mask) {
				t.Errorf("unequal subnet masks: want: %s, got: %s", testcase.want.Mask, subnet.Mask)
			}
		})
	}
}
