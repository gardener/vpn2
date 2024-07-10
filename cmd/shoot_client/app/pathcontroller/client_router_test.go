package pathcontroller

import (
	"errors"
	"net"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type noopNetRouter struct{}

func (noopNetRouter) updateRouting(_ net.IP) (_ error) {
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
	BeforeEach(func() {
		pinger = &fakePinger{
			badIPs: make(map[string]struct{}),
		}

		router = &clientRouter{
			netRouter: noopNetRouter{},
			log:       logr.Discard(),
			pinger:    pinger,
			goodIPs:   make(map[string]struct{}),
		}
	})

	Describe("#pingAllShootClients", func() {
		Context("1 healthy client and 1 unhealthy client", func() {
			badIP := net.ParseIP("192.168.0.1")
			healthyIP := net.ParseIP("192.168.0.2")
			BeforeEach(func() {
				pinger.badIPs[badIP.String()] = struct{}{}
			})
			It("should result in only the healthy client being in goodIPs map", func() {
				clients := []net.IP{badIP, healthyIP}
				router.pingAllShootClients(clients)
				Expect(router.goodIPs).To(HaveKey(healthyIP.String()))
				Expect(router.goodIPs).ToNot(HaveKey(badIP.String()))
			})
		})
	})

	Describe("#setCurrentShootClient", func() {
		Context("with 2 healthy clients", func() {
			healthyIP1 := net.ParseIP("192.168.0.1")
			healthyIP2 := net.ParseIP("192.168.0.2")
			BeforeEach(func() {
				router.goodIPs[healthyIP1.String()] = struct{}{}
				router.goodIPs[healthyIP2.String()] = struct{}{}
			})
			Context("and current not in goodIPs", func() {
				It("should have one of the healthy client ips as current", func() {
					err := router.determinePrimaryShootClient()
					Expect(err).To(BeNil())
					// current is selected by random
					Expect(router.primary).To(BeElementOf([]net.IP{healthyIP1, healthyIP2}))
				})
			})
			Context("and current in goodIPs", func() {
				BeforeEach(func() {
					router.primary = healthyIP1
				})
				It("should not change the current ip", func() {
					err := router.determinePrimaryShootClient()
					Expect(err).To(BeNil())
					Expect(router.primary).To(Equal(healthyIP1))
				})
			})
		})
	})
})
