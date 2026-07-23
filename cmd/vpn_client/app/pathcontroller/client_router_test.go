// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pathcontroller

import (
	"errors"
	"net/netip"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeNetRouter implements netRouter. state is the source of truth returned by getNexthopHealth;
// setHealthCalls records which IPs had their health state changed.
type fakeNetRouter struct {
	routingConfigured bool
	state             map[netip.Addr]bool
	setHealthCalls    map[netip.Addr]bool
}

func newFakeNetRouter() *fakeNetRouter {
	return &fakeNetRouter{
		state:          make(map[netip.Addr]bool),
		setHealthCalls: make(map[netip.Addr]bool),
	}
}

func (f *fakeNetRouter) setupRouting(_ []netip.Addr) error {
	f.routingConfigured = true
	return nil
}

func (f *fakeNetRouter) setNexthopHealth(clientIP netip.Addr, healthy bool, _ []netip.Addr) error {
	f.state[clientIP] = healthy
	f.setHealthCalls[clientIP] = healthy
	return nil
}

func (f *fakeNetRouter) getNexthopHealth(clientIPs []netip.Addr) (map[netip.Addr]bool, error) {
	states := make(map[netip.Addr]bool, len(clientIPs))
	for _, ip := range clientIPs {
		states[ip] = f.state[ip]
	}
	return states, nil
}

// fakePinger implements pinger interface and returns an error if clientIP used for ping is part of badIPs
type fakePinger struct {
	badIPs map[netip.Addr]struct{}
}

func (f *fakePinger) Ping(client netip.Addr) (_ error) {
	if _, ok := f.badIPs[client]; ok {
		return errors.New("unhealthy")
	}
	return nil
}

var _ = Describe("#ClientRouter", func() {
	var router *clientRouter
	var pinger *fakePinger
	var netRouter *fakeNetRouter
	BeforeEach(func() {
		pinger = &fakePinger{
			badIPs: make(map[netip.Addr]struct{}),
		}
		netRouter = newFakeNetRouter()

		router = &clientRouter{
			netRouter: netRouter,
			log:       logr.Discard(),
			pinger:    pinger,
		}
	})

	Describe("#reconcileNexthopGroup", func() {
		ip1 := netip.MustParseAddr("192.168.0.1")
		ip2 := netip.MustParseAddr("192.168.0.2")
		clients := []netip.Addr{ip1, ip2}

		BeforeEach(func() {
			netRouter.state[ip1] = true
			netRouter.state[ip2] = true
		})

		Context("when one client becomes unhealthy", func() {
			It("should update the unhealthy client's health to false", func() {
				router.reconcileNexthopGroup(ip1, false, clients)
				Expect(netRouter.setHealthCalls).To(HaveKeyWithValue(ip1, false))
				Expect(netRouter.state[ip1]).To(BeFalse())
				Expect(netRouter.state[ip2]).To(BeTrue())
			})
		})

		Context("when both clients are unhealthy", func() {
			It("should never set both next hops unhealthy to avoid a complete outage", func() {
				// The independent per-client loops reconcile one after another; the second one must
				// keep the last remaining healthy next hop up.
				router.reconcileNexthopGroup(ip1, false, clients)
				router.reconcileNexthopGroup(ip2, false, clients)
				upCount := 0
				for _, up := range netRouter.state {
					if up {
						upCount++
					}
				}
				Expect(upCount).To(Equal(1))
			})
		})

		Context("when a previously failing client becomes healthy again", func() {
			BeforeEach(func() {
				netRouter.state[ip1] = false
			})
			It("should update the client's health back to true", func() {
				router.reconcileNexthopGroup(ip1, true, clients)
				Expect(netRouter.setHealthCalls).To(HaveKeyWithValue(ip1, true))
				Expect(netRouter.state[ip1]).To(BeTrue())
			})
		})

		Context("when no health change occurs", func() {
			It("should not call setNexthopHealth", func() {
				router.reconcileNexthopGroup(ip1, true, clients)
				Expect(netRouter.setHealthCalls).To(BeEmpty())
			})
		})
	})
})
