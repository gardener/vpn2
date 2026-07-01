// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pathcontroller

import (
	"context"
	"fmt"
	"net"
	"slices"
	"sync"
	"sync/atomic"
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
	updateInterval     time.Duration

	log logr.Logger
	// mu serializes the reconcileLink function across all clients
	mu sync.Mutex
}

type netRouter interface {
	// setupRouting sets up the ECMP multipath routes for all shoot networks using every
	// shoot client's ip6tnl device as a nexthop.
	setupRouting(clientIPs []net.IP) error
	// setLinkState brings the ip6tnl device corresponding to the given shoot client IP up or down.
	setLinkState(clientIP net.IP, up bool) error
	// getLinkStates returns the state (up/down) of the ip6tnl device of every given shoot client, keyed by client IP.
	getLinkStates(clientIPs []net.IP) (map[string]bool, error)
}

type pinger interface {
	Ping(client net.IP) error
}

func (r *clientRouter) Run(ctx context.Context, clientIPs []net.IP) error {
	// Set up the ECMP multipath route table once. Afterward the route table is never touched
	// again; only the administrative state of the individual ip6tnl links is changed up/down in
	// the per-client loops.
	if err := r.netRouter.setupRouting(clientIPs); err != nil {
		return err
	}

	// Run an independent loop per shoot client so that a slow or hanging ping to one client
	// never blocks the ping and keepalive of the other client.
	var wg sync.WaitGroup
	for _, ip := range clientIPs {
		wg.Add(1)
		go func(clientIP net.IP) {
			defer wg.Done()
			r.runClient(ctx, clientIP, clientIPs)
		}(ip)
	}
	wg.Wait()
	return ctx.Err()
}

// runClient drives the lifecycle of a single shoot client: on every tick it sends the
// UDP keepalive and it pings the client to reconcile the ip6tnl link state.
func (r *clientRouter) runClient(ctx context.Context, clientIP net.IP, allClients []net.IP) {
	ticker := time.NewTicker(r.updateInterval)
	defer ticker.Stop()

	var pinging atomic.Bool
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Always send the keepalive so the back route can be set up correctly and the tunnel
			// controller does not run into its update timeout, independent of ping latency.
			if err := tunnel.Send(clientIP, r.kubeAPIServerPodIP); err != nil {
				r.log.Info("error sending UDP packet with own IP to vpn-shoot", "ip", clientIP, "error", err)
			}
			// Only start a new ping if the previous one for this client already finished so a
			// slow/hanging ping never blocks the timer ticks.
			if pinging.CompareAndSwap(false, true) {
				go func() {
					defer pinging.Store(false)
					healthy := r.pinger.Ping(clientIP) == nil
					r.reconcileLink(clientIP, healthy, allClients)
				}()
			}
		}
	}
}

// reconcileLink updates the state of the ip6tnl link of a single shoot client based
// on its ping result. A healthy but down link is brought up, a failing but up link is set down.
// To avoid a complete outage, the last remaining up link is never set down. The mutex serializes
// the read-modify-write across the independent per-client loops.
func (r *clientRouter) reconcileLink(client net.IP, healthy bool, allClients []net.IP) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Fetch the state of all ip6tnl links with a single netlink call and correlate them to their shoot client IP.
	states, err := r.netRouter.getLinkStates(allClients)
	if err != nil {
		r.log.Error(err, "failed to get ip6tnl link states", "ip", client)
		return
	}

	up := states[client.String()]
	switch {
	case healthy && !up:
		if err := r.netRouter.setLinkState(client, true); err != nil {
			r.log.Error(err, "failed to set ip6tnl link up", "ip", client)
			return
		}
		r.log.Info("client recovered, setting ip6tnl link up", "ip", client)
	case !healthy && up:
		// This link is up. Only set it down if at least one other link is still up, so we never
		// cause a complete outage by bringing the last remaining link down.
		if !anyOtherLinkUp(states, client) {
			r.log.Info("client not healthy but not setting ip6tnl link down because it is the last healthy link", "ip", client)
			return
		}
		if err := r.netRouter.setLinkState(client, false); err != nil {
			r.log.Error(err, "failed to set ip6tnl link down", "ip", client)
			return
		}
		r.log.Info("client not healthy, setting ip6tnl link down", "ip", client)
	case !healthy:
		r.log.Info("client not healthy, ip6tnl link already down", "ip", client)
	}
}

// anyOtherLinkUp reports whether any link other than the given client's is up, based on the
// pre-fetched link states.
func anyOtherLinkUp(states map[string]bool, client net.IP) bool {
	key := client.String()
	for ip, up := range states {
		if ip == key {
			continue
		}
		if up {
			return true
		}
	}
	return false
}

type netlinkRouter struct {
	shootPodNetworks     []network.CIDR
	shootServiceNetworks []network.CIDR
	shootNodeNetworks    []network.CIDR
	seedPodNetwork       network.CIDR

	log logr.Logger
}

func (r *netlinkRouter) setupRouting(clientIPs []net.IP) error {
	// Build a nexthop for every shoot client's ip6tnl device so that all of them are used
	// simultaneously via an ECMP multipath route.
	var nexthops []*netlink.NexthopInfo
	for _, clientIP := range clientIPs {
		clientIndex := network.ClientIndexFromBondingShootClientIP(clientIP)
		linkName := network.BondIP6TunnelLinkName(clientIndex)
		tunnelLink, err := netlink.LinkByName(linkName)
		if err != nil {
			return fmt.Errorf("failed to get link %s: %w", linkName, err)
		}
		nexthops = append(nexthops, &netlink.NexthopInfo{LinkIndex: tunnelLink.Attrs().Index})
	}

	var (
		serviceNetworks []network.CIDR
		podNetworks     []network.CIDR
		nodeNetworks    []network.CIDR
	)

	// we don't need the specific mappings here because the /8 routes encompass all shoot networks
	_, _, _, err := network.ShootNetworksForNetmap(r.shootPodNetworks, r.shootServiceNetworks, r.shootNodeNetworks)
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
			route := network.MultiPathRouteForNetwork(n.ToIPNet(), nexthops)
			r.log.Info("replacing multipath route", "route", route, "net", n)
			if err = netlink.RouteReplace(&route); err != nil {
				return fmt.Errorf("error replacing route for %s: %w", n, err)
			}
		}
	}
	return nil
}

// setLinkState brings the ip6tnl device corresponding to the given shoot client IP up or down.
func (r *netlinkRouter) setLinkState(clientIP net.IP, up bool) error {
	clientIndex := network.ClientIndexFromBondingShootClientIP(clientIP)
	linkName := network.BondIP6TunnelLinkName(clientIndex)
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get link %s: %w", linkName, err)
	}
	if up {
		return netlink.LinkSetUp(link)
	}
	return netlink.LinkSetDown(link)
}

// getLinkStates reports the administrative state (up/down) of the ip6tnl device of every given
// shoot client, keyed by client IP. It uses a single netlink dump to fetch all links at once.
func (r *netlinkRouter) getLinkStates(clientIPs []net.IP) (map[string]bool, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to list links: %w", err)
	}
	upByName := make(map[string]bool, len(links))
	for _, link := range links {
		upByName[link.Attrs().Name] = link.Attrs().Flags&net.FlagUp != 0
	}

	states := make(map[string]bool, len(clientIPs))
	for _, clientIP := range clientIPs {
		clientIndex := network.ClientIndexFromBondingShootClientIP(clientIP)
		linkName := network.BondIP6TunnelLinkName(clientIndex)
		up, ok := upByName[linkName]
		if !ok {
			return nil, fmt.Errorf("link %s not found", linkName)
		}
		states[clientIP.String()] = up
	}
	return states, nil
}
