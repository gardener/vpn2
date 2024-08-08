// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pathcontroller

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/go-logr/logr"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

type icmpPinger struct {
	log     logr.Logger
	timeout time.Duration
	retries int
	lastSeq atomic.Int32
}

const echoPayload = "HELLO-R-U-THERE"

func (p *icmpPinger) Ping(client net.IP) error {
	var err error
	for i := 0; i < 1+p.retries; i++ {
		err = p.pingWithTimer(client)
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

func (p *icmpPinger) pingWithTimer(client net.IP) error {
	timer := time.Now()
	err := p.ping(client)

	if d := time.Since(timer); d > 100*time.Millisecond {
		if err == nil {
			p.log.Info("ping to client took more than 100ms", "ip", client, "duration", fmt.Sprintf("%dms", d.Milliseconds()))
		} else {
			var neterr net.Error
			if errors.As(err, &neterr) && neterr.Timeout() {
				err = fmt.Errorf("i/o timeout")
			}
			p.log.Info("ping failed", "ip", client, "duration", fmt.Sprintf("%dms", d.Milliseconds()), "error", err)
		}
	}
	return err
}

func (p *icmpPinger) ping(client net.IP) error {
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

	seq := p.lastSeq.Add(1)
	msg := icmp.Message{
		Type: ipv6.ICMPTypeEchoRequest,
		Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  int(seq),
			Data: []byte(echoPayload),
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
		echo, ok := rm.Body.(*icmp.Echo)
		if !ok {
			return fmt.Errorf("error parsing response as ICMPTypeEchoReply")
		}
		if echo.Seq != int(seq) {
			return fmt.Errorf("unexpected sequence number: %d != %d", echo.Seq, seq)
		}
		if !bytes.Equal(echo.Data, []byte(echoPayload)) {
			return fmt.Errorf("payload mismatch: %s", string(echo.Data))
		}
		return nil
	default:
		return err
	}
}

func (p *icmpPinger) neighborSolicitation(client net.IP) error {
	if len(client) != net.IPv6len {
		return fmt.Errorf("only usable with ipv6")
	}
	device, err := net.InterfaceByName(constants.BondDevice)
	if err != nil {
		return fmt.Errorf("bonding device %q not found: %w", constants.BondDevice, err)
	}
	ips, err := network.GetLinkIPAddressesByName(constants.BondDevice, network.ScopeLink)
	if err != nil {
		return fmt.Errorf("getting link IP addresses failed: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("no link IP address for device %s", constants.BondDevice)
	} else if len(ips) > 1 {
		return fmt.Errorf("link IP address not unique for device %s: [%s]", constants.BondDevice)
	}

	ns := icmp.Message{
		Type: ipv6.ICMPTypeNeighborSolicitation,
		Code: 0,
		Body: &icmp.RawBody{
			Data: buildNSBody(client, device.HardwareAddr),
		},
	}

	data, err := ns.Marshal(nil)
	if err != nil {
		return fmt.Errorf("error marshalling ICMP message: %w", err)
	}

	conn, err := icmp.ListenPacket("ip6:ipv6-icmp", fmt.Sprintf("%s%%%s", ips[0], constants.BondDevice))
	if err != nil {
		return fmt.Errorf("error opening raw socket: %w", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(p.timeout / 2)
	pc := conn.IPv6PacketConn()
	if err := pc.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("error setting deadline: %w", err)
	}

	// The multicast hop limit of 255 ensures that the Neighbor Solicitation message
	// remains within the local link. If the hop limit is set to any value less than 255,
	// the message might have been relayed or forwarded by a router, which is not desirable.
	// A hop limit of 255 guarantees that the packet is dropped if it has been routed.
	// The packet is dropped silently if not set.
	// see https://datatracker.ietf.org/doc/html/rfc4861#section-7.1.1
	if err := pc.SetMulticastHopLimit(255); err != nil {
		return fmt.Errorf("error setting multicast hop limit: %w", err)
	}

	// calculate solicited-node multicast address
	destIP := net.ParseIP("ff02::1:ff00:0")
	copy(destIP[13:], client[13:]) // copy last 24 bits
	dst := &net.IPAddr{IP: destIP}
	_, err = pc.WriteTo(data, nil, dst)
	if err != nil {
		return fmt.Errorf("error sending ICMP message to %s: %w", dst, err)
	}

	// Listen for Neighbor Advertisement response
	reply := make([]byte, 1500)
	n, _, _, err := pc.ReadFrom(reply)
	if err != nil {
		var neterr net.Error
		if errors.As(err, &neterr) && neterr.Timeout() {
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
	// :    Octet 0    :    Octet 1    :    Octet 2    :    Octet 3    :
	// :0              :    1          :        2      :            3  :
	// :0 1 2 3 4 5 6 7:8 9 0 1 2 3 4 5:6 7 8 9 0 1 2 3:4 5 6 7 8 9 0 1:
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |                           Reserved (4 bytes)                  |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |                                                               |
	// +                                                               +
	// |                                                               |
	// +                       Target Address                          +
	// |                                                               |
	// +                                                               +
	// |                                                               |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |   Type (1)    |    Length     |             Data...
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-

	buf := make([]byte, 22+len(hwAddr))
	// Target Address (16 bytes)
	copy(buf[4:20], target)

	// Option: Source Link-Layer Address
	buf[20] = 1 // Type (Source Link-Layer Address)
	buf[21] = 1 // Length (in units of 8 octets)
	copy(buf[22:], hwAddr)

	return buf
}

func toStringArray(ips []net.IP) []string {
	if len(ips) == 0 {
		return nil
	}
	out := make([]string, len(ips))
	for i, ip := range ips {
		out[i] = ip.String()
	}
	return out
}
