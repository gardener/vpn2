/*
 * SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package ippool

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

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
func NewIPAddressBroker(manager IPPoolManager, base net.IP, startIndex, endIndex int, ownName string, waitTime time.Duration) (IPAddressBroker, error) {
	return &ipAddressBroker{
		manager:    manager,
		base:       base,
		startIndex: startIndex,
		endIndex:   endIndex,
		ownName:    ownName,
		waitTime:   waitTime,
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
		index := rand.Intn(b.endIndex-b.startIndex+1) + b.startIndex
		base4 := b.base.To4()
		ip := net.IPv4(base4[0], base4[1], base4[2], byte(index)).String()
		if _, ok := lookupResult.ForeignUsed[ip]; ok {
			continue
		}
		if _, ok := lookupResult.ForeignReserved[ip]; ok {
			continue
		}
		return ip
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
		time.Sleep(b.waitTime * time.Duration(rand.Intn(10)/10))
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
