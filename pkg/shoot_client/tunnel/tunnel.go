// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/go-logr/logr"
	"github.com/vishvananda/netlink"
	"k8s.io/utils/ptr"
)

const (
	tunnelControllerPort   = 5400
	cleanUpPeriod          = 15 * time.Minute
	creationFailureBackoff = 30 * time.Second
	expirationDuration     = 10 * time.Minute
	retriesToListen        = 30
	retryListenWait        = 500 * time.Millisecond
)

// NewController creates a new tunnel controller server.
func NewController() *Controller {
	return &Controller{
		kubeApiservers: map[string]*kubeApiserverData{},
		nextClean:      time.Now().Add(cleanUpPeriod),
	}
}

type kubeApiserverData struct {
	lock                sync.Mutex
	log                 logr.Logger
	podIP               string
	localAddr           net.IP
	remoteAddr          net.IP
	lastSeen            time.Time
	creationComplete    bool
	lastCreationFailed  *time.Time
	creationFailedCount int
	lastError           error
}

func (d *kubeApiserverData) setLastSeen() {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.lastSeen = time.Now()
}

func (d *kubeApiserverData) needsUpdate(podIP string) bool {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.podIP != podIP {
		return true
	}
	if d.creationComplete {
		return false
	} else if d.lastCreationFailed != nil {
		// if creation of tunnel device or update of route failed, retry again only after a backoff
		return time.Since(*d.lastCreationFailed) > creationFailureBackoff
	}
	return true
}

func (d *kubeApiserverData) update() {
	d.lock.Lock()
	defer d.lock.Unlock()

	name := fmt.Sprintf("%s-ip6tnl-%02x", constants.BondDevice, d.remoteAddr[len(d.remoteAddr)-1])

	if err := network.DeleteLinkByName(name); err != nil {
		d._setFailed(fmt.Errorf("failed to delete link %s: %w", name, err))
		return
	}

	if err := network.CreateTunnel(name, d.localAddr, d.remoteAddr); err != nil {
		d._setFailed(fmt.Errorf("failed to create tunnel device %s: %w", name, err))
		return
	}
	d.log.Info("tunnel device created", "name", name)

	link, err := netlink.LinkByName(name)
	if err != nil {
		d._setFailed(fmt.Errorf("failed to get link %s: %w", name, err))
		return
	}

	ip := net.ParseIP(d.podIP)
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}
	if ip == nil {
		d._setFailed(fmt.Errorf("failed to parse pod IP %s: %w", d.podIP, err))
		return
	}

	if err := network.ReplaceRoute(d.log, &net.IPNet{IP: ip, Mask: net.CIDRMask(len(ip)*8, len(ip)*8)}, link); err != nil {
		d._setFailed(fmt.Errorf("failed to replace route %s: %w", name, err))
		return
	}

	d.creationComplete = true
	d.lastError = nil
	d.lastCreationFailed = nil
	d.creationFailedCount = 0
}

func (d *kubeApiserverData) delete() {
	d.lock.Lock()
	defer d.lock.Unlock()

	name := fmt.Sprintf("%s-ip6tnl-%02x", constants.BondDevice, d.remoteAddr[len(d.remoteAddr)-1])
	if err := network.DeleteLinkByName(name); err != nil {
		d.log.Error(err, "failed to delete old tunnel device", "name", name)
	} else {
		d.log.Info("tunnel device deleted", "name", name)
	}
}

func (d *kubeApiserverData) _setFailed(err error) {
	d.lastCreationFailed = ptr.To(time.Now())
	d.creationFailedCount++
	d.lastError = err
	d.log.Error(err, "failed to update tunnel controller")
}

func (d *kubeApiserverData) isOutdated() bool {
	d.lock.Lock()
	defer d.lock.Unlock()
	return time.Since(d.lastSeen) > expirationDuration
}

// Controller is a server receiving UDP requests to create ipv6tnl devices.
type Controller struct {
	lock           sync.Mutex
	kubeApiservers map[string]*kubeApiserverData
	nextClean      time.Time
}

// Run runs the tunnel controller
func (c *Controller) Run(log logr.Logger) error {
	ips, err := network.GetLinkIPAddressesByName(constants.BondDevice, network.ScopeUniverse)
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		return fmt.Errorf("no IP addresses found for %s", constants.BondDevice)
	}
	if len(ips[0]) != 16 {
		return fmt.Errorf("expected ipv6 address for %s, got %s", constants.BondDevice, ips[0])
	}

	localBond := ips[0]
	localAddress := net.UDPAddr{
		IP:   localBond,
		Port: tunnelControllerPort,
	}
	var conn *net.UDPConn
	for i := 0; i < retriesToListen; i++ {
		conn, err = net.ListenUDP("udp6", &localAddress)
		if err == nil {
			break
		}
		time.Sleep(retryListenWait)
	}
	if err != nil {
		return fmt.Errorf("error listening on UDP: %w", err)
	}
	defer conn.Close()

	log.Info("server listening for UDP6 packages on IP of bond device", "address", localAddress.String())

	buffer := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Error(err, "reading from UDP failed")
			continue
		}
		podIP := string(buffer[:n])

		key := clientAddr.IP.String()

		c.lock.Lock()
		data := c.kubeApiservers[key]
		if data == nil {
			data = &kubeApiserverData{
				log:        log,
				localAddr:  localBond,
				remoteAddr: clientAddr.IP,
				podIP:      podIP,
			}
			c.kubeApiservers[key] = data
		}
		c.lock.Unlock()

		data.setLastSeen()
		if data.needsUpdate(podIP) {
			go data.update()
		}
		if c.nextClean.After(time.Now()) {
			c.nextClean = time.Now().Add(cleanUpPeriod)
			go c.clean()
		}
	}
}

func (c *Controller) clean() {
	c.lock.Lock()
	defer c.lock.Unlock()
	for key, data := range c.kubeApiservers {
		if data.isOutdated() {
			delete(c.kubeApiservers, key)
			data.delete()
		}
	}
}
