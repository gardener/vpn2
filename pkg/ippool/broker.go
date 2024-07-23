// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package ippool

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"time"

	"github.com/gardener/vpn2/pkg/config"
)

type ipAddressBroker struct {
	manager    IPPoolManager
	base       net.IP
	startIndex int
	endIndex   int
	ownName    string
	waitTime   time.Duration
	ownIP      string
	ownUsed    bool
}

// IPAddressBroker is the broker to retrieve a IP from the IP pool
type IPAddressBroker = *ipAddressBroker

var logName bool

// NewIPAddressBroker creates a new instance
func NewIPAddressBroker(manager IPPoolManager, cfg *config.VPNClient) (IPAddressBroker, error) {
	return &ipAddressBroker{
		manager:    manager,
		base:       cfg.VPNNetwork.IP,
		startIndex: cfg.StartIndex,
		endIndex:   cfg.EndIndex,
		ownName:    cfg.PodName,
		waitTime:   cfg.WaitTime,
	}, nil
}

func (b *ipAddressBroker) getExistingIPAddresses(ctx context.Context) (*IPPoolUsageLookupResult, error) {
	return b.manager.UsageLookup(ctx, b.ownName)
}

func (b *ipAddressBroker) log(fmtstr string, args ...interface{}) {
	if logName {
		fmtstr = b.ownName + ": " + fmtstr
	}
	println(fmt.Sprintf(fmtstr, args...))
}

func (b *ipAddressBroker) announceIPAddress(ctx context.Context, used bool, lookupResult *IPPoolUsageLookupResult) error {
	if lookupResult.OwnUsed {
		return nil
	}
	var ip string
	if used {
		ip = lookupResult.OwnIP
		if ip == "" {
			return fmt.Errorf("internal error: own ip undefined")
		}
	} else {
		ip = b.findFreeIPAddress(lookupResult)
		if ip == "" {
			return fmt.Errorf("no free IP address found")
		}
	}
	if err := b.manager.SetIPAddress(ctx, b.ownName, ip, used); err != nil {
		return err
	}

	b.ownIP = ip
	b.ownUsed = used
	return nil
}

func (b *ipAddressBroker) findFreeIPAddress(lookupResult *IPPoolUsageLookupResult) string {
	for i := 0; i < 1000; i++ {
		index := rand.N(b.endIndex-b.startIndex+1) + b.startIndex
		ip := make(net.IP, len(b.base))
		copy(ip, b.base)
		ip[len(ip)-1] = byte(index)
		s := ip.String()
		if _, ok := lookupResult.ForeignUsed[s]; ok {
			continue
		}
		if _, ok := lookupResult.ForeignReserved[s]; ok {
			continue
		}
		return s
	}
	return ""
}

func (b *ipAddressBroker) hasConflict(lookupResult *IPPoolUsageLookupResult) bool {
	_, found1 := lookupResult.ForeignUsed[b.ownIP]
	_, found2 := lookupResult.ForeignReserved[b.ownIP]
	return found1 || found2
}

func (b *ipAddressBroker) AcquireIP(ctx context.Context) (string, error) {
	var err error
	var result *IPPoolUsageLookupResult
	for i := 0; i < 30; i++ {
		result, err = b.getExistingIPAddresses(ctx)
		if err != nil {
			return "", fmt.Errorf("existing IP address lookup failed: %w", err)
		}
		if result.OwnUsed {
			return result.OwnIP, nil
		}
		err = b.announceIPAddress(ctx, false, result)
		if err != nil {
			return "", fmt.Errorf("reserving IP address failed: %w", err)
		}
		b.log("reserving ip %s", b.ownIP)
		time.Sleep(b.waitTime)
		result, err = b.getExistingIPAddresses(ctx)
		if err != nil {
			return "", fmt.Errorf("existing IP address lookup failed: %w", err)
		}
		if !b.hasConflict(result) {
			break
		}
		b.log("conflict, retrying...")
		time.Sleep(b.waitTime * time.Duration(rand.N(10)/10))
	}

	if b.hasConflict(result) {
		return "", fmt.Errorf("cannot find any free IP address")
	}

	err = b.announceIPAddress(ctx, true, result)
	if err != nil {
		return "", fmt.Errorf("using IP address failed: %w", err)
	}
	b.log("using ip %s", b.ownIP)
	return b.ownIP, nil
}
