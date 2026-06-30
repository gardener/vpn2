// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pathcontroller

import (
	"errors"
	"net"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeNetRouter implements netRouter and records the link states it was asked to set.
type fakeNetRouter struct {
	routingConfigured bool
	linkStates        map[string]bool
}

func newFakeNetRouter() *fakeNetRouter {
	return &fakeNetRouter{linkStates: make(map[string]bool)}
}

func (f *fakeNetRouter) updateRouting(_ []net.IP) error {
	f.routingConfigured = true
	return nil
}

func (f *fakeNetRouter) setLinkState(clientIP net.IP, up bool) error {
	f.linkStates[clientIP.String()] = up
	return nil
}

// fakePinger implements pinger interface and returns and error if clientIP used for ping is part of badIPs
type fakePinger struct {
	badIPs map[string]struct{}
}

func (f *fakePinger) Ping(client net.IP) (_ error) {
	if _, ok := f.badIPs[client.String()]; ok {
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
			badIPs: make(map[string]struct{}),
		}
		netRouter = newFakeNetRouter()

		router = &clientRouter{
			netRouter: netRouter,
			log:       logr.Discard(),
			pinger:    pinger,
			linkUp:    make(map[string]bool),
		}
	})

	Describe("#pingAllShootClients", func() {
		Context("1 healthy client and 1 unhealthy client", func() {
			badIP := net.ParseIP("192.168.0.1")
			healthyIP := net.ParseIP("192.168.0.2")
			BeforeEach(func() {
				pinger.badIPs[badIP.String()] = struct{}{}
			})
			It("should mark only the healthy client as healthy", func() {
				clients := []net.IP{badIP, healthyIP}
				healthy := router.pingAllShootClients(clients)
				Expect(healthy).To(HaveKeyWithValue(healthyIP.String(), true))
				Expect(healthy).To(HaveKeyWithValue(badIP.String(), false))
			})
		})
	})

	Describe("#reconcileLinks", func() {
		ip1 := net.ParseIP("192.168.0.1")
		ip2 := net.ParseIP("192.168.0.2")
		clients := []net.IP{ip1, ip2}

		BeforeEach(func() {
			router.linkUp[ip1.String()] = true
			router.linkUp[ip2.String()] = true
		})

		Context("when one client is unhealthy and the other healthy", func() {
			It("should set the unhealthy link down and keep the healthy link up", func() {
				healthy := map[string]bool{ip1.String(): false, ip2.String(): true}
				router.reconcileLinks(clients, healthy)
				Expect(netRouter.linkStates).To(HaveKeyWithValue(ip1.String(), false))
				Expect(netRouter.linkStates).ToNot(HaveKey(ip2.String()))
				Expect(router.linkUp[ip1.String()]).To(BeFalse())
				Expect(router.linkUp[ip2.String()]).To(BeTrue())
			})
		})

		Context("when both clients are unhealthy", func() {
			It("should never set both links down to avoid a complete outage", func() {
				healthy := map[string]bool{ip1.String(): false, ip2.String(): false}
				router.reconcileLinks(clients, healthy)
				// Exactly one link must remain up.
				upCount := 0
				for _, up := range router.linkUp {
					if up {
						upCount++
					}
				}
				Expect(upCount).To(Equal(1))
			})
		})

		Context("when a previously failing link becomes healthy again", func() {
			BeforeEach(func() {
				router.linkUp[ip1.String()] = false
			})
			It("should bring the link back up", func() {
				healthy := map[string]bool{ip1.String(): true, ip2.String(): true}
				router.reconcileLinks(clients, healthy)
				Expect(netRouter.linkStates).To(HaveKeyWithValue(ip1.String(), true))
				Expect(router.linkUp[ip1.String()]).To(BeTrue())
			})
		})

		Context("when only one link is up and that client turns unhealthy", func() {
			BeforeEach(func() {
				router.linkUp[ip2.String()] = false
			})
			It("should keep the last link up and not touch any link", func() {
				healthy := map[string]bool{ip1.String(): false, ip2.String(): false}
				router.reconcileLinks(clients, healthy)
				Expect(netRouter.linkStates).To(BeEmpty())
				Expect(router.linkUp[ip1.String()]).To(BeTrue())
			})
		})
	})
})
