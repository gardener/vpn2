// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pathcontroller

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/gardener/vpn2/pkg/config"

	"github.com/gardener/vpn2/pkg/network"
	"github.com/gardener/vpn2/pkg/utils"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const Name = "path-controller"

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   Name,
		Short: Name,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			log, err := utils.InitRun(cmd, Name)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithCancel(cmd.Context())
			return run(ctx, cancel, log)
		},
	}

	return cmd
}

func run(ctx context.Context, _ context.CancelFunc, log logr.Logger) error {
	cfg, err := config.GetPathControllerConfig(log)
	if err != nil {
		return err
	}

	if err = network.ValidateCIDR(cfg.VPNNetwork, cfg.IPFamilies); err != nil {
		return err
	}
	checkNetwork := cfg.NodeNetwork
	if checkNetwork.String() == "" {
		checkNetwork = cfg.ServiceNetwork
	}
	if checkNetwork.String() == "" {
		return errors.New("network to check is undefined")
	}

	router := &ClientRouter{
		cfg:        cfg,
		checkedNet: checkNetwork.ToIPNet(),
		goodIPs:    make(map[string]struct{}),
		log:        log.WithName("pingRouter"),
	}

	// acquired ip is not necessary here, because we don't care about the subnet
	_, clientIPs := network.GetBondAddressAndTargetsSeedClient(nil, cfg.VPNNetwork.ToIPNet(), cfg.HAVPNClients)
	ticker := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			router.pingAllShootClients(clientIPs)
			_, ok := router.goodIPs[router.current.String()]
			if !ok {
				newIP, err := router.selectNewShootClient()
				if err != nil {
					// not error here to retry because in creation there will be some time when nothing is
					// available. If we errored we block the creation process.
					log.Error(err, "error selecting a new shoot client")
					continue
				}
				err = router.updateRouting(newIP)
				if err != nil {
					return err
				}
				router.current = newIP
			}
		}
	}
}

func routeForNetwork(net *net.IPNet, newIP net.IP, bondLink netlink.Link) netlink.Route {
	// ip route replace $net via $newIp dev bond0
	return netlink.Route{
		// Gw is the equivalent to via in ip route replace command
		Gw:  newIP,
		Dst: net,
		//Table:     unix.RT_TABLE_MAIN,
		LinkIndex: bondLink.Attrs().Index,
	}
}

type ClientRouter struct {
	cfg config.PathController

	log        logr.Logger
	checkedNet *net.IPNet
	current    net.IP
	mu         sync.Mutex
	goodIPs    map[string]struct{}
}

func (p *ClientRouter) selectNewShootClient() (net.IP, error) {
	// just use the first ip that is in goodIps map
	for ip := range p.goodIPs {
		return net.ParseIP(ip), nil
	}
	return nil, errors.New("no more good ips in pool")
}

func (p *ClientRouter) updateRouting(newIP net.IP) error {
	bondDev, err := netlink.LinkByName("bond0")
	if err != nil {
		return err
	}

	nets := []*net.IPNet{
		p.cfg.ServiceNetwork.ToIPNet(),
		p.cfg.PodNetwork.ToIPNet(),
	}
	if p.cfg.NodeNetwork.String() != "" {
		nets = append(nets, p.cfg.NodeNetwork.ToIPNet())
	}

	for _, n := range nets {
		route := routeForNetwork(n, newIP, bondDev)
		p.log.Info("replacing route", "route", route, "net", n)
		err = netlink.RouteReplace(&route)
		if err != nil {
			return fmt.Errorf("error replacing route for %s: %w", n, err)
		}
	}
	return nil
}

func (p *ClientRouter) pingAllShootClients(clients []net.IP) {
	var wg sync.WaitGroup
	for _, client := range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := pingClient(client)
			p.mu.Lock()
			defer p.mu.Unlock()
			if err != nil {
				p.log.Info("client not healthy, removing from pool", "ip", client)
				delete(p.goodIPs, client.String())
			} else {
				p.goodIPs[client.String()] = struct{}{}
			}
		}()
	}
	wg.Wait()
}

const protocolICMP = 1

func pingClient(client net.IP) error {
	c, err := icmp.ListenPacket("udp4", "")
	if err != nil {
		return fmt.Errorf("error listening for packets: %w", err)
	}
	defer c.Close()

	deadline := time.Now().Add(2 * time.Second)
	err = c.SetReadDeadline(deadline)
	if err != nil {
		return fmt.Errorf("error setting deadline: %w", err)

	}

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: os.Getpid() & 0xffff, Seq: 1,
			Data: []byte("HELLO-R-U-THERE"),
		},
	}
	marshaledMsg, err := msg.Marshal(nil)
	if err != nil {
		return fmt.Errorf("error marshaling msg: %w", err)

	}
	if _, err := c.WriteTo(marshaledMsg, &net.UDPAddr{IP: client}); err != nil {
		return fmt.Errorf("error writing to client: %w", err)

	}

	rb := make([]byte, 1500)
	n, _, err := c.ReadFrom(rb)
	if err != nil {
		return fmt.Errorf("error reading response: %w", err)

	}
	rm, err := icmp.ParseMessage(protocolICMP, rb[:n])
	if err != nil {
		return fmt.Errorf("error parsing response: %w", err)
	}
	switch rm.Type {
	case ipv4.ICMPTypeEchoReply:
		return nil
	default:
		return err
	}
}
