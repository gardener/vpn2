// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pathcontroller

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gardener/vpn2/pkg/network"
	"github.com/go-logr/logr"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

type icmpPinger struct {
	log     logr.Logger
	timeout time.Duration
	retries int
}

func (p icmpPinger) Ping(client net.IP) error {
	var err error
	for i := 0; i < 1+p.retries; i++ {
		err = p.ping(client)
		if err == nil {
			break
		}
		if i == 0 {
			go func() {
				// send neighbor solicitation to speed up discovery the link-layer address of a neighbor
				p.log.Info("sending neighbor solicitation", "ip", client.String())
				err := p.neighborSolicitation(client)
				if err != nil {
					p.log.Info("neighbor solicitation failed", "error", err.Error())
				} else {
					p.log.Info("received neighbor advertisement")
				}
			}()
		}
	}
	return err
}

func (p icmpPinger) ping(client net.IP) error {
	timer := time.Now()
	defer func() {
		if d := time.Since(timer); d > 100*time.Millisecond {
			p.log.Info("ping to client took more than 100ms", "ip", client, "duration", fmt.Sprintf("%dms", d.Milliseconds()))
		}
	}()

	c, err := icmp.ListenPacket("udp6", "")
	if err != nil {
		return fmt.Errorf("error listening for packets: %w", err)
	}
	defer c.Close()

	deadline := time.Now().Add(p.timeout)
	err = c.SetReadDeadline(deadline)
	if err != nil {
		return fmt.Errorf("error setting deadline: %w", err)
	}

	msg := icmp.Message{
		Type: ipv6.ICMPTypeEchoRequest,
		Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  1,
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
	rm, err := icmp.ParseMessage(ipv6.ICMPTypeEchoReply.Protocol(), rb[:n])
	if err != nil {
		return fmt.Errorf("error parsing response: %w", err)
	}
	switch rm.Type {
	case ipv6.ICMPTypeEchoReply:
		return nil
	default:
		return err
	}
}

func (p icmpPinger) neighborSolicitation(client net.IP) error {
	if len(client) != net.IPv6len {
		return fmt.Errorf("only usable with ipv6")
	}
	ifi, err := net.InterfaceByName("bond0")
	if err != nil {
		return fmt.Errorf("bonding device not found: %w", err)
	}
	ips, err := network.GetLinkIPAddressesByName("bond0", network.ScopeLink)
	if err != nil {
		return fmt.Errorf("getting link IP address failed: %w", err)
	}
	if len(ips) != 1 {
		return fmt.Errorf("link IP address not unique: %d", len(ips))
	}

	ns := icmp.Message{
		Type: ipv6.ICMPTypeNeighborSolicitation,
		Code: 0,
		Body: &icmp.RawBody{
			Data: buildNSBody(client, ifi.HardwareAddr),
		},
	}

	data, err := ns.Marshal(nil)
	if err != nil {
		return fmt.Errorf("error marshalling ICMP message: %w", err)
	}

	conn, err := icmp.ListenPacket("ip6:ipv6-icmp", ips[0].String()+"%bond0")
	if err != nil {
		return fmt.Errorf("error opening raw socket: %w", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(p.timeout / 2)
	pc := conn.IPv6PacketConn()
	if err := pc.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("error setting deadline: %w", err)
	}
	if err := pc.SetHopLimit(255); err != nil {
		return fmt.Errorf("error setting hop limit: %w", err)
	}
	if err := pc.SetMulticastHopLimit(255); err != nil {
		return fmt.Errorf("error setting hop limit: %w", err)
	}

	// calculate solicited-node multicase address
	destIP := net.ParseIP("ff02::1:ff00:0")
	copy(destIP[13:], client[13:]) // copy last 24 bits
	dst := &net.IPAddr{IP: destIP}
	_, err = pc.WriteTo(data, nil, dst)
	if err != nil {
		return fmt.Errorf("error sending ICMP message: %w", err)
	}

	// Listen for Neighbor Advertisement response
	reply := make([]byte, 1500)
	n, _, _, err := pc.ReadFrom(reply)
	if err != nil {
		if strings.Contains(err.Error(), "i/o timeout") {
			return fmt.Errorf("i/o timeout")
		}
		return fmt.Errorf("error reading from socket: %w", err)
	}

	rm, err := icmp.ParseMessage(ipv6.ICMPTypeNeighborAdvertisement.Protocol(), reply[:n])
	if err != nil {
		return fmt.Errorf("error parsing ICMP message: %w", err)
	}

	switch rm.Type {
	case ipv6.ICMPTypeNeighborAdvertisement:
		return nil
	default:
		return fmt.Errorf("received unexpected ICMP message: %#v", rm)
	}
}

func buildNSBody(target net.IP, hwAddr net.HardwareAddr) []byte {
	// ICMPv6 Neighbor Solicitation message body format:
	//  0                   1                   2                   3
	//  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |                           Reserved                            |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |                                                               |
	// +                                                               +
	// |                                                               |
	// +                       Target Address                          +
	// |                                                               |
	// +                                                               +
	// |                                                               |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |   Type (1)    |    Length     |             Data              |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

	buf := make([]byte, 22+len(hwAddr))
	// Reserved (4 bytes)
	copy(buf[0:4], make([]byte, 4))

	// Target Address (16 bytes)
	copy(buf[4:20], target)

	// Option: Source Link-Layer Address
	buf[20] = 1 // Type (Source Link-Layer Address)
	buf[21] = 1 // Length (in units of 8 octets)
	copy(buf[22:], hwAddr)

	return buf
}
