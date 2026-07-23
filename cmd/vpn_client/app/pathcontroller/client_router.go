// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pathcontroller

import (
	"context"
	"fmt"
	"net/netip"
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
	// mu serializes the reconcileNexthopGroup function across all clients
	mu sync.Mutex
}

type netRouter interface {
	// setupRouting sets up the static shoot-network routes pointing at the resilient ECMP next hop
	// groups built from every shoot client's ip6tnl device.
	setupRouting(clientIPs []netip.Addr) error
	// setNexthopHealth adjusts nexthop weights based on client health
	setNexthopHealth(clientIP netip.Addr, healthy bool, allClients []netip.Addr) error
	// getNexthopHealth returns the current health status of each client, keyed by client IP.
	getNexthopHealth(clientIPs []netip.Addr) (map[netip.Addr]bool, error)
}

type pinger interface {
	Ping(client netip.Addr) error
}

func (r *clientRouter) Run(ctx context.Context, clientIPs []netip.Addr) error {
	// Set up the shoot-network routes once. Afterward the route table is never touched again; only
	// the weights of the resilient ECMP next hop groups are adjusted as clients go down or recover.
	if err := r.netRouter.setupRouting(clientIPs); err != nil {
		return err
	}

	// Run an independent loop per shoot client so that a slow or hanging ping to one client
	// never blocks the ping and keepalive of the other client.
	var wg sync.WaitGroup
	for _, ip := range clientIPs {
		wg.Add(1)
		go func(clientIP netip.Addr) {
			defer wg.Done()
			r.runClient(ctx, clientIP, clientIPs)
		}(ip)
	}
	wg.Wait()
	return ctx.Err()
}

// runClient drives the lifecycle of a single shoot client: on every tick it sends the
// UDP keepalive, and it pings the client to reconcile next hop health-based weights.
func (r *clientRouter) runClient(ctx context.Context, clientIP netip.Addr, allClients []netip.Addr) {
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
			if err := tunnel.Send(clientIP.AsSlice(), r.kubeAPIServerPodIP); err != nil {
				r.log.Info("error sending UDP packet with own IP to vpn-shoot", "ip", clientIP, "error", err)
			}
			// Only start a new ping if the previous one for this client already finished so a
			// slow/hanging ping never blocks the timer ticks.
			if pinging.CompareAndSwap(false, true) {
				go func() {
					defer pinging.Store(false)
					healthy := r.pinger.Ping(clientIP) == nil
					r.updateNeighborCache(clientIP, healthy)
					r.reconcileNexthopGroup(clientIP, healthy, allClients)
				}()
			}
		}
	}
}

// reconcileNexthopGroup updates next hop weights for a single shoot client based on its health.
// When a client becomes unhealthy, its weight stays at default while others are set to overweight,
// causing graceful bucket migration away from it over time. When a client recovers,
// all weights are restored to default for equal distribution.
// If another client is already unhealthy, the new unhealthy client is ignored.
func (r *clientRouter) reconcileNexthopGroup(client netip.Addr, healthy bool, allClients []netip.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Fetch the current health state of all next hops
	states, err := r.netRouter.getNexthopHealth(allClients)
	if err != nil {
		r.log.Error(err, "failed to get current next hop health state", "clientIP", client, "allClients", allClients)
		return
	}

	// Compare health state (ping) vs. next hop weight (up / down)
	up := states[client]

	switch {
	case healthy && !up:
		r.log.Info("client recovered, restoring next hop weights", "ip", client)
	case !healthy && up:
		// Keep at most one unhealthy client at a time. If another client is already
		// marked unhealthy, do not mark this one unhealthy as well, otherwise the
		// weight skew would be canceled out.
		for otherClient, otherHealthy := range states {
			if otherClient != client && !otherHealthy {
				r.log.Info("client not healthy, but doing nothing because another unhealthy client already exists", "clientIP", client, "unhealthyPeer", otherClient)
				return
			}
		}
		r.log.Info("client not healthy, setting next hop to underweight for graceful migration", "ip", client)
	case !healthy:
		r.log.Info("client is still down and underweight", "ip", client)
		return
	default:
		// healthy and up - nothing to do
		return
	}
	// Update the health state and recalculate weights for all next hops
	if err = r.netRouter.setNexthopHealth(client, healthy, allClients); err != nil {
		r.log.Error(err, "failed to update next hop health", "ip", client, "healthy", healthy)
		return
	}
}

// updateNeighborCache deletes the neighbor table entry for a failed clientIP.
// This is necessary because the neighbor entry may be stale and cause the ping to fail even after the client has recovered.
func (r *clientRouter) updateNeighborCache(clientIP netip.Addr, healthy bool) {
	if !healthy {
		err := network.DeleteNeighborEntry(clientIP.AsSlice(), constants.BondDevice)

		if err != nil {
			r.log.Error(err, "failed to delete neighbor cache entry", "clientIP", clientIP)
		}
	}
}

type netlinkRouter struct {
	shootPodNetworks     []network.CIDR
	shootServiceNetworks []network.CIDR
	shootNodeNetworks    []network.CIDR
	seedPodNetwork       network.CIDR

	// clients holds all shoot client IPs; it is set during setupRouting.
	clients []netip.Addr
	// health tracks the current health status of each client's next hops.
	health map[netip.Addr]bool

	log logr.Logger
}

func (r *netlinkRouter) setupRouting(clientIPs []netip.Addr) error {
	r.clients = append([]netip.Addr(nil), clientIPs...)
	r.health = make(map[netip.Addr]bool, len(clientIPs))
	for _, clientIP := range clientIPs {
		r.health[clientIP] = true
	}

	// Create the per-client device next hops once.
	if err := r.ensureDeviceNexthops(clientIPs); err != nil {
		return fmt.Errorf("failed to set up next hops for ip6tnl devices: %w", err)
	}
	// Initialize resilient groups with all clients at equal weight (1).
	if err := r.adjustGroupWeights(clientIPs); err != nil {
		return fmt.Errorf("failed to initialize resilient next hop groups with ip6tnl device next hops: %w", err)
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

	// Point every shoot network route at the family-appropriate resilient next hop group. The routes
	// are static; only the weights of group members change when clients go down or recover.
	for _, nw := range nets {
		for _, n := range nw {
			dst := n.ToIPNet()
			groupID := constants.NexthopGroupIDforIPv4
			if dst.IP.To4() == nil {
				groupID = constants.NexthopGroupIDforIPv6
			}
			r.log.Info("replacing route via resilient next hop group", "net", n, "group", groupID)
			if err := network.ReplaceRouteViaNexthopGroup(dst, groupID); err != nil {
				return fmt.Errorf("error replacing route for %s: %w", n, err)
			}
		}
	}
	return nil
}

// ensureDeviceNexthops creates or updates the per-device next hop objects for all shoot clients.
func (r *netlinkRouter) ensureDeviceNexthops(clients []netip.Addr) error {
	for _, clientIP := range clients {
		clientIndex := network.ClientIndexFromBondingShootClientIP(clientIP.AsSlice())
		linkName := network.BondIP6TunnelLinkName(clientIndex)
		if _, err := netlink.LinkByName(linkName); err != nil {
			return fmt.Errorf("failed to get link %s: %w", linkName, err)
		}
		v4ID, v6ID := getNexthopIDsforClientIndex(clientIndex)
		if err := network.ReplaceDeviceNexthop(v4ID, linkName, false); err != nil {
			return err
		}
		if err := network.ReplaceDeviceNexthop(v6ID, linkName, true); err != nil {
			return err
		}
	}
	return nil
}

// adjustGroupWeights updates both IPv4 and IPv6 resilient groups to contain all clients with
// calculated weights based on their health status.
// If a client is unhealthy, it gets underweight while all others get overweight.
func (r *netlinkRouter) adjustGroupWeights(clients []netip.Addr) error {
	weights := make([]int, len(clients))
	var unhealthyClient netip.Addr
	unhealthyCount := 0
	for _, clientIP := range clients {
		if !r.health[clientIP] {
			unhealthyClient = clientIP
			unhealthyCount++
		}
	}

	// If exactly one client is unhealthy, make it underweight
	if unhealthyCount == 1 {
		for i, clientIP := range clients {
			if clientIP == unhealthyClient {
				weights[i] = constants.NexthopWeightDefault
			} else {
				weights[i] = constants.NexthopWeightOverweight
			}
		}
	} else {
		// All healthy or multiple unhealthy: equal weights
		for i := range weights {
			weights[i] = constants.NexthopWeightDefault
		}
	}

	v4IDs := make([]int, 0, len(clients))
	v4Weights := make([]int, 0, len(clients))
	v6IDs := make([]int, 0, len(clients))
	v6Weights := make([]int, 0, len(clients))
	for i, clientIP := range clients {
		clientIndex := network.ClientIndexFromBondingShootClientIP(clientIP.AsSlice())
		v4ID, v6ID := getNexthopIDsforClientIndex(clientIndex)
		v4IDs = append(v4IDs, v4ID)
		v4Weights = append(v4Weights, weights[i])
		v6IDs = append(v6IDs, v6ID)
		v6Weights = append(v6Weights, weights[i])
	}
	if len(v4IDs) == 0 {
		return fmt.Errorf("no shoot client next hops to configure")
	}
	if err := network.ReplaceResilientNexthopGroup(constants.NexthopGroupIDforIPv4, v4IDs, v4Weights,
		constants.ResilientNexthopBuckets, constants.ResilientNexthopIdleTimer, constants.ResilientNexthopUnbalancedTimer); err != nil {
		return err
	}
	return network.ReplaceResilientNexthopGroup(constants.NexthopGroupIDforIPv6, v6IDs, v6Weights,
		constants.ResilientNexthopBuckets, constants.ResilientNexthopIdleTimer, constants.ResilientNexthopUnbalancedTimer)
}

// setNexthopHealth updates the health status of a client and recalculates weights for all nexthops.
func (r *netlinkRouter) setNexthopHealth(clientIP netip.Addr, healthy bool, allClients []netip.Addr) error {
	r.health[clientIP] = healthy
	return r.adjustGroupWeights(allClients)
}

// getNexthopHealth reports the current health status of each shoot client's nexthops, keyed by client IP.
func (r *netlinkRouter) getNexthopHealth(clientIPs []netip.Addr) (map[netip.Addr]bool, error) {
	states := make(map[netip.Addr]bool, len(clientIPs))
	for _, clientIP := range clientIPs {
		states[clientIP] = r.health[clientIP]
	}
	return states, nil
}

// getNexthopIDsforClientIndex returns the correct next hop ID to be used for a vpn client based on its index.
// The ID is the base ID for the next hop type (IPv4 or IPv6) plus the client index.
func getNexthopIDsforClientIndex(clientIndex int) (v4ID, v6ID int) {
	v4ID = constants.NexthopDeviceBaseIDforIPv4 + clientIndex
	v6ID = constants.NexthopDeviceBaseIDforIPv6 + clientIndex

	return
}
