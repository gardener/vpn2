// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pathcontroller

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/vishvananda/netlink"

	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/gardener/vpn2/pkg/shoot_client/tunnel"
)

type clientRouter struct {
	pinger             pinger
	netRouter          netRouter
	kubeAPIServerPodIP string

	log        logr.Logger
	checkedNet *net.IPNet
	primary    net.IP
	mu         sync.Mutex
	goodIPs    map[string]struct{}
	ticker     *time.Ticker
}

type netRouter interface {
	updateRouting(net.IP) error
}

type pinger interface {
	Ping(client net.IP) error
}

func (r *clientRouter) Run(ctx context.Context, clientIPs []net.IP) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.ticker.C:
			r.pingAllShootClients(clientIPs)
			err := r.determinePrimaryShootClient()
			if err != nil {
				// don't return error here because in creation there will be some time when nothing is
				// available. If we returned the error we would exit the path-controller
				r.log.Error(err, "")
			}
		}
	}
}

func (r *clientRouter) selectNewPrimaryShootClient() (net.IP, error) {
	// just use a random ip that is in goodIps map
	for ip := range r.goodIPs {
		return net.ParseIP(ip), nil
	}
	return nil, errors.New("no more good ips in pool")
}

func (r *clientRouter) determinePrimaryShootClient() error {
	_, ok := r.goodIPs[r.primary.String()]
	if !ok {
		newIP, err := r.selectNewPrimaryShootClient()
		if err != nil {
			return fmt.Errorf("error selecting a new shoot client: %w", err)
		}
		err = r.netRouter.updateRouting(newIP)
		if err != nil {
			return err
		}
		r.log.Info("switching primary shoot client", "old", r.primary, "new", newIP)
		r.primary = newIP
	}
	return nil
}

func (r *clientRouter) pingAllShootClients(clients []net.IP) {
	var wg sync.WaitGroup
	for _, client := range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := r.pinger.Ping(client)
			r.mu.Lock()
			defer r.mu.Unlock()
			if err != nil {
				r.log.Info("client not healthy, removing from pool", "ip", client)
				delete(r.goodIPs, client.String())
			} else {
				r.goodIPs[client.String()] = struct{}{}
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			// sending own IP to other side of tunnel so that the back route can be setup correctly
			err := tunnel.Send(client, r.kubeAPIServerPodIP)
			if err != nil {
				r.log.Info("error sending UDP packet with own IP to vpn-shoot", "ip", client, "error", err)
			}
		}()
	}
	wg.Wait()
}

type netlinkRouter struct {
	shootPodNetworks     []network.CIDR
	shootServiceNetworks []network.CIDR
	shootNodeNetworks    []network.CIDR
	seedPodNetwork       network.CIDR

	log logr.Logger
}

func (r *netlinkRouter) updateRouting(newIP net.IP) error {
	clientIndex := network.ClientIndexFromBondingShootClientIP(newIP)
	tunnelLink, err := netlink.LinkByName(network.BondIP6TunnelLinkName(clientIndex))
	if err != nil {
		return err
	}

	var (
		serviceNetworks []network.CIDR
		podNetworks     []network.CIDR
		nodeNetworks    []network.CIDR
	)

	_, _, _, err = network.ShootNetworksForNetmap(r.shootPodNetworks, r.shootServiceNetworks, r.shootNodeNetworks)
	if err != nil {
		return err
	}

	// Check if there is an overlap between the seed pod network and shoot networks.
	overlap := network.OverLapAny(r.seedPodNetwork, slices.Concat(r.shootPodNetworks, r.shootServiceNetworks, r.shootNodeNetworks)...)

	// IPv4 networks are mapped to 240/4, IPv6 networks are kept as is
	for _, serviceNetwork := range r.shootServiceNetworks {
		if serviceNetwork.IP.To4() != nil && overlap {
			serviceNetworks = append(serviceNetworks, network.ParseIPNetIgnoreError(constants.ShootServiceNetworkMapped))
		} else {
			serviceNetworks = append(serviceNetworks, serviceNetwork)
		}
	}
	for _, podNetwork := range r.shootPodNetworks {
		if podNetwork.IP.To4() != nil && overlap {
			podNetworks = append(podNetworks, network.ParseIPNetIgnoreError(constants.ShootPodNetworkMapped))
		} else {
			podNetworks = append(podNetworks, podNetwork)
		}
	}
	for _, nodeNetwork := range r.shootNodeNetworks {
		if nodeNetwork.IP.To4() != nil && overlap {
			nodeNetworks = append(nodeNetworks, network.ParseIPNetIgnoreError(constants.ShootNodeNetworkMapped))
		} else {
			nodeNetworks = append(nodeNetworks, nodeNetwork)
		}
	}

	nets := [][]network.CIDR{
		serviceNetworks,
		podNetworks,
		nodeNetworks,
	}

	for _, nw := range nets {
		for _, n := range nw {
			route := routeForNetwork(n.ToIPNet(), tunnelLink)
			r.log.Info("replacing route", "route", route, "net", n)
			err = netlink.RouteReplace(&route)
			if err != nil {
				return fmt.Errorf("error replacing route for %s: %w", n, err)
			}
		}
	}
	return nil
}

func routeForNetwork(net *net.IPNet, tunnelLink netlink.Link) netlink.Route {
	return netlink.Route{
		Dst:       net,
		LinkIndex: tunnelLink.Attrs().Index,
	}
}
