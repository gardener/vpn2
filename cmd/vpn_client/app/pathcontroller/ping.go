// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pathcontroller

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type icmpPinger struct {
	log     logr.Logger
	timeout time.Duration
	retries int
}

const protocolICMP = 1

func (p icmpPinger) Ping(client net.IP) error {
	var err error
	for i := 0; i < 1+p.retries; i++ {
		err = p.ping(client)
		if err == nil {
			break
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

	c, err := icmp.ListenPacket("udp4", "")
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
