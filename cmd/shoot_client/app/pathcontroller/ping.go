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
	log logr.Logger
}

const protocolICMP = 1

func (p icmpPinger) Ping(client net.IP) error {
	timer := time.Now()
	defer func() {
		if time.Since(timer) > 100*time.Millisecond {
			p.log.Info("ping to client took more than 100ms", "ip", client)
		}
	}()

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
