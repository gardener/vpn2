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

	log logr.Logger
	mu  sync.Mutex
	// linkUp tracks the desired/believed administrative state of the ip6tnl link for each
	// shoot client IP. It is used to ensure that we never set all ip6tnl links down at the
	// same time, which would cause a complete outage.
	linkUp map[string]bool
	// routingConfigured indicates whether the ECMP multipath routing has been set up successfully.
	routingConfigured bool
	ticker            *time.Ticker
}

type netRouter interface {
	// updateRouting sets up the ECMP multipath routes for all shoot networks using every
	// shoot client's ip6tnl device as a nexthop.
	updateRouting(clientIPs []net.IP) error
	// setLinkState brings the ip6tnl device corresponding to the given shoot client IP up or down.
	setLinkState(clientIP net.IP, up bool) error
}

type pinger interface {
	Ping(client net.IP) error
}

func (r *clientRouter) Run(ctx context.Context, clientIPs []net.IP) error {
	// Initially we assume all ip6tnl links are up (they are created and set up during bonding setup).
	for _, ip := range clientIPs {
		r.linkUp[ip.String()] = true
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.ticker.C:
			if !r.routingConfigured {
				// Set up the ECMP multipath route once. This may fail during startup when the
				// ip6tnl links are not ready yet, so we retry on every tick until it succeeds.
				if err := r.netRouter.updateRouting(clientIPs); err != nil {
					r.log.Error(err, "error configuring multipath routing, will retry")
				} else {
					r.routingConfigured = true
				}
			}
			healthy := r.pingAllShootClients(clientIPs)
			r.reconcileLinks(clientIPs, healthy)
		}
	}
}

// reconcileLinks updates the administrative state of the ip6tnl links based on the ping results.
// Healthy links are brought up, failing links are set down so that Linux stops using them for
// sending traffic. The route table itself is kept as-is (the multipath route stays intact), so
// existing connections on the still-healthy link are not killed. To avoid a complete outage, the
// last remaining healthy link is never set down.
func (r *clientRouter) reconcileLinks(clients []net.IP, healthy map[string]bool) {
	// First bring up links that became healthy again.
	for _, client := range clients {
		key := client.String()
		if healthy[key] && !r.linkUp[key] {
			if err := r.netRouter.setLinkState(client, true); err != nil {
				r.log.Error(err, "failed to set ip6tnl link up", "ip", client)
				continue
			}
			r.log.Info("ping succeeded again, setting ip6tnl link up", "ip", client)
			r.linkUp[key] = true
		}
	}

	// Then set down links that are failing, but never set down the last remaining healthy link.
	for _, client := range clients {
		key := client.String()
		if !healthy[key] && r.linkUp[key] {
			if r.countUpLinks() <= 1 {
				r.log.Info("ping failed but not setting ip6tnl link down because it is the last healthy link; keeping route table as-is to avoid a complete outage", "ip", client)
				continue
			}
			if err := r.netRouter.setLinkState(client, false); err != nil {
				r.log.Error(err, "failed to set ip6tnl link down", "ip", client)
				continue
			}
			r.log.Info("ping failed, setting ip6tnl link down so Linux stops using it for sending traffic", "ip", client)
			r.linkUp[key] = false
		}
	}
}

// countUpLinks returns the number of ip6tnl links that are currently believed to be up.
func (r *clientRouter) countUpLinks() int {
	count := 0
	for _, up := range r.linkUp {
		if up {
			count++
		}
	}
	return count
}

// pingAllShootClients pings every shoot client via its ip6tnl device and returns a map indicating
// whether each client (keyed by its IP string) is healthy.
func (r *clientRouter) pingAllShootClients(clients []net.IP) map[string]bool {
	var wg sync.WaitGroup
	healthy := make(map[string]bool, len(clients))
	for _, client := range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := r.pinger.Ping(client)
			r.mu.Lock()
			defer r.mu.Unlock()
			if err != nil {
				r.log.Info("ping to vpn-shoot failed", "ip", client)
				healthy[client.String()] = false
			} else {
				healthy[client.String()] = true
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
	return healthy
}

type netlinkRouter struct {
	shootPodNetworks     []network.CIDR
	shootServiceNetworks []network.CIDR
	shootNodeNetworks    []network.CIDR
	seedPodNetwork       network.CIDR

	log logr.Logger
}

func (r *netlinkRouter) updateRouting(clientIPs []net.IP) error {
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
			route := multiPathRouteForNetwork(n.ToIPNet(), nexthops)
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

// multiPathRouteForNetwork builds a route for the given network using all provided nexthops as an
// ECMP multipath route. Each route gets its own copy of the nexthop entries so that netlink does
// not share/mutate slices between routes.
func multiPathRouteForNetwork(dst *net.IPNet, nexthops []*netlink.NexthopInfo) netlink.Route {
	multiPath := make([]*netlink.NexthopInfo, len(nexthops))
	for i, nh := range nexthops {
		multiPath[i] = &netlink.NexthopInfo{LinkIndex: nh.LinkIndex}
	}
	return netlink.Route{
		Dst:       dst,
		MultiPath: multiPath,
	}
}
