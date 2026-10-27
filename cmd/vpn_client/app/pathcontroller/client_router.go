// SPDX-FileCopyrightText: Contributors to the Gardener project
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

	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/gardener/vpn2/pkg/shoot_client/tunnel"
	vpnclient "github.com/gardener/vpn2/pkg/vpn_client"
)

type clientRouter struct {
	pinger             pinger
	netRouter          netRouter
	kubeAPIServerPodIP string
	updateInterval     time.Duration

	log logr.Logger
	// mu serializes the reconcileRoutingPolicy function across all clients
	mu sync.Mutex
}

type netRouter interface {
	// setupRouting sets up the per-client routing tables, fwmark ip rules, and iptables mark rules.
	setupRouting(clientIPs []netip.Addr) error
	// setShootClientHealth redirects or restores the fwmark ip rule for clientIP based on health.
	setShootClientHealth(clientIP netip.Addr, healthy bool, allClients []netip.Addr) error
	// getShootClientHealth returns the current health status of each client, keyed by client IP.
	getShootClientHealth(clientIPs []netip.Addr) (map[netip.Addr]bool, error)
}

type pinger interface {
	Ping(client netip.Addr) error
}

func (r *clientRouter) Run(ctx context.Context, clientIPs []netip.Addr) error {
	// Set up routing once. Afterward only fwmark ip rules are adjusted on health changes.
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
// UDP keepalive, and it pings the client to reconcile the routing policy (single-path vs. multipath) based on the ping result.
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
					r.reconcileRoutingPolicy(clientIP, healthy, allClients)
				}()
			}
		}
	}
}

// reconcileRoutingPolicy updates the fwmark ip rule for a single shoot client based on its health.
// When a client becomes unhealthy, its mark is redirected to the surviving client's table.
// When a client recovers, its mark is restored to its own routing table.
// If another client is already unhealthy, the new unhealthy client is ignored.
func (r *clientRouter) reconcileRoutingPolicy(client netip.Addr, healthy bool, allClients []netip.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Fetch the current health state of all shoot clients
	states, err := r.netRouter.getShootClientHealth(allClients)
	if err != nil {
		r.log.Error(err, "failed to get current shoot client health state", "clientIP", client, "allClients", allClients)
		return
	}

	// Compare health state (ping) vs. shoot client health (up / down)
	up := states[client]

	switch {
	case healthy && !up:
		r.log.Info("client recovered, restoring multi-path routing policy", "ip", client)
	case !healthy && up:
		// Keep at most one unhealthy client at a time. If another client is already
		// marked unhealthy, do not mark this one unhealthy as well, otherwise the
		// redirection would be canceled out.
		for otherClient, otherHealthy := range states {
			if otherClient != client && !otherHealthy {
				r.log.Info("client not healthy, but doing nothing because another unhealthy client already exists", "clientIP", client, "unhealthyPeer", otherClient)
				return
			}
		}
		r.log.Info("client not healthy, setting single-path routing policy to surviving client", "ip", client)
		// Delete the neighbor cache entry for the failed client to avoid stale entries causing ping failures after recovery.
		r.deleteNeighborCache(client)
	case !healthy:
		r.log.Info("client is still down", "ip", client)
		return
	default:
		// healthy and up - nothing to do
		return
	}
	// Update the health state and recalculate routing rules for all clients
	if err = r.netRouter.setShootClientHealth(client, healthy, allClients); err != nil {
		r.log.Error(err, "failed to update shoot client health", "ip", client, "healthy", healthy)
		return
	}
}

// deleteNeighborCache deletes the neighbor table entry for a failed clientIP.
// This is necessary because the neighbor entry may be stale and cause the ping to fail even after the client has recovered.
func (r *clientRouter) deleteNeighborCache(clientIP netip.Addr) {
	err := network.DeleteNeighborEntry(clientIP.AsSlice(), constants.BondDevice)

	if err != nil {
		r.log.Error(err, "failed to delete neighbor cache entry", "clientIP", clientIP)
	}
}

type netlinkRouter struct {
	shootPodNetworks     []network.CIDR
	shootServiceNetworks []network.CIDR
	shootNodeNetworks    []network.CIDR
	seedPodNetwork       network.CIDR

	// clients holds all shoot client IPs; it is set during setupRouting.
	clients []netip.Addr
	// health tracks the current health status of each client.
	health map[netip.Addr]bool

	log logr.Logger
}

func (r *netlinkRouter) setupRouting(clientIPs []netip.Addr) error {
	r.clients = append([]netip.Addr(nil), clientIPs...)
	r.health = make(map[netip.Addr]bool, len(clientIPs))
	for _, clientIP := range clientIPs {
		r.health[clientIP] = true
	}

	var (
		serviceNetworks []network.CIDR
		podNetworks     []network.CIDR
		nodeNetworks    []network.CIDR
	)

	// Check if there is an overlap between the seed pod network and shoot networks.
	overlap := network.OverLapAny(r.seedPodNetwork, slices.Concat(r.shootPodNetworks, r.shootServiceNetworks, r.shootNodeNetworks)...)

	// IPv4 networks are mapped to 240/4, IPv6 networks are kept as-is
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

	// Install per-client routing tables: each shoot network is routed via the client's ip6tnl device
	// in a dedicated table. The fwmark ip rules below steer new connections into the right table.
	for _, clientIP := range clientIPs {
		clientIndex := network.ClientIndexFromBondingShootClientIP(clientIP.AsSlice())
		linkName := network.BondIP6TunnelLinkName(clientIndex)
		tableID := constants.RoutingTableBaseID + clientIndex
		for _, nw := range nets {
			for _, n := range nw {
				dst := n.ToIPNet()
				r.log.Info("replacing route in per-client table", "net", n, "link", linkName, "table", tableID)
				if err := network.ReplaceRouteViaDevice(dst, linkName, tableID); err != nil {
					return fmt.Errorf("error replacing route for %s in table %d: %w", n, tableID, err)
				}
			}
		}
	}

	// IPv6 shoot networks need a loopback route in the main table so the initial routing lookup
	// for locally-generated packets succeeds. Without it the kernel returns ENETUNREACHABLE before
	// iptables mangle OUTPUT fires, preventing the fwmark from being set.
	for _, nw := range nets {
		for _, n := range nw {
			if n.IP.To4() == nil {
				if err := network.AddLoopbackRoute(n.ToIPNet()); err != nil {
					return fmt.Errorf("error adding loopback route for %s: %w", n, err)
				}
			}
		}
	}

	// Install fwmark ip rules: one per client, both IPv4 and IPv6.
	for _, clientIP := range clientIPs {
		clientIndex := network.ClientIndexFromBondingShootClientIP(clientIP.AsSlice())
		tableID := constants.RoutingTableBaseID + clientIndex
		mark := constants.FwmarkBase + clientIndex
		priority := constants.FwmarkRulePriorityBase + clientIndex
		for _, ipv6 := range []bool{false, true} {
			if err := network.AddFWmarkRule(mark, tableID, priority, ipv6); err != nil {
				return fmt.Errorf("error adding fwmark rule for client %d: %w", clientIndex, err)
			}
		}
	}

	// Install iptables MARK/CONNMARK rules to distribute new connections across shoot clients.
	// Pass the effective (post-overlap-mapping) networks so only shoot-destined traffic is marked.
	allNets := slices.Concat(nets...)
	if err := vpnclient.SetMarkRules(r.log, allNets, len(clientIPs)); err != nil {
		return fmt.Errorf("error setting iptables mark rules: %w", err)
	}

	return nil
}

// setShootClientHealth updates the health status of a client and adjusts fwmark ip rules accordingly.
func (r *netlinkRouter) setShootClientHealth(clientIP netip.Addr, healthy bool, allClients []netip.Addr) error {
	r.health[clientIP] = healthy

	clientIndex := network.ClientIndexFromBondingShootClientIP(clientIP.AsSlice())
	mark := constants.FwmarkBase + clientIndex
	priority := constants.FwmarkRulePriorityBase + clientIndex

	var targetTableID int
	if healthy {
		// Restore the client's mark to its own routing table.
		targetTableID = constants.RoutingTableBaseID + clientIndex
	} else {
		// Redirect the client's mark to a surviving client's table.
		for _, other := range allClients {
			if other != clientIP && r.health[other] {
				otherIndex := network.ClientIndexFromBondingShootClientIP(other.AsSlice())
				targetTableID = constants.RoutingTableBaseID + otherIndex
				break
			}
		}
		if targetTableID == 0 {
			// No healthy client found — leave the rule as-is to avoid dropping all connectivity.
			return nil
		}
	}

	for _, ipv6 := range []bool{false, true} {
		if err := network.ReplaceFWmarkRule(mark, targetTableID, priority, ipv6); err != nil {
			return fmt.Errorf("error replacing fwmark rule for client %d: %w", clientIndex, err)
		}
	}
	return nil
}

// getShootClientHealth reports the current health status of each shoot client, keyed by client IP.
func (r *netlinkRouter) getShootClientHealth(clientIPs []netip.Addr) (map[netip.Addr]bool, error) {
	states := make(map[netip.Addr]bool, len(clientIPs))
	for _, clientIP := range clientIPs {
		states[clientIP] = r.health[clientIP]
	}
	return states, nil
}
