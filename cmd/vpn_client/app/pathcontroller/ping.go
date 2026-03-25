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
	tries := p.retries + 1
	for try := 1; try <= tries; try++ {
		err = p.pingWithTimer(client)
		if err == nil {
			break
		}
		p.log.Info("ping failed", "ip", client, "error", err, "try", fmt.Sprintf("%d/%d", try, tries))
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
			if neterr, ok := errors.AsType[net.Error](err); ok && neterr.Timeout() {
				err = fmt.Errorf("i/o timeout after %dms", d.Milliseconds())
			}
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

	seq := p.lastSeq.Add(1) & 0xffff // is marshaled as uint16 so we need to mask it
	msg := icmp.Message{
		Type: ipv6.ICMPTypeEchoRequest,
		Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff, // is marshaled as uint16 so we need to mask it
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
