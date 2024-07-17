// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"encoding/json"
	"fmt"
	"net"
)

// Send sends the kube-apiserver pod IP to the tunnel controller.
func Send(client net.IP, tunnelConfig IP6Tunnel) error {
	serverAddr, err := net.ResolveUDPAddr("udp6", fmt.Sprintf("[%s]:%d", client.String(), UDPPort))
	if err != nil {
		return fmt.Errorf("Error resolving address: %w", err)
	}
	conn, err := net.DialUDP("udp6", nil, serverAddr)
	if err != nil {
		return fmt.Errorf("Error dialing UDP: %w", err)
	}
	defer conn.Close()

	data, err := json.Marshal(tunnelConfig)
	if err != nil {
		return fmt.Errorf("Error marshalling tunnel config: %w", err)
	}
	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("Error sending tunnel config: %w", err)
	}
	return nil
}
