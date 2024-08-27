// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"fmt"
	"net"
)

// Send sends a client IP to the tunnel controller.
func Send(tunnelControllerIP net.IP, clientIP string) error {
	serverAddr := fmt.Sprintf("[%s]:%d", tunnelControllerIP.String(), tunnelControllerPort)
	conn, err := net.Dial("udp6", serverAddr)
	if err != nil {
		return fmt.Errorf("Error dialing UDP for %s: %w", serverAddr, err)
	}
	defer conn.Close()

	if _, err = conn.Write([]byte(clientIP)); err != nil {
		return fmt.Errorf("Error sending data to %s: %w", serverAddr, err)
	}
	return nil
}
