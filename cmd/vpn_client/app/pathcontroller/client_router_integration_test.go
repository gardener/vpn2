// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package pathcontroller

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
)

const (
	shootPodNetworkV4 = "10.250.0.0/16"
	shootPodNetworkV6 = "fd00:250::/64"
	shootTCP4Addr     = "10.250.0.2:28080"
	shootTCP6Addr     = "[fd00:250::2]:28081"
	seedSrc4Addr      = "10.251.0.1"
	seedSrc6Addr      = "fd00:251::1"
)

var scenarioSeq atomic.Uint32

var _ = Describe("ClientRouter policy routing integration", Serial, func() {
	It("shoot is IPv4 only", func() {
		runPolicyRoutingScenario(true, false)
	})

	It("shoot is IPv6 only", func() {
		runPolicyRoutingScenario(false, true)
	})

	It("shoot is dual-stack IPv4 + IPv6", func() {
		runPolicyRoutingScenario(true, true)
	})

	It("new connections succeed while fwmark rules are being switched", func() {
		runFwmarkSwitchUnderLoadScenario()
	})

	It("non-shoot traffic is not marked", func() {
		runNonShootTrafficUnmarkedScenario()
	})
})

type trafficCase struct {
	name    string
	network string
	localIP string
	remote  string
}

func runPolicyRoutingScenario(enableIPv4, enableIPv6 bool) {
	suffix := fmt.Sprintf("%04x", scenarioSeq.Add(1))
	nsName := "pcint-" + suffix
	vethHost := "pcvh" + suffix
	vethPeer := "pcvp" + suffix
	rt0 := "rt0" + suffix
	rt1 := "rt1" + suffix

	clientNetIPs := []net.IP{
		network.BondingShootClientIP(network.ParseIPNetIgnoreError(constants.DefaultVPNNetwork.String()).ToIPNet(), 0),
		network.BondingShootClientIP(network.ParseIPNetIgnoreError(constants.DefaultVPNNetwork.String()).ToIPNet(), 1),
	}
	// Convert []net.IP to []netip.Addr
	clientIPs, err := network.IPtoAddr(clientNetIPs)
	Expect(err).NotTo(HaveOccurred())
	link0 := network.BondIP6TunnelLinkName(0)
	link1 := network.BondIP6TunnelLinkName(1)

	cleanup := func() {
		_ = runIP("route", "del", shootPodNetworkV4)
		_ = runIP("-6", "route", "del", shootPodNetworkV6)
		// Remove loopback routes installed by setupRouting for IPv6 shoot networks.
		_ = runIP("-6", "route", "del", "fd00:250::2/128", "dev", "lo")
		// Remove per-client routing table routes
		for _, tableID := range []int{constants.RoutingTableBaseID, constants.RoutingTableBaseID + 1} {
			_ = runIP("route", "flush", "table", fmt.Sprintf("%d", tableID))
			_ = runIP("-6", "route", "flush", "table", fmt.Sprintf("%d", tableID))
		}
		// Remove fwmark ip rules
		for i := range 2 {
			mark := constants.FwmarkBase + i
			priority := constants.FwmarkRulePriorityBase + i
			deleteAnyFWmarkRule(mark, priority, false)
			deleteAnyFWmarkRule(mark, priority, true)
		}
		// Flush iptables mangle OUTPUT rules and SHOOT_MARK chain installed by SetMarkRules
		_ = exec.Command("iptables", "-t", "mangle", "-F", "OUTPUT").Run()
		_ = exec.Command("iptables", "-t", "mangle", "-F", constants.ShootMarkChain).Run()
		_ = exec.Command("iptables", "-t", "mangle", "-X", constants.ShootMarkChain).Run()
		_ = exec.Command("ip6tables", "-t", "mangle", "-F", "OUTPUT").Run()
		_ = exec.Command("ip6tables", "-t", "mangle", "-F", constants.ShootMarkChain).Run()
		_ = exec.Command("ip6tables", "-t", "mangle", "-X", constants.ShootMarkChain).Run()
		_ = runIP("addr", "del", seedSrc4Addr+"/32", "dev", "lo")
		_ = runIP("-6", "addr", "del", seedSrc6Addr+"/128", "dev", "lo")
		_ = network.DeleteLinkByName(link0)
		_ = network.DeleteLinkByName(link1)
		_ = runIP("link", "del", vethHost)
		_ = runIP("netns", "del", nsName)
	}
	// Shared tunnel objects can survive failed runs; clear them before setup.
	cleanup()
	DeferCleanup(cleanup)

	Expect(runIP("netns", "add", nsName)).To(Succeed())
	Expect(runIP("link", "add", vethHost, "type", "veth", "peer", "name", vethPeer)).To(Succeed())
	Expect(runIP("link", "set", vethPeer, "netns", nsName)).To(Succeed())

	Expect(runIP("addr", "add", "2001:db8:1::101/64", "dev", vethHost)).To(Succeed())
	Expect(runIP("addr", "add", "2001:db8:1::102/64", "dev", vethHost)).To(Succeed())
	Expect(runIP("link", "set", vethHost, "up")).To(Succeed())

	Expect(runIP("-n", nsName, "link", "set", "lo", "up")).To(Succeed())
	Expect(runIP("-n", nsName, "addr", "add", "2001:db8:1::201/64", "dev", vethPeer)).To(Succeed())
	Expect(runIP("-n", nsName, "addr", "add", "2001:db8:1::202/64", "dev", vethPeer)).To(Succeed())
	Expect(runIP("-n", nsName, "link", "set", vethPeer, "up")).To(Succeed())

	Expect(runIP("-6", "tunnel", "add", link0, "mode", "any", "local", "2001:db8:1::101", "remote", "2001:db8:1::201", "dev", vethHost)).To(Succeed())
	Expect(runIP("-6", "tunnel", "add", link1, "mode", "any", "local", "2001:db8:1::102", "remote", "2001:db8:1::202", "dev", vethHost)).To(Succeed())
	Expect(runIP("link", "set", link0, "up")).To(Succeed())
	Expect(runIP("link", "set", link1, "up")).To(Succeed())

	Expect(runIP("-n", nsName, "-6", "tunnel", "add", rt0, "mode", "any", "local", "2001:db8:1::201", "remote", "2001:db8:1::101", "dev", vethPeer)).To(Succeed())
	Expect(runIP("-n", nsName, "-6", "tunnel", "add", rt1, "mode", "any", "local", "2001:db8:1::202", "remote", "2001:db8:1::102", "dev", vethPeer)).To(Succeed())
	Expect(runIP("-n", nsName, "link", "set", rt0, "up")).To(Succeed())
	Expect(runIP("-n", nsName, "link", "set", rt1, "up")).To(Succeed())

	Expect(disableRPFilter(vethHost)).To(Succeed())
	Expect(runInNetns(nsName, "sh", "-c", "echo 0 > /proc/sys/net/ipv4/conf/all/rp_filter; echo 0 > /proc/sys/net/ipv4/conf/default/rp_filter; echo 0 > /proc/sys/net/ipv4/conf/lo/rp_filter; echo 0 > /proc/sys/net/ipv4/conf/"+rt0+"/rp_filter; echo 0 > /proc/sys/net/ipv4/conf/"+rt1+"/rp_filter; echo 0 > /proc/sys/net/ipv4/conf/"+vethPeer+"/rp_filter")).To(Succeed())

	var traffic []trafficCase
	var shootPodNetworks []network.CIDR

	if enableIPv4 {
		Expect(runIP("-n", nsName, "addr", "add", "10.250.0.2/32", "dev", "lo")).To(Succeed())
		Expect(runIP("addr", "add", seedSrc4Addr+"/32", "dev", "lo")).To(Succeed())
		Expect(runIP("-n", nsName, "route", "replace", seedSrc4Addr+"/32", "nexthop", "dev", rt0, "nexthop", "dev", rt1)).To(Succeed())
		stop4, err := startNamespaceEchoServer(nsName, "tcp4", shootTCP4Addr)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(stop4)
		traffic = append(traffic, trafficCase{name: "ipv4", network: "tcp4", localIP: seedSrc4Addr, remote: shootTCP4Addr})
		shootPodNetworks = append(shootPodNetworks, network.ParseIPNetIgnoreError("10.250.0.2/32"))
	}

	if enableIPv6 {
		Expect(runIP("-n", nsName, "-6", "addr", "add", "fd00:250::2/128", "dev", "lo")).To(Succeed())
		Expect(runIP("-6", "addr", "add", seedSrc6Addr+"/128", "dev", "lo")).To(Succeed())
		// Back-routes: the remote namespace needs to reach seedSrc6Addr. IPv6 device-only
		// multipath routes are not supported, so route via rt0 only; rt1 handles forward traffic.
		Expect(runIP("-n", nsName, "-6", "route", "replace", seedSrc6Addr+"/128", "dev", rt0)).To(Succeed())
		// Main-table forward route for IPv6 connectivity: ip6tables mangle does not work in this
		// container (nf_conntrack_ipv6 unavailable in Docker Desktop on Mac), so marks are never
		// set and fwmark rules never match. Add a plain main-table route so the echo server is
		// reachable; per-client table routes and ip rules are verified by the assertions above.
		// IPv6 device-only multipath is not supported, so route via link0 only.
		Expect(runIP("-6", "route", "replace", "fd00:250::2/128", "dev", link0)).To(Succeed())
		stop6, err := startNamespaceEchoServer(nsName, "tcp6", shootTCP6Addr)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(stop6)
		traffic = append(traffic, trafficCase{name: "ipv6", network: "tcp6", localIP: seedSrc6Addr, remote: shootTCP6Addr})
		shootPodNetworks = append(shootPodNetworks, network.ParseIPNetIgnoreError("fd00:250::2/128"))
	}

	Expect(traffic).NotTo(BeEmpty())

	netRouter := &netlinkRouter{
		shootPodNetworks: shootPodNetworks,
		seedPodNetwork:   network.ParseIPNetIgnoreError("10.10.0.0/16"),
		log:              logr.Discard(),
	}
	Expect(netRouter.setupRouting(clientIPs)).To(Succeed())

	By("verifying per-client routing tables are populated")
	for i, clientIP := range clientIPs {
		clientIndex := network.ClientIndexFromBondingShootClientIP(clientIP.AsSlice())
		tableID := constants.RoutingTableBaseID + clientIndex
		linkName := network.BondIP6TunnelLinkName(i)
		for _, cidr := range shootPodNetworks {
			ipFamily := "-4"
			if cidr.IP.To4() == nil {
				ipFamily = "-6"
			}
			out, err := runIPOut(ipFamily, "route", "show", "table", fmt.Sprintf("%d", tableID))
			Expect(err).NotTo(HaveOccurred(), "table %d should be queryable", tableID)
			Expect(out).To(ContainSubstring(linkName), "table %d should route via %s", tableID, linkName)
		}
	}

	if enableIPv6 {
		By("verifying loopback routes are installed in main table for IPv6 shoot networks")
		for _, cidr := range shootPodNetworks {
			if cidr.IP.To4() != nil {
				continue
			}
			out, err := runIPOut("-6", "route", "show", cidr.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(ContainSubstring("dev lo"), "IPv6 shoot network %s must have a lo route in the main table so the initial routing lookup succeeds before mangle OUTPUT fires", cidr)
		}
	}

	By("verifying fwmark ip rules are installed")
	for i := range clientIPs {
		clientIndex := network.ClientIndexFromBondingShootClientIP(clientIPs[i].AsSlice())
		mark := constants.FwmarkBase + clientIndex
		tableID := constants.RoutingTableBaseID + clientIndex
		for _, ipv6 := range []bool{false, true} {
			family := netlink.FAMILY_V4
			if ipv6 {
				family = netlink.FAMILY_V6
			}
			rules, err := netlink.RuleList(family)
			Expect(err).NotTo(HaveOccurred())
			found := false
			for _, r := range rules {
				if r.Mark == uint32(mark) && r.Table == tableID { //nolint:gosec
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "fwmark rule for mark %d → table %d (ipv6=%v) should exist", mark, tableID, ipv6)
		}
	}

	router := &clientRouter{netRouter: netRouter, log: logr.Discard()}

	// trafficV4 is the subset of traffic cases that use IPv4; only these can be used for
	// fwmark-based routing tests because ip6tables mangle OUTPUT does not fire in the
	// Docker Desktop on Mac kernel (nf_conntrack_ipv6 is unavailable).
	var trafficV4 []trafficCase
	for _, tc := range traffic {
		if tc.network == "tcp4" {
			trafficV4 = append(trafficV4, tc)
		}
	}

	By("echo servers are reachable before any health changes")
	for _, tc := range traffic {
		Eventually(func() error {
			return runShortEcho(tc.network, tc.localIP, tc.remote, "pre-check-"+tc.name)
		}, "5s", "200ms").Should(Succeed())
	}

	if len(trafficV4) > 0 {
		By("initially each IPv4 link can carry traffic individually")
		Expect(netRouter.setShootClientHealth(clientIPs[1], false, clientIPs)).To(Succeed())
		for _, tc := range trafficV4 {
			Eventually(func() error {
				return runShortEcho(tc.network, tc.localIP, tc.remote, "initial-link0-"+tc.name)
			}, "5s", "200ms").Should(Succeed())
		}
		Expect(netRouter.setShootClientHealth(clientIPs[1], true, clientIPs)).To(Succeed())
		Expect(netRouter.setShootClientHealth(clientIPs[0], false, clientIPs)).To(Succeed())
		for _, tc := range trafficV4 {
			Eventually(func() error {
				return runShortEcho(tc.network, tc.localIP, tc.remote, "initial-link1-"+tc.name)
			}, "5s", "200ms").Should(Succeed())
		}
		Expect(netRouter.setShootClientHealth(clientIPs[0], true, clientIPs)).To(Succeed())
	}

	By("an unhealthy link is redirected via the surviving client's routing table")
	router.reconcileRoutingPolicy(clientIPs[1], false, clientIPs)
	health, err := netRouter.getShootClientHealth(clientIPs)
	Expect(err).NotTo(HaveOccurred())
	Expect(health[clientIPs[0]]).To(BeTrue())
	Expect(health[clientIPs[1]]).To(BeFalse())

	By("verifying fwmark rule for unhealthy client now points to surviving client's table")
	{
		clientIndex1 := network.ClientIndexFromBondingShootClientIP(clientIPs[1].AsSlice())
		mark1 := constants.FwmarkBase + clientIndex1
		clientIndex0 := network.ClientIndexFromBondingShootClientIP(clientIPs[0].AsSlice())
		survivingTableID := constants.RoutingTableBaseID + clientIndex0
		for _, ipv6 := range []bool{false, true} {
			family := netlink.FAMILY_V4
			if ipv6 {
				family = netlink.FAMILY_V6
			}
			rules, err := netlink.RuleList(family)
			Expect(err).NotTo(HaveOccurred())
			found := false
			for _, r := range rules {
				if r.Mark == uint32(mark1) && r.Table == survivingTableID { //nolint:gosec
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "mark %d should be redirected to surviving table %d (ipv6=%v)", mark1, survivingTableID, ipv6)
		}
	}

	By("persistent connections are created on the remaining good link")
	tx0Before, tx1Before, _ := txPackets(link0, link1)
	conns := make([]net.Conn, 0, len(trafficV4))
	for _, tc := range trafficV4 {
		conn, err := dialWithRetry(tc.network, tc.localIP, tc.remote, 5*time.Second)
		Expect(err).NotTo(HaveOccurred())
		conns = append(conns, conn)
		Expect(writeReadEcho(conn, "persist-before-re-add-"+tc.name)).To(Succeed())
	}
	if len(trafficV4) > 0 {
		tx0After, tx1After, err := txPackets(link0, link1)
		Expect(err).NotTo(HaveOccurred())
		Expect(tx0After - tx0Before).To(BeNumerically(">", 0))
		Expect(tx1After).To(Equal(tx1Before))
	}

	By("re-adding the recovered link does not disturb existing persistent connections")
	router.reconcileRoutingPolicy(clientIPs[1], true, clientIPs)
	if len(conns) > 0 {
		tx0Before, tx1Before, err = txPackets(link0, link1)
		fmt.Fprintf(GinkgoWriter, "tx0Before: %d tx1Before: %d\n", tx0Before, tx1Before)
		Expect(err).NotTo(HaveOccurred())
		for i := 0; i < 20; i++ {
			for _, conn := range conns {
				Expect(writeReadEcho(conn, fmt.Sprintf("persist-after-re-add-%d", i))).To(Succeed())
			}
		}
		tx0After, tx1After, err := txPackets(link0, link1)
		Expect(err).NotTo(HaveOccurred())
		fmt.Fprintf(GinkgoWriter, "tx0After: %d tx1After: %d\n", tx0After, tx1After)
		Expect((tx0After - tx0Before) + (tx1After - tx1Before)).To(BeNumerically(">", 0))
		for _, conn := range conns {
			Expect(conn.Close()).To(Succeed())
		}
	}

	By("new IPv4 connections are distributed over both links and rebalance near 50/50")
	for _, tc := range trafficV4 {
		c0, c1, unknown, err := sampleConnectionDistribution(tc.network, tc.localIP, tc.remote, link0, link1, 160)
		Expect(err).NotTo(HaveOccurred())
		Expect(c0).To(BeNumerically(">", 0))
		Expect(c1).To(BeNumerically(">", 0))
		total := c0 + c1
		Expect(total).To(BeNumerically(">=", 100), "too many unclassified %s connections: %d", tc.name, unknown)
		ratio := float64(c0) / float64(total)
		fmt.Fprintf(GinkgoWriter, "ratio after rebalance:  %f\n", ratio)
		Expect(ratio).To(BeNumerically(">", 0.30))
		Expect(ratio).To(BeNumerically("<", 0.70))
	}
}

// runNonShootTrafficUnmarkedScenario verifies that SetMarkRules only marks connections whose
// destination is in the shoot network. Connections to other addresses must leave the socket mark
// at zero so they are unaffected by the fwmark ip rules. Covers both IPv4 and IPv6.
func runNonShootTrafficUnmarkedScenario() {
	const (
		shootNet4      = "10.253.0.0/24"
		shootEchoIP4   = "10.253.0.2"
		shootEchoAddr4 = shootEchoIP4 + ":29090"
		nonShootIP4    = "10.254.0.2"
		nonShootAddr4  = nonShootIP4 + ":29091"
		localIP4       = "10.253.0.1"

		shootNet6      = "fd00:253::/64"
		shootEchoIP6   = "fd00:253::2"
		shootEchoAddr6 = "[" + shootEchoIP6 + "]:29092"
		nonShootIP6    = "fd00:254::2"
		nonShootAddr6  = "[" + nonShootIP6 + "]:29093"
		localIP6       = "fd00:253::1"
	)

	clientNetIPs := []net.IP{
		network.BondingShootClientIP(network.ParseIPNetIgnoreError(constants.DefaultVPNNetwork.String()).ToIPNet(), 0),
		network.BondingShootClientIP(network.ParseIPNetIgnoreError(constants.DefaultVPNNetwork.String()).ToIPNet(), 1),
	}
	clientIPs, err := network.IPtoAddr(clientNetIPs)
	Expect(err).NotTo(HaveOccurred())
	link0 := network.BondIP6TunnelLinkName(0)
	link1 := network.BondIP6TunnelLinkName(1)

	cleanup := func() {
		for _, tableID := range []int{constants.RoutingTableBaseID, constants.RoutingTableBaseID + 1} {
			_ = runIP("route", "flush", "table", fmt.Sprintf("%d", tableID))
			_ = runIP("-6", "route", "flush", "table", fmt.Sprintf("%d", tableID))
		}
		for i := range 2 {
			mark := constants.FwmarkBase + i
			priority := constants.FwmarkRulePriorityBase + i
			deleteAnyFWmarkRule(mark, priority, false)
			deleteAnyFWmarkRule(mark, priority, true)
		}
		_ = exec.Command("iptables", "-t", "mangle", "-F", "OUTPUT").Run()
		_ = exec.Command("iptables", "-t", "mangle", "-F", constants.ShootMarkChain).Run()
		_ = exec.Command("iptables", "-t", "mangle", "-X", constants.ShootMarkChain).Run()
		_ = exec.Command("ip6tables", "-t", "mangle", "-F", "OUTPUT").Run()
		_ = exec.Command("ip6tables", "-t", "mangle", "-F", constants.ShootMarkChain).Run()
		_ = exec.Command("ip6tables", "-t", "mangle", "-X", constants.ShootMarkChain).Run()
		_ = runIP("addr", "del", localIP4+"/32", "dev", "lo")
		_ = runIP("addr", "del", shootEchoIP4+"/32", "dev", "lo")
		_ = runIP("addr", "del", nonShootIP4+"/32", "dev", "lo")
		_ = runIP("-6", "addr", "del", localIP6+"/128", "dev", "lo")
		_ = runIP("-6", "addr", "del", shootEchoIP6+"/128", "dev", "lo")
		_ = runIP("-6", "addr", "del", nonShootIP6+"/128", "dev", "lo")
		_ = runIP("-6", "route", "del", shootNet6, "dev", "lo")
		_ = network.DeleteLinkByName(link0)
		_ = network.DeleteLinkByName(link1)
	}
	cleanup()
	DeferCleanup(cleanup)

	// Add all IPs to loopback so connections stay in the host netns; iptables mangle OUTPUT fires
	// on locally-originated packets before routing, so SO_MARK reflects the iptables mark.
	Expect(runIP("addr", "add", localIP4+"/32", "dev", "lo")).To(Succeed())
	Expect(runIP("addr", "add", shootEchoIP4+"/32", "dev", "lo")).To(Succeed())
	Expect(runIP("addr", "add", nonShootIP4+"/32", "dev", "lo")).To(Succeed())
	Expect(runIP("-6", "addr", "add", localIP6+"/128", "dev", "lo")).To(Succeed())
	Expect(runIP("-6", "addr", "add", shootEchoIP6+"/128", "dev", "lo")).To(Succeed())
	Expect(runIP("-6", "addr", "add", nonShootIP6+"/128", "dev", "lo")).To(Succeed())

	// Dummy link devices so setupRouting can install per-client table routes.
	Expect(runIP("link", "add", link0, "type", "dummy")).To(Succeed())
	Expect(runIP("link", "add", link1, "type", "dummy")).To(Succeed())
	Expect(runIP("link", "set", link0, "up")).To(Succeed())
	Expect(runIP("link", "set", link1, "up")).To(Succeed())

	// Echo servers bound directly in the host netns on loopback addresses.
	stopShoot4, err := startEchoServer("tcp4", shootEchoAddr4)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(stopShoot4)
	stopNonShoot4, err := startEchoServer("tcp4", nonShootAddr4)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(stopNonShoot4)
	stopShoot6, err := startEchoServer("tcp6", shootEchoAddr6)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(stopShoot6)
	stopNonShoot6, err := startEchoServer("tcp6", nonShootAddr6)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(stopNonShoot6)

	netRouter := &netlinkRouter{
		shootPodNetworks: []network.CIDR{
			network.ParseIPNetIgnoreError(shootNet4),
			network.ParseIPNetIgnoreError(shootNet6),
		},
		seedPodNetwork: network.ParseIPNetIgnoreError("10.10.0.0/16"),
		log:            logr.Discard(),
	}
	Expect(netRouter.setupRouting(clientIPs)).To(Succeed())

	By("IPv4 shoot-destined connections receive a non-zero mark")
	Eventually(func() error {
		mark, err := dialAndGetMark("tcp4", localIP4, shootEchoAddr4)
		if err != nil {
			return err
		}
		if mark == 0 {
			return fmt.Errorf("expected non-zero mark for shoot-destined connection, got 0")
		}
		return nil
	}, "5s", "200ms").Should(Succeed())

	By("IPv4 non-shoot-destined connections receive mark zero")
	Consistently(func() error {
		mark, err := dialAndGetMark("tcp4", localIP4, nonShootAddr4)
		if err != nil {
			return err
		}
		if mark != 0 {
			return fmt.Errorf("expected mark 0 for non-shoot connection, got %d", mark)
		}
		return nil
	}, "1s", "100ms").Should(Succeed())

	By("IPv6 shoot-destined connections receive a non-zero mark")
	// IPv6 conntrack requires nf_conntrack_ipv6 which is absent in Docker Desktop on Mac.
	// Skip the mark check if conntrack doesn't track IPv6; the ip6tables rules and loopback
	// routes are already verified by the structural assertions in runPolicyRoutingScenario.
	if ipv6ConntrackAvailable() {
		Eventually(func() error {
			mark, err := dialAndGetMark("tcp6", localIP6, shootEchoAddr6)
			if err != nil {
				return err
			}
			if mark == 0 {
				return fmt.Errorf("expected non-zero mark for IPv6 shoot-destined connection, got 0")
			}
			return nil
		}, "5s", "200ms").Should(Succeed())

		By("IPv6 non-shoot-destined connections receive mark zero")
		Consistently(func() error {
			mark, err := dialAndGetMark("tcp6", localIP6, nonShootAddr6)
			if err != nil {
				return err
			}
			if mark != 0 {
				return fmt.Errorf("expected mark 0 for IPv6 non-shoot connection, got %d", mark)
			}
			return nil
		}, "1s", "100ms").Should(Succeed())
	} else {
		fmt.Fprintln(GinkgoWriter, "skipping IPv6 conntrack mark check: nf_conntrack_ipv6 unavailable")
	}
}

// startEchoServer starts a TCP echo server bound in the current network namespace.
func startEchoServer(networkName, addr string) (func(), error) {
	ln, err := net.Listen(networkName, addr)
	if err != nil {
		return nil, err
	}
	var closed atomic.Bool
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if closed.Load() {
					return
				}
				continue
			}
			go func(c net.Conn) {
				defer c.Close()
				reader := bufio.NewReader(c)
				for {
					_ = c.SetDeadline(time.Now().Add(3 * time.Second))
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					if _, err := io.WriteString(c, line); err != nil {
						return
					}
				}
			}(conn)
		}
	}()
	return func() {
		closed.Store(true)
		_ = ln.Close()
	}, nil
}

// dialAndGetMark opens a TCP connection from localIP to remoteAddr, sends one echo exchange,
// and returns the conntrack mark set by iptables mangle OUTPUT CONNMARK --save-mark.
// It reads the mark from /proc/net/nf_conntrack while the connection is still open.
func dialAndGetMark(network_, localIP, remoteAddr string) (int, error) {
	ip := net.ParseIP(localIP)
	var localAddr net.Addr
	if ip.To4() != nil {
		localAddr = &net.TCPAddr{IP: ip.To4()}
	} else {
		localAddr = &net.TCPAddr{IP: ip}
	}
	d := net.Dialer{
		LocalAddr: localAddr,
		Timeout:   3 * time.Second,
	}
	conn, err := d.Dial(network_, remoteAddr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	// Send one exchange so mangle OUTPUT has fired and CONNMARK --save-mark has run.
	if err := writeReadEcho(conn, "mark-probe"); err != nil {
		return 0, err
	}

	// Determine the local port to find the right conntrack entry.
	localPort := conn.LocalAddr().(*net.TCPAddr).Port

	data, err := os.ReadFile("/proc/net/nf_conntrack")
	if err != nil {
		return 0, fmt.Errorf("reading nf_conntrack: %w", err)
	}
	// Match on all four tuple fields to avoid collisions with TIME_WAIT entries from other tests.
	remoteHost, remotePort, _ := net.SplitHostPort(remoteAddr)
	srcMatch := fmt.Sprintf("src=%s", localIP)
	dstMatch := fmt.Sprintf("dst=%s", remoteHost)
	sportMatch := fmt.Sprintf("sport=%d", localPort)
	dportMatch := fmt.Sprintf("dport=%s", remotePort)
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, srcMatch) ||
			!strings.Contains(line, dstMatch) ||
			!strings.Contains(line, sportMatch) ||
			!strings.Contains(line, dportMatch) {
			continue
		}
		// Parse "mark=N" from the conntrack line.
		for _, field := range strings.Fields(line) {
			if strings.HasPrefix(field, "mark=") {
				var mark int
				if _, err := fmt.Sscanf(field, "mark=%d", &mark); err == nil {
					return mark, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("conntrack entry not found for %s->%s sport=%d", localIP, remoteAddr, localPort)
}

func runFwmarkSwitchUnderLoadScenario() {
	suffix := fmt.Sprintf("%04x", scenarioSeq.Add(1))
	nsName := "pcfws-" + suffix
	vethHost := "fwvh" + suffix
	vethPeer := "fwvp" + suffix
	rt0 := "fwr0" + suffix
	rt1 := "fwr1" + suffix

	clientNetIPs := []net.IP{
		network.BondingShootClientIP(network.ParseIPNetIgnoreError(constants.DefaultVPNNetwork.String()).ToIPNet(), 0),
		network.BondingShootClientIP(network.ParseIPNetIgnoreError(constants.DefaultVPNNetwork.String()).ToIPNet(), 1),
	}
	clientIPs, err := network.IPtoAddr(clientNetIPs)
	Expect(err).NotTo(HaveOccurred())
	link0 := network.BondIP6TunnelLinkName(0)
	link1 := network.BondIP6TunnelLinkName(1)

	const (
		localAddr  = "10.252.0.1"
		remoteAddr = "10.252.0.2"
		echoAddr   = remoteAddr + ":29080"
	)

	cleanup := func() {
		for _, tableID := range []int{constants.RoutingTableBaseID, constants.RoutingTableBaseID + 1} {
			_ = runIP("route", "flush", "table", fmt.Sprintf("%d", tableID))
		}
		for i := range 2 {
			mark := constants.FwmarkBase + i
			priority := constants.FwmarkRulePriorityBase + i
			deleteAnyFWmarkRule(mark, priority, false)
			deleteAnyFWmarkRule(mark, priority, true)
		}
		_ = exec.Command("iptables", "-t", "mangle", "-F", "OUTPUT").Run()
		_ = exec.Command("ip6tables", "-t", "mangle", "-F", "OUTPUT").Run()
		_ = runIP("addr", "del", localAddr+"/32", "dev", "lo")
		_ = network.DeleteLinkByName(link0)
		_ = network.DeleteLinkByName(link1)
		_ = runIP("link", "del", vethHost)
		_ = runIP("netns", "del", nsName)
	}
	cleanup()
	DeferCleanup(cleanup)

	Expect(runIP("netns", "add", nsName)).To(Succeed())
	Expect(runIP("link", "add", vethHost, "type", "veth", "peer", "name", vethPeer)).To(Succeed())
	Expect(runIP("link", "set", vethPeer, "netns", nsName)).To(Succeed())

	Expect(runIP("addr", "add", "2001:db8:2::101/64", "dev", vethHost)).To(Succeed())
	Expect(runIP("addr", "add", "2001:db8:2::102/64", "dev", vethHost)).To(Succeed())
	Expect(runIP("link", "set", vethHost, "up")).To(Succeed())

	Expect(runIP("-n", nsName, "link", "set", "lo", "up")).To(Succeed())
	Expect(runIP("-n", nsName, "addr", "add", "2001:db8:2::201/64", "dev", vethPeer)).To(Succeed())
	Expect(runIP("-n", nsName, "addr", "add", "2001:db8:2::202/64", "dev", vethPeer)).To(Succeed())
	Expect(runIP("-n", nsName, "link", "set", vethPeer, "up")).To(Succeed())

	Expect(runIP("-6", "tunnel", "add", link0, "mode", "any", "local", "2001:db8:2::101", "remote", "2001:db8:2::201", "dev", vethHost)).To(Succeed())
	Expect(runIP("-6", "tunnel", "add", link1, "mode", "any", "local", "2001:db8:2::102", "remote", "2001:db8:2::202", "dev", vethHost)).To(Succeed())
	Expect(runIP("link", "set", link0, "up")).To(Succeed())
	Expect(runIP("link", "set", link1, "up")).To(Succeed())

	Expect(runIP("-n", nsName, "-6", "tunnel", "add", rt0, "mode", "any", "local", "2001:db8:2::201", "remote", "2001:db8:2::101", "dev", vethPeer)).To(Succeed())
	Expect(runIP("-n", nsName, "-6", "tunnel", "add", rt1, "mode", "any", "local", "2001:db8:2::202", "remote", "2001:db8:2::102", "dev", vethPeer)).To(Succeed())
	Expect(runIP("-n", nsName, "link", "set", rt0, "up")).To(Succeed())
	Expect(runIP("-n", nsName, "link", "set", rt1, "up")).To(Succeed())

	Expect(disableRPFilter(vethHost)).To(Succeed())
	Expect(runInNetns(nsName, "sh", "-c",
		"echo 0 > /proc/sys/net/ipv4/conf/all/rp_filter; "+
			"echo 0 > /proc/sys/net/ipv4/conf/default/rp_filter; "+
			"echo 0 > /proc/sys/net/ipv4/conf/lo/rp_filter; "+
			"echo 0 > /proc/sys/net/ipv4/conf/"+rt0+"/rp_filter; "+
			"echo 0 > /proc/sys/net/ipv4/conf/"+rt1+"/rp_filter; "+
			"echo 0 > /proc/sys/net/ipv4/conf/"+vethPeer+"/rp_filter")).To(Succeed())

	Expect(runIP("-n", nsName, "addr", "add", remoteAddr+"/32", "dev", "lo")).To(Succeed())
	Expect(runIP("addr", "add", localAddr+"/32", "dev", "lo")).To(Succeed())
	Expect(runIP("-n", nsName, "route", "replace", localAddr+"/32", "nexthop", "dev", rt0, "nexthop", "dev", rt1)).To(Succeed())

	stop, err := startNamespaceEchoServer(nsName, "tcp4", echoAddr)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(stop)

	netRouter := &netlinkRouter{
		shootPodNetworks: []network.CIDR{network.ParseIPNetIgnoreError(remoteAddr + "/32")},
		seedPodNetwork:   network.ParseIPNetIgnoreError("10.10.0.0/16"),
		log:              logr.Discard(),
	}
	Expect(netRouter.setupRouting(clientIPs)).To(Succeed())

	By("echo server is reachable before switching")
	Eventually(func() error {
		return runShortEcho("tcp4", localAddr, echoAddr, "pre-switch")
	}, "5s", "200ms").Should(Succeed())

	By("new connections all succeed while fwmark rules are continuously switched")
	// The switcher goroutine alternates the health state of client 1 as fast as possible,
	// which causes setShootClientHealth to add the new rule then delete the old one on every
	// iteration. The loader goroutines each open a new connection on every iteration.
	// No connection attempt should fail due to a window where no matching rule exists.
	const (
		switchDuration = 3 * time.Second
		connRate       = 500 // connections per second across all loaders
		numLoaders     = 8
	)

	ctx, cancel := context.WithTimeout(context.Background(), switchDuration)
	defer cancel()

	// switcher: flip health of client1 back and forth as fast as possible
	var switchCount atomic.Int64
	go func() {
		healthy := false
		for ctx.Err() == nil {
			_ = netRouter.setShootClientHealth(clientIPs[1], healthy, clientIPs)
			healthy = !healthy
			switchCount.Add(1)
		}
		// Leave both clients healthy at the end so cleanup rules are in their canonical state.
		_ = netRouter.setShootClientHealth(clientIPs[1], true, clientIPs)
	}()

	// loaders: each goroutine opens a new connection and does one echo on every iteration.
	var (
		totalAttempts atomic.Int64
		totalFailures atomic.Int64
		wg            sync.WaitGroup
	)
	for range numLoaders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// limit conn/s per goroutine — avoid conntrack misclassifying SYNs
			// as INVALID under extreme load, which leaves mark=0 and fails routing.
			ticker := time.NewTicker(time.Second / time.Duration(connRate/numLoaders))
			defer ticker.Stop()
			for i := 0; ctx.Err() == nil; i++ {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
				totalAttempts.Add(1)
				if err := runShortEcho("tcp4", localAddr, echoAddr, fmt.Sprintf("load-%d", i)); err != nil {
					totalFailures.Add(1)
					fmt.Fprintf(GinkgoWriter, "connection failed: %v\n", err)
				}
			}
		}()
	}
	wg.Wait()

	fmt.Fprintf(GinkgoWriter, "switches: %d, attempts: %d, failures: %d\n",
		switchCount.Load(), totalAttempts.Load(), totalFailures.Load())
	Expect(totalAttempts.Load()).To(BeNumerically(">", 0))
	Expect(totalFailures.Load()).To(Equal(int64(0)))
}

func runIP(args ...string) error {
	cmd := exec.Command("ip", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runIPOut(args ...string) (string, error) {
	cmd := exec.Command("ip", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("ip %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func runInNetns(nsName string, cmd string, args ...string) error {
	fullArgs := append([]string{"netns", "exec", nsName, cmd}, args...)
	ipCmd := exec.Command("ip", fullArgs...)
	out, err := ipCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip %s failed: %w: %s", strings.Join(fullArgs, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func disableRPFilter(vethHost string) error {
	paths := []string{
		"/proc/sys/net/ipv4/conf/all/rp_filter",
		"/proc/sys/net/ipv4/conf/default/rp_filter",
		"/proc/sys/net/ipv4/conf/lo/rp_filter",
		"/proc/sys/net/ipv4/conf/" + vethHost + "/rp_filter",
		"/proc/sys/net/ipv4/conf/bond0-ip6tnl0/rp_filter",
		"/proc/sys/net/ipv4/conf/bond0-ip6tnl1/rp_filter",
	}
	for _, p := range paths {
		if err := os.WriteFile(p, []byte("0"), 0o644); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("failed to set %s: %w", p, err)
		}
	}
	return nil
}

func startNamespaceEchoServer(nsName, networkName, addr string) (func(), error) {
	targetNS, err := netns.GetFromName(nsName)
	if err != nil {
		return nil, err
	}

	ready := make(chan error, 1)
	listenerCh := make(chan net.Listener, 1)
	var closed atomic.Bool

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		origNS, err := netns.Get()
		if err != nil {
			ready <- err
			return
		}
		defer origNS.Close()

		if err := netns.Set(targetNS); err != nil {
			ready <- err
			return
		}
		defer func() {
			_ = netns.Set(origNS)
		}()

		ln, err := net.Listen(networkName, addr)
		if err != nil {
			ready <- err
			return
		}
		listenerCh <- ln
		ready <- nil

		for {
			conn, err := ln.Accept()
			if err != nil {
				if closed.Load() {
					return
				}
				continue
			}
			go func(c net.Conn) {
				defer c.Close()
				reader := bufio.NewReader(c)
				for {
					_ = c.SetDeadline(time.Now().Add(3 * time.Second))
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					if _, err := io.WriteString(c, line); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	if err := <-ready; err != nil {
		targetNS.Close()
		return nil, err
	}
	ln := <-listenerCh

	return func() {
		closed.Store(true)
		_ = ln.Close()
		_ = targetNS.Close()
	}, nil
}

func dialWithLocalSource(networkName, localIP, remoteAddr string) (net.Conn, error) {
	d := net.Dialer{Timeout: 3 * time.Second}
	switch networkName {
	case "tcp4":
		d.LocalAddr = &net.TCPAddr{IP: net.ParseIP(localIP).To4()}
	case "tcp6":
		d.LocalAddr = &net.TCPAddr{IP: net.ParseIP(localIP)}
	default:
		return nil, fmt.Errorf("unsupported network %s", networkName)
	}
	return d.Dial(networkName, remoteAddr)
}

func dialWithRetry(networkName, localIP, remoteAddr string, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := dialWithLocalSource(networkName, localIP, remoteAddr)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("dial timeout reached")
	}
	return nil, lastErr
}

func writeReadEcho(conn net.Conn, payload string) error {
	msg := payload + "\n"
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return err
	}
	if _, err := io.WriteString(conn, msg); err != nil {
		return err
	}
	buf := make([]byte, len(msg))
	if _, err := io.ReadFull(conn, buf); err != nil {
		return err
	}
	if string(buf) != msg {
		return fmt.Errorf("echo mismatch: got %q want %q", string(buf), msg)
	}
	return nil
}

func runShortEcho(networkName, localIP, remoteAddr, payload string) error {
	conn, err := dialWithLocalSource(networkName, localIP, remoteAddr)
	if err != nil {
		return err
	}
	defer conn.Close()
	return writeReadEcho(conn, payload)
}

// deleteAnyFWmarkRule removes the first ip rule matching mark+priority regardless of table.
// Used only in test cleanup where the current table is unknown.
func deleteAnyFWmarkRule(mark, priority int, ipv6 bool) {
	family := netlink.FAMILY_V4
	if ipv6 {
		family = netlink.FAMILY_V6
	}
	rules, _ := netlink.RuleList(family)
	for _, r := range rules {
		if r.Mark == uint32(mark) && r.Priority == priority { //nolint:gosec
			_ = netlink.RuleDel(&r)
			return
		}
	}
}

func txPackets(link0, link1 string) (uint64, uint64, error) {
	l0, err := netlink.LinkByName(link0)
	if err != nil {
		return 0, 0, err
	}
	l1, err := netlink.LinkByName(link1)
	if err != nil {
		return 0, 0, err
	}
	if l0.Attrs().Statistics == nil || l1.Attrs().Statistics == nil {
		return 0, 0, errors.New("missing link statistics")
	}
	return l0.Attrs().Statistics.TxPackets, l1.Attrs().Statistics.TxPackets, nil
}

func sampleConnectionDistribution(networkName, localIP, remoteAddr, link0, link1 string, samples int) (count0, count1, unknown int, err error) {
	for i := 0; i < samples; i++ {
		before0, before1, e := txPackets(link0, link1)
		if e != nil {
			return 0, 0, 0, e
		}
		if e = runShortEcho(networkName, localIP, remoteAddr, fmt.Sprintf("probe-%s-%d", networkName, i)); e != nil {
			return 0, 0, 0, e
		}
		time.Sleep(10 * time.Millisecond)
		after0, after1, e := txPackets(link0, link1)
		if e != nil {
			return 0, 0, 0, e
		}
		d0 := int64(after0 - before0)
		d1 := int64(after1 - before1)
		switch {
		case d0 > d1 && d0 > 0:
			count0++
		case d1 > d0 && d1 > 0:
			count1++
		case d0 == d1 && d0 > 0:
			// If both links changed equally for a sample, it is ambiguous and excluded.
			unknown++
		default:
			unknown++
		}
	}
	if total := count0 + count1; total > 0 {
		if math.IsNaN(float64(count0) / float64(total)) {
			return 0, 0, 0, fmt.Errorf("invalid distribution result")
		}
	}
	return count0, count1, unknown, nil
}

// ipv6ConntrackAvailable reports whether the kernel tracks IPv6 connections in nf_conntrack.
// Docker Desktop on Mac runs a Linux VM whose kernel omits nf_conntrack_ipv6, so IPv6 entries
// never appear in /proc/net/nf_conntrack. We probe by opening a real IPv6 connection to ::1.
func ipv6ConntrackAvailable() bool {
	ln, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		return false
	}
	defer ln.Close()
	conn, err := net.DialTimeout("tcp6", ln.Addr().String(), time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	data, err := os.ReadFile("/proc/net/nf_conntrack")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "src=::1")
}
