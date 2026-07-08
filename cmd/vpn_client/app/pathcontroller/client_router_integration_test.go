// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package pathcontroller

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
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

var _ = Describe("ClientRouter resilient nexthop integration", Serial, func() {
	It("shoot is IPv4 only", func() {
		runResilientNexthopScenario(true, false)
	})

	It("shoot is IPv6 only", func() {
		runResilientNexthopScenario(false, true)
	})

	It("shoot is dual-stack IPv4 + IPv6", func() {
		runResilientNexthopScenario(true, true)
	})
})

type trafficCase struct {
	name    string
	network string
	localIP string
	remote  string
}

func runResilientNexthopScenario(enableIPv4, enableIPv6 bool) {
	suffix := fmt.Sprintf("%04x", scenarioSeq.Add(1))
	nsName := "pcint-" + suffix
	vethHost := "pcvh" + suffix
	vethPeer := "pcvp" + suffix
	rt0 := "rt0" + suffix
	rt1 := "rt1" + suffix

	clientIPs := []net.IP{
		network.BondingShootClientIP(network.ParseIPNetIgnoreError(constants.DefaultVPNNetwork.String()).ToIPNet(), 0),
		network.BondingShootClientIP(network.ParseIPNetIgnoreError(constants.DefaultVPNNetwork.String()).ToIPNet(), 1),
	}
	link0 := network.BondIP6TunnelLinkName(0)
	link1 := network.BondIP6TunnelLinkName(1)

	cleanup := func() {
		_ = runIP("route", "del", shootPodNetworkV4)
		_ = runIP("-6", "route", "del", shootPodNetworkV6)
		_ = runIP("nexthop", "del", "id", fmt.Sprintf("%d", constants.NexthopGroupIDforIPv4))
		_ = runIP("nexthop", "del", "id", fmt.Sprintf("%d", constants.NexthopGroupIDforIPv6))
		_ = runIP("nexthop", "del", "id", fmt.Sprintf("%d", constants.NexthopDeviceBaseIDforIPv4+0))
		_ = runIP("nexthop", "del", "id", fmt.Sprintf("%d", constants.NexthopDeviceBaseIDforIPv4+1))
		_ = runIP("nexthop", "del", "id", fmt.Sprintf("%d", constants.NexthopDeviceBaseIDforIPv6+0))
		_ = runIP("nexthop", "del", "id", fmt.Sprintf("%d", constants.NexthopDeviceBaseIDforIPv6+1))
		_ = runIP("addr", "del", seedSrc4Addr+"/32", "dev", "lo")
		_ = runIP("-6", "addr", "del", seedSrc6Addr+"/128", "dev", "lo")
		_ = network.DeleteLinkByName(link0)
		_ = network.DeleteLinkByName(link1)
		_ = runIP("link", "del", vethHost)
		_ = runIP("netns", "del", nsName)
	}
	// Shared tunnel/nexthop objects can survive failed runs; clear them before setup.
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
		Expect(runIP("-n", nsName, "-6", "route", "replace", seedSrc6Addr+"/128", "dev", rt0)).To(Succeed())
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
	Expect(enableL4ECMPHashing()).To(Succeed())

	router := &clientRouter{netRouter: netRouter, log: logr.Discard()}

	By("initially both links can carry traffic")
	Expect(netRouter.setNexthopMember(clientIPs[1], false)).To(Succeed())
	for _, tc := range traffic {
		Eventually(func() error {
			return runShortEcho(tc.network, tc.localIP, tc.remote, "initial-link0-"+tc.name)
		}, "5s", "200ms").Should(Succeed())
	}
	Expect(netRouter.setNexthopMember(clientIPs[1], true)).To(Succeed())
	Expect(netRouter.setNexthopMember(clientIPs[0], false)).To(Succeed())
	for _, tc := range traffic {
		Eventually(func() error {
			return runShortEcho(tc.network, tc.localIP, tc.remote, "initial-link1-"+tc.name)
		}, "5s", "200ms").Should(Succeed())
	}
	Expect(netRouter.setNexthopMember(clientIPs[0], true)).To(Succeed())

	By("a troubled link is removed from the resilient group")
	router.reconcileNexthopGroup(clientIPs[1], false, clientIPs)
	members, err := netRouter.getNexthopGroupMembers(clientIPs)
	Expect(err).NotTo(HaveOccurred())
	Expect(members[clientIPs[0].String()]).To(BeTrue())
	Expect(members[clientIPs[1].String()]).To(BeFalse())

	By("persistent connections are created on the remaining good link")
	tx0Before, tx1Before, _ := txPackets(link0, link1)
	conns := make([]net.Conn, 0, len(traffic))
	for _, tc := range traffic {
		conn, err := dialWithRetry(tc.network, tc.localIP, tc.remote, 5*time.Second)
		Expect(err).NotTo(HaveOccurred())
		conns = append(conns, conn)
		Expect(writeReadEcho(conn, "persist-before-re-add-"+tc.name)).To(Succeed())
	}
	tx0After, tx1After, err := txPackets(link0, link1)
	Expect(err).NotTo(HaveOccurred())
	Expect(tx0After - tx0Before).To(BeNumerically(">", 0))
	Expect(tx1After).To(Equal(tx1Before))

	By("re-adding the recovered link does not disturb existing persistent connections")
	tx0Before, tx1Before, err = txPackets(link0, link1)
	fmt.Fprintf(GinkgoWriter, "tx0Before: %d tx1Before: %d\n", tx0Before, tx1Before)
	Expect(err).NotTo(HaveOccurred())
	router.reconcileNexthopGroup(clientIPs[1], true, clientIPs)
	for i := 0; i < 20; i++ {
		for _, conn := range conns {
			Expect(writeReadEcho(conn, fmt.Sprintf("persist-after-re-add-%d", i))).To(Succeed())
		}
	}
	tx0After, tx1After, err = txPackets(link0, link1)
	Expect(err).NotTo(HaveOccurred())
	fmt.Fprintf(GinkgoWriter, "tx0After: %d tx1After: %d\n", tx0After, tx1After)
	Expect((tx0After - tx0Before) + (tx1After - tx1Before)).To(BeNumerically(">", 0))
	for _, conn := range conns {
		Expect(conn.Close()).To(Succeed())
	}

	By("new connections are distributed over both links and rebalance near 50/50")
	for _, tc := range traffic {
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

func enableL4ECMPHashing() error {
	if err := os.WriteFile("/proc/sys/net/ipv4/fib_multipath_hash_policy", []byte(constants.ECMPHashPolicyL4), 0o644); err != nil {
		return fmt.Errorf("failed to set IPv4 fib_multipath_hash_policy: %w", err)
	}
	if err := os.WriteFile("/proc/sys/net/ipv6/fib_multipath_hash_policy", []byte(constants.ECMPHashPolicyL4), 0o644); err != nil {
		return fmt.Errorf("failed to set IPv6 fib_multipath_hash_policy: %w", err)
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
