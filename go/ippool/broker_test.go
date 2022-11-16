package ippool

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
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

func TestBroker(t *testing.T) {
	logName = true
	manager := newMockIPPoolManager()
	base := net.IPv4(192, 168, 120, 0)
	count := 10
	brokers := make([]IPAddressBroker, count)
	var err error
	for i := 0; i < count; i++ {
		brokers[i], err = NewIPAddressBroker(manager, base, 10, 10+count, podName(i), baseWait)
		if err != nil {
			t.Errorf("new failed: %s", err)
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
