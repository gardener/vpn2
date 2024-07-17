// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gardener/vpn2/pkg/network"
	"github.com/go-logr/logr"
	"github.com/vishvananda/netlink"
	"k8s.io/utils/ptr"
)

const (
	UDPPort       = 5400
	cleanUpPeriod = 15 * time.Minute
)

// IP6Tunnel contains addresses of kube-apiserver to build tunnel and route.
type IP6Tunnel struct {
	// KubeAPIServerPodIP is the IP address of the kube-apiserver pod.
	KubeAPIServerPodIP string `json:"kubeAPIServerPodIP"`
}

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
	localBond           net.IP
	remoteBond          net.IP
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

func (d *kubeApiserverData) needsUpdate(tunnelConfig IP6Tunnel) bool {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.podIP != tunnelConfig.KubeAPIServerPodIP {
		return true
	}
	if d.creationComplete {
		return false
	} else if d.lastCreationFailed != nil {
		return time.Since(*d.lastCreationFailed) > 30*time.Second
	}
	return true
}

func (d *kubeApiserverData) update() {
	d.lock.Lock()
	defer d.lock.Unlock()

	name := fmt.Sprintf("bond0-ip6tnl-%02x", d.remoteBond[len(d.remoteBond)-1])

	if err := network.DeleteLinkByName(name); err != nil {
		d._setFailed(fmt.Errorf("failed to delete link %s: %w", name, err))
		return
	}

	if err := network.CreateTunnelIP6Tnl(name, d.localBond, d.remoteBond); err != nil {
		d._setFailed(fmt.Errorf("failed to create tunnel %s: %w", name, err))
		return
	}
	d.log.Info("tunnel created", "name", name)

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

	if err := network.RouteReplace(d.log, &net.IPNet{IP: ip, Mask: net.CIDRMask(len(ip)*8, len(ip)*8)}, link); err != nil {
		d._setFailed(fmt.Errorf("failed to replace route %s: %w", name, err))
		return
	}

	d.creationComplete = true
	d.lastError = nil
	d.lastCreationFailed = nil
	d.creationFailedCount = 0
}

func (d *kubeApiserverData) delete() error {
	d.lock.Lock()
	defer d.lock.Unlock()

	name := fmt.Sprintf("bond0-ip6tnl-%02x", d.remoteBond[len(d.remoteBond)-1])
	return network.DeleteLinkByName(name)
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
	return time.Since(d.lastSeen) > 10*time.Minute
}

// Controller is a server receiving UDP requests to create ipv6tnl devices.
type Controller struct {
	kubeApiservers map[string]*kubeApiserverData
	nextClean      time.Time
}

// Run runs the tunnel controller
func (c *Controller) Run(log logr.Logger) error {
	ips, err := network.GetLinkIPAddressesByName("bond0", network.ScopeUniverse)
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		return fmt.Errorf("no IP addresses found for bond0")
	}
	if len(ips[0]) != 16 {
		return fmt.Errorf("expected ipv6 address for bond0, got %s", ips[0])
	}

	localBond := ips[0]
	localAddress := fmt.Sprintf("[%s]:%d", localBond.String(), UDPPort)
	addr, err := net.ResolveUDPAddr("udp6", localAddress)
	if err != nil {
		return fmt.Errorf("error resolving UDP address: %w", err)
	}

	var conn *net.UDPConn
	for i := 0; i < 30; i++ {
		conn, err = net.ListenUDP("udp6", addr)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("error listening on UDP: %w", err)
	}
	defer conn.Close()

	log.Info("server listening for UDP6 packages on bond0 IP", "address", localAddress)

	buffer := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Error(err, "reading from UDP failed")
			continue
		}
		tunnelConfig := IP6Tunnel{}
		if err := json.Unmarshal(buffer[:n], &tunnelConfig); err != nil {
			log.Error(err, "parsing tunnel configuration failed")
		}

		key := clientAddr.IP.String()
		data := c.kubeApiservers[key]
		if data == nil {
			data = &kubeApiserverData{
				log:        log,
				localBond:  localBond,
				remoteBond: clientAddr.IP,
				podIP:      tunnelConfig.KubeAPIServerPodIP,
			}
			c.kubeApiservers[key] = data
		}
		data.setLastSeen()
		if data.needsUpdate(tunnelConfig) {
			go data.update()
		}
		if c.nextClean.After(time.Now()) {
			c.nextClean = time.Now().Add(cleanUpPeriod)
			go c.clean()
		}
	}
}

func (c *Controller) clean() {
	for key, data := range c.kubeApiservers {
		if data.isOutdated() {
			delete(c.kubeApiservers, key)
			if err := data.delete(); err != nil {
				data.log.Error(err, "failed to delete old tunnel configuration")
			}
		}
	}
}
