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
	data         map[string]ipdata
	setWaitGroup sync.WaitGroup
}

var _ IPPoolManager = &mockManager{}

func newMockIPPoolManager() *mockManager {
	return &mockManager{
		data: map[string]ipdata{},
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
	m.setWaitGroup.Add(1)
	go func() {
		time.Sleep(baseWait / 3)
		m.Lock()
		defer m.Unlock()

		m.data[podName] = ipdata{ip: ip, used: used}
		m.setWaitGroup.Done()
	}()

	return nil
}

func (m *mockManager) waitSetIPAddressComplete() {
	m.setWaitGroup.Wait()
}

func (m *mockManager) getData() map[string]ipdata {
	m.Lock()
	defer m.Unlock()
	return m.data
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

func TestBrokerFullPoolUsageIPv6Large(t *testing.T) {
	testBroker(t, 100, 0xff00, true)
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
			IP:   net.ParseIP("fd8f:6d53:b97a:1::a:0"),
			Mask: net.CIDRMask(104, 128),
		})
	}
	for i := 0; i < count; i++ {
		cfg := config.VPNClient{
			VPNNetwork: vpnNetwork,
			PodName:    podName(i),
			WaitTime:   baseWait,
		}
		brokers[i], err = NewIPAddressBroker(manager, &cfg)
		if err != nil {
			t.Errorf("new failed: %s", err)
			return
		}
		if err = brokers[i].SetStartAndEndIndex(10, 10+space-1); err != nil {
			t.Errorf("set range failed: %s", err)
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
				err = fmt.Errorf("pod-%d failed: %s", i, err2)

			}
			waitGroup.Done()
		}(brokers[i])
	}
	waitGroup.Wait()

	manager.waitSetIPAddressComplete()

	if space < count {
		if err == nil {
			t.Errorf("expected to fail as no free IP available")
		} else {
			found := false
			for _, txt := range []string{"cannot find any free IP address", "no free IP address found"} {
				if strings.Contains(err.Error(), txt) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("unexpected error: %s", err)
			}
		}
		return
	}

	if err != nil {
		t.Errorf("acquire failed: %s", err)
	}

	data := manager.getData()
	if len(data) != count {
		t.Errorf("pod count mismatch: %d != %d", len(data), count)
	}
	ips := map[string]string{}
	for name, value := range data {
		if value.ip == "" || !value.used {
			t.Errorf("no used IP for pod %s", name)
			continue
		}
		if other := ips[value.ip]; other != "" {
			t.Errorf("duplicate IP for pod %s and %s", name, other)
		}
		ips[value.ip] = name
	}
}
