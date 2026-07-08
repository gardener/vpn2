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
	// mu serializes the reconcileNexthopGroup function across all clients
	mu sync.Mutex
}

type netRouter interface {
	// setupRouting sets up the static shoot-network routes pointing at the resilient ECMP nexthop
	// groups built from every shoot client's ip6tnl device.
	setupRouting(clientIPs []net.IP) error
	// setNexthopMember adds/removes the shoot client's nexthops to/from the resilient groups.
	setNexthopMember(clientIP net.IP, member bool) error
	// getNexthopGroupMembers returns whether each shoot client's nexthops are currently active members
	// of the resilient groups, keyed by client IP.
	getNexthopGroupMembers(clientIPs []net.IP) (map[string]bool, error)
}

type pinger interface {
	Ping(client net.IP) error
}

func (r *clientRouter) Run(ctx context.Context, clientIPs []net.IP) error {
	// Set up the shoot-network routes once. Afterward the route table is never touched again; only
	// the membership of the resilient ECMP nexthop groups is changed as clients go down or recover.
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
// UDP keepalive and it pings the client to reconcile resilient-group membership.
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
					r.reconcileNexthopGroup(clientIP, healthy, allClients)
				}()
			}
		}
	}
}

// reconcileNexthopGroup updates resilient-group membership for a single shoot client based on its ping
// result. A healthy but inactive member is added back to the groups, a failing active member is
// removed from the groups. To avoid a complete outage, the last remaining active member is never
// removed. The mutex serializes the read-modify-write across the independent per-client loops.
func (r *clientRouter) reconcileNexthopGroup(client net.IP, healthy bool, allClients []net.IP) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Fetch the state of all ip6tnl links with a single netlink call and correlate them to their shoot client IP.
	states, err := r.netRouter.getNexthopGroupMembers(allClients)
	if err != nil {
		r.log.Error(err, "failed to get ip6tnl link states", "ip", client)
		return
	}

	up := states[client.String()]
	switch {
	case healthy && !up:
		if err := r.netRouter.setNexthopMember(client, true); err != nil {
			r.log.Error(err, "failed to set ip6tnl link up", "ip", client)
			return
		}
		r.log.Info("client recovered, adding nexthop back to resilient groups", "ip", client)
	case !healthy && up:
		// This link is up. Only set it down if at least one other link is still up, so we never
		// cause a complete outage by bringing the last remaining link down.
		if !anyOtherLinkUp(states, client) {
			r.log.Info("client not healthy but not removing nexthop because it is the last healthy member", "ip", client)
			return
		}
		if err := r.netRouter.setNexthopMember(client, false); err != nil {
			r.log.Error(err, "failed to set ip6tnl link down", "ip", client)
			return
		}
		r.log.Info("client not healthy, removing nexthop from resilient groups", "ip", client)
	case !healthy:
		r.log.Info("client not healthy, nexthop already inactive in resilient groups", "ip", client)
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

	// clients holds all shoot client IPs; it is set during setupRouting.
	clients []net.IP
	// members tracks whether a client's nexthops are currently active members of the resilient groups.
	members map[string]bool

	log logr.Logger
}

func (r *netlinkRouter) setupRouting(clientIPs []net.IP) error {
	r.clients = append([]net.IP(nil), clientIPs...)
	r.members = make(map[string]bool, len(clientIPs))
	for _, clientIP := range clientIPs {
		r.members[clientIP.String()] = true
	}

	// Create the per-client device nexthops once and initialize resilient groups with all clients.
	if err := r.ensureDeviceNexthops(clientIPs); err != nil {
		return err
	}
	if err := r.replaceGroupMembership(clientIPs); err != nil {
		return err
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

	// Point every shoot-network route at the family-appropriate resilient nexthop group. The routes
	// are static; only the group membership changes when clients go down or recover.
	for _, nw := range nets {
		for _, n := range nw {
			dst := n.ToIPNet()
			groupID := constants.NexthopGroupIDforIPv4
			if dst.IP.To4() == nil {
				groupID = constants.NexthopGroupIDforIPv6
			}
			r.log.Info("replacing route via resilient nexthop group", "net", n, "group", groupID)
			if err := network.ReplaceRouteViaNexthopGroup(dst, groupID); err != nil {
				return fmt.Errorf("error replacing route for %s: %w", n, err)
			}
		}
	}
	return nil
}

// ensureDeviceNexthops creates or updates the per-device nexthop objects for all shoot clients.
func (r *netlinkRouter) ensureDeviceNexthops(clients []net.IP) error {
	for _, clientIP := range clients {
		clientIndex := network.ClientIndexFromBondingShootClientIP(clientIP)
		linkName := network.BondIP6TunnelLinkName(clientIndex)
		if _, err := netlink.LinkByName(linkName); err != nil {
			return fmt.Errorf("failed to get link %s: %w", linkName, err)
		}
		v4ID := constants.NexthopDeviceBaseIDforIPv4 + clientIndex
		v6ID := constants.NexthopDeviceBaseIDforIPv6 + clientIndex
		if err := network.ReplaceDeviceNexthop(v4ID, linkName, false); err != nil {
			return err
		}
		if err := network.ReplaceDeviceNexthop(v6ID, linkName, true); err != nil {
			return err
		}
	}
	return nil
}

// membersAsIPs returns all clients currently marked as group members.
func (r *netlinkRouter) membersAsIPs() []net.IP {
	active := make([]net.IP, 0, len(r.clients))
	for _, clientIP := range r.clients {
		if r.members[clientIP.String()] {
			active = append(active, clientIP)
		}
	}
	return active
}

// replaceGroupMembership updates both IPv4 and IPv6 resilient groups to contain exactly the given
// shoot clients.
func (r *netlinkRouter) replaceGroupMembership(clients []net.IP) error {
	v4IDs := make([]int, 0, len(clients))
	v6IDs := make([]int, 0, len(clients))
	for _, clientIP := range clients {
		clientIndex := network.ClientIndexFromBondingShootClientIP(clientIP)
		v4IDs = append(v4IDs, constants.NexthopDeviceBaseIDforIPv4+clientIndex)
		v6IDs = append(v6IDs, constants.NexthopDeviceBaseIDforIPv6+clientIndex)
	}
	if len(v4IDs) == 0 {
		return fmt.Errorf("no shoot client nexthops to configure")
	}
	if err := network.ReplaceResilientNexthopGroup(constants.NexthopGroupIDforIPv4, v4IDs,
		constants.ResilientNexthopBuckets, constants.ResilientNexthopIdleTimer, constants.ResilientNexthopUnbalancedTimer); err != nil {
		return err
	}
	return network.ReplaceResilientNexthopGroup(constants.NexthopGroupIDforIPv6, v6IDs,
		constants.ResilientNexthopBuckets, constants.ResilientNexthopIdleTimer, constants.ResilientNexthopUnbalancedTimer)
}

// setNexthopMember updates resilient-group membership for the given shoot client
func (r *netlinkRouter) setNexthopMember(clientIP net.IP, member bool) error {
	key := clientIP.String()
	current := r.members[key]
	if current == member {
		return nil
	}
	r.members[key] = member
	active := r.membersAsIPs()
	if err := r.replaceGroupMembership(active); err != nil {
		r.members[key] = current
		return err
	}
	return nil
}

// getNexthopGroupMembers reports whether each shoot client's nexthops are currently members of the
// resilient groups, keyed by client IP.
func (r *netlinkRouter) getNexthopGroupMembers(clientIPs []net.IP) (map[string]bool, error) {
	states := make(map[string]bool, len(clientIPs))
	for _, clientIP := range clientIPs {
		states[clientIP.String()] = r.members[clientIP.String()]
	}
	return states, nil
}
