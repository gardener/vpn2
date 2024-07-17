// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package ippool

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/network"
)

const baseWait = 5 * time.Millisecond

type ipdata struct {
	ip   string
	used bool
}
type mockManager struct {
	sync.Mutex
	data map[string]*ipdata
}

var _ IPPoolManager = &mockManager{}

func newMockIPPoolManager() *mockManager {
	return &mockManager{
		data: map[string]*ipdata{},
	}
}

func (m *mockManager) UsageLookup(ctx context.Context, podName string) (*IPPoolUsageLookupResult, error) {
	m.Lock()
	defer m.Unlock()
	result := &IPPoolUsageLookupResult{
		OwnName:         podName,
		ForeignUsed:     map[string]struct{}{},
		ForeignReserved: map[string]struct{}{},
	}
	for key, v := range m.data {
		ip := v.ip
		used := v.used
		if ip != "" {
			if key == podName {
				result.OwnIP = ip
				result.OwnUsed = used
			} else if used {
				result.ForeignUsed[ip] = struct{}{}
			} else {
				result.ForeignReserved[ip] = struct{}{}
			}
		}
	}
	return result, nil
}

func (m *mockManager) SetIPAddress(ctx context.Context, podName, ip string, used bool) error {
	go func() {
		time.Sleep(baseWait / 3)
		m.Lock()
		defer m.Unlock()

		v := m.data[podName]
		if v == nil {
			v = &ipdata{}
			m.data[podName] = v
		}

		v.ip = ip
		v.used = used
	}()

	return nil
}

func podName(i int) string {
	return fmt.Sprintf("pod-%d", i)
}

func TestBrokerFullPoolUsage(t *testing.T) {
	testBroker(t, 10, 10, false)
}

func TestBrokerOverbookedPool(t *testing.T) {
	testBroker(t, 11, 10, false)
}

func TestBrokerFullPoolUsageIPv6(t *testing.T) {
	testBroker(t, 10, 10, true)
}

func testBroker(t *testing.T, count, space int, ipv6 bool) {
	logName = true
	manager := newMockIPPoolManager()
	brokers := make([]IPAddressBroker, count)
	var err error

	vpnNetwork := network.CIDR(net.IPNet{
		IP:   net.IPv4(192, 168, 120, 0),
		Mask: net.CIDRMask(24, 32),
	})
	if ipv6 {
		vpnNetwork = network.CIDR(net.IPNet{
			IP:   net.ParseIP("fd8f:6d53:b97a:1::"),
			Mask: net.CIDRMask(120, 128),
		})
	}
	for i := 0; i < count; i++ {
		cfg := config.VPNClient{
			VPNNetwork: vpnNetwork,
			StartIndex: 10,
			EndIndex:   10 + space,
			PodName:    podName(i),
			WaitTime:   baseWait,
		}
		brokers[i], err = NewIPAddressBroker(manager, &cfg)
		if err != nil {
			t.Errorf("new failed: %s", err)
			return
		}
	}

	var waitGroup sync.WaitGroup
	for i := 0; i < count; i++ {
		waitGroup.Add(1)
		go func(broker IPAddressBroker) {
			ctx := context.TODO()
			_, err2 := broker.AcquireIP(ctx)
			if err2 != nil {
				err = err2
			}
			waitGroup.Done()
		}(brokers[i])
	}
	waitGroup.Wait()
	time.Sleep(baseWait / 2) // wait for delayed update of mockManager

	if space < count {
		if err == nil {
			t.Errorf("expected to fail as no free IP available")
		} else {
			if !strings.Contains(err.Error(), "cannot find any free IP address") {
				t.Errorf("unexpected error: %s (should contain 'cannot find any free IP address')", err)
			}
		}
		return
	}

	if err != nil {
		t.Errorf("acquire failed: %s", err)
	}

	if len(manager.data) != count {
		t.Errorf("pod count mismatch: %d != %d", len(manager.data), count)
	}
	ips := map[string]string{}
	for name, value := range manager.data {
		if value.ip == "" || !value.used {
			t.Errorf("no used IP for pod %s", name)
		}
		if other := ips[value.ip]; other != "" {
			t.Errorf("duplicate IP for pod %s and %s", name, other)
		}
		ips[value.ip] = name
	}
}
