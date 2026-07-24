// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network_test

import (
	"bytes"
	"math"
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
			want:       net.ParseIP("fd8f:6d53:b97a:1:bb00::b:0"),
		},
		{
			name:       "vpn-shoot-1",
			vpnNetwork: vpnNetwork,
			index:      1,
			want:       net.ParseIP("fd8f:6d53:b97a:1:bb00::b:1"),
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
				IP:   net.ParseIP("fd8f:6d53:b97a:1:bb00::b:0"),
				Mask: net.CIDRMask(96, 128),
			},
		},
		{
			name:       "vpn-shoot-1",
			vpnNetwork: vpnNetwork,
			index:      1,
			want: net.IPNet{
				IP:   net.ParseIP("fd8f:6d53:b97a:1:bb00::b:1"),
				Mask: net.CIDRMask(96, 128),
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
		net.ParseIP("fd8f:6d53:b97a:1:bb00::b:0"),
		net.ParseIP("fd8f:6d53:b97a:1:bb00::b:1"),
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
			acquiredIP: net.ParseIP("fd8f:6d53:b97a:1:aa00:0:6746:576a"),
			want: net.IPNet{
				IP:   net.ParseIP("fd8f:6d53:b97a:1:aa00:0:6746:576a"),
				Mask: net.CIDRMask(96, 128),
			},
		},
		{
			name:       "kube-apiserver-2",
			acquiredIP: net.ParseIP("fd8f:6d53:b97a:1:aa00:0:d659:60f9"),
			want: net.IPNet{
				IP:   net.ParseIP("fd8f:6d53:b97a:1:aa00:0:d659:60f9"),
				Mask: net.CIDRMask(96, 128),
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
	wantBase := net.ParseIP("fd8f:6d53:b97a:1:aa00::")
	if !base.Equal(wantBase) {
		t.Errorf("unequal base client ip: want: %+v, got: %+v", wantBase, base)
	}
	if startIndex != 1 {
		t.Errorf("unequal startIndex: want: %d, got: %d", 1, startIndex)
	}
	if endIndex != 0xffffffff {
		t.Errorf("unequal endIndex: want: %d, got: %d", 0xffffffff, endIndex)
	}
}

func Test_BondingSeedClientAddress(t *testing.T) {
	tt := []struct {
		name       string
		vpnNetwork *net.IPNet
		podName    string
		want       net.IP
	}{
		{
			name:       "kube-apiserver deployment pod 1",
			vpnNetwork: vpnNetwork,
			podName:    "kube-apiserver-964cff756-twn4c",
			want:       net.ParseIP("fd8f:6d53:b97a:1:aa00:0:6746:576a"),
		},
		{
			name:       "kube-apiserver deployment pod 2",
			vpnNetwork: vpnNetwork,
			podName:    "kube-apiserver-964cff756-v6scw",
			want:       net.ParseIP("fd8f:6d53:b97a:1:aa00:0:d659:60f9"),
		},
		{
			name:       "kube-apiserver deployment pod 3",
			vpnNetwork: vpnNetwork,
			podName:    "kube-apiserver-964cff756-vw8fx",
			want:       net.ParseIP("fd8f:6d53:b97a:1:aa00:0:cc8b:a25"),
		},
		{
			name:       "single byte difference still generates different hash",
			vpnNetwork: vpnNetwork,
			podName:    "kube-apiserver-964cff756-vw8fy",
			want:       net.ParseIP("fd8f:6d53:b97a:1:aa00:0:cb8b:892"),
		},
		{
			name:       "different deployment, same random still generates different hash",
			vpnNetwork: vpnNetwork,
			podName:    "kube-apiserver-84f48dc696-twn4c",
			want:       net.ParseIP("fd8f:6d53:b97a:1:aa00:0:9d9f:ae24"),
		},
	}
	for _, testcase := range tt {
		t.Run(testcase.name, func(t *testing.T) {
			clientAddr := BondingSeedClientAddress(testcase.vpnNetwork, testcase.podName)
			if !clientAddr.IP.Equal(testcase.want) {
				t.Errorf("unequal seed client ip: want: %+v, got: %+v", testcase.want, clientAddr)
			}
		})
	}

}

func Test_BondingSeedClientAddressClash(t *testing.T) {
	// Test all possible combinations of the last 5 characters of the pod name to check if any
	// different pod name maps to the same 32-bit seed client IP suffix.
	// With 5 characters and 27 possible values each we are looking at 27^5 combinations (~14M),
	// which is well within the 32-bit address space (~4B). A good hash function produces zero
	// clashes across this sample space.
	podPrefix := "kube-apiserver-964cff756-"
	podName := podPrefix + "twn4c"
	baseIP := BondingSeedClientAddress(vpnNetwork, podName).IP
	charset := []byte("bcdfghjklmnpqrstvwxz2456789") // see https://github.com/kubernetes/apimachinery/blob/master/pkg/util/rand/rand.go#L83

	clashCount := 0
	for _, c1 := range charset {
		for _, c2 := range charset {
			for _, c3 := range charset {
				for _, c4 := range charset {
					for _, c5 := range charset {
						suffix := string([]byte{c1, c2, c3, c4, c5})
						candidatePodName := podPrefix + suffix
						if candidatePodName == podName {
							continue
						}

						candidateIP := BondingSeedClientAddress(vpnNetwork, candidatePodName).IP
						if candidateIP.Equal(baseIP) {
							clashCount++
						}
					}
				}
			}
		}
	}

	allCombinations := math.Pow(float64(len(charset)), 5.0)
	clashRatio := float64(clashCount) / allCombinations

	if clashCount > 0 {
		t.Errorf("too many clashes: %d in %f combinations (%.8f%%), expected zero clashes", clashCount, allCombinations, clashRatio*100)
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
				IP:   net.ParseIP("fd8f:6d53:b97a:1:ff00:0:100:0"),
				Mask: net.CIDRMask(112, 128),
			},
		},
		{
			name:       "vpn-seed-server-1",
			vpnNetwork: vpnNetwork,
			vpnIndex:   1,
			want: CIDR{
				IP:   net.ParseIP("fd8f:6d53:b97a:1:ff00:0:101:0"),
				Mask: net.CIDRMask(112, 128),
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

func Test_BondingShootClientSubnet(t *testing.T) {
	subnet := BondingShootClientSubnet(vpnNetwork)
	want := &net.IPNet{IP: net.ParseIP("fd8f:6d53:b97a:1:bb00::b:0"), Mask: net.CIDRMask(96, 128)}

	if !subnet.IP.Equal(want.IP) {
		t.Errorf("unequal shoot subnet ip: want: %+v, got: %+v", want.IP, subnet.IP)
	}
	if !bytes.Equal(subnet.Mask, want.Mask) {
		t.Errorf("unequal shoot subnet mask: want: %s, got: %s", want.Mask, subnet.Mask)
	}
}

func Test_BondingSeedClientSubnet(t *testing.T) {
	subnet := BondingSeedClientSubnet(vpnNetwork)
	want := &net.IPNet{IP: net.ParseIP("fd8f:6d53:b97a:1:aa00::"), Mask: net.CIDRMask(96, 128)}

	if !subnet.IP.Equal(want.IP) {
		t.Errorf("unequal seed subnet ip: want: %+v, got: %+v", want.IP, subnet.IP)
	}
	if !bytes.Equal(subnet.Mask, want.Mask) {
		t.Errorf("unequal seed subnet mask: want: %s, got: %s", want.Mask, subnet.Mask)
	}
}
