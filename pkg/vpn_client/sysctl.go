// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpn_client

import (
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/lorenzosaino/go-sysctl"

	"github.com/gardener/vpn2/pkg/config"
)

// EnableIPv6Networking enables IPv6 networking on the system.
func EnableIPv6Networking(log logr.Logger) error {
	strVal, err := sysctl.Get("net.ipv6.conf.all.disable_ipv6")
	if err != nil {
		return fmt.Errorf("failed to read net.ipv6.conf.all.disable_ipv6: %w", err)
	}
	value, err := strconv.ParseInt(strVal, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse net.ipv6.conf.all.disable_ipv6 value %q: %w", strVal, err)
	}
	if value == 1 {
		log.Info("IPv6 networking is disabled in the pod, trying to enable it")
		// Enable IPv6 networking on the system (needed for GKE clusters)
		if err := sysctl.Set("net.ipv6.conf.all.disable_ipv6", "0"); err != nil {
			return fmt.Errorf("failed to enable IPv6 networking: %w (hint: container may need to be privileged)", err)
		}
		log.Info("IPv6 networking enabled")
	}
	return nil
}

// DisableMartianLogging disables logging of packets with un-routable source addresses (martians) globally and for new interfaces.
func DisableMartianLogging() error {
	// Disable logging of packets with un-routable source addresses (martians) globally.
	if err := sysctl.Set("net.ipv4.conf.all.log_martians", "0"); err != nil {
		return err
	}
	// Disable logging of packets with un-routable source addresses (martians) for new interfaces.
	if err := sysctl.Set("net.ipv4.conf.default.log_martians", "0"); err != nil {
		return err
	}
	return nil
}

// DisableRpFilter disables reverse path filtering globally and for new interfaces.
func DisableRpFilter() error {
	// Disable reverse path filtering globally.
	if err := sysctl.Set("net.ipv4.conf.all.rp_filter", "0"); err != nil {
		return err
	}
	// Disable reverse path filtering for new interfaces.
	if err := sysctl.Set("net.ipv4.conf.default.rp_filter", "0"); err != nil {
		return err
	}
	return nil
}

// EnableIPForwarding enables IP forwarding for both IPv4 and IPv6 on the system.
func EnableIPForwarding() error {
	// Enable IPv4 forwarding on the system.
	if err := sysctl.Set("net.ipv4.ip_forward", "1"); err != nil {
		return err
	}
	// Enable IPv6 forwarding on the system.
	if err := sysctl.Set("net.ipv6.conf.all.forwarding", "1"); err != nil {
		return err
	}
	return nil
}

// KernelSettings sets the kernel parameters required for the VPN tunnel to function properly.
func KernelSettings(log logr.Logger, cfg config.VPNClient) error {
	// Disable martian logging on both sides.
	if err := DisableMartianLogging(); err != nil {
		return err
	}
	// Disable reverse path filtering on both sides.
	if err := DisableRpFilter(); err != nil {
		return err
	}
	// For seed clients, we need to enable IPv6 networking to be able to use IPv6 addresses for the tunnel.
	if !cfg.IsShootClient {
		return EnableIPv6Networking(log)
	}
	// For shoot clients, we need to enable IP forwarding to be able to route traffic from the tunnel to the shoot cluster and back.
	return EnableIPForwarding()
}
