// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpn_client

import (
	"fmt"

	ciliumSysctl "github.com/cilium/cilium/pkg/datapath/linux/sysctl"
	"github.com/go-logr/logr"
	"github.com/spf13/afero"

	"github.com/gardener/vpn2/pkg/config"
)

var sysctl ciliumSysctl.Sysctl

func init() {
	sysctl = ciliumSysctl.NewDirectSysctl(afero.NewOsFs(), "/proc/sys")
}

// EnableIPv6Networking enables IPv6 networking on the system.
func EnableIPv6Networking(log logr.Logger) error {
	value, err := sysctl.ReadInt([]string{"net", "ipv6", "conf", "all", "disable_ipv6"})
	if err != nil {
		return fmt.Errorf("failed to read net.ipv6.conf.all.disable_ipv6: %w", err)
	}
	if value == 1 {
		log.Info("IPv6 networking is disabled in the pod, trying to enable it")
		// Enable IPv6 networking on the system (needed for GKE clusters)
		if err := sysctl.Disable([]string{"net", "ipv6", "conf", "all", "disable_ipv6"}); err != nil {
			return fmt.Errorf("failed to enable IPv6 networking: %w (hint: container may need to be privileged)", err)
		}
		log.Info("IPv6 networking enabled")
	}
	return nil
}

// KernelSettings sets the kernel parameters required for the VPN tunnel to function properly.
func KernelSettings(log logr.Logger, cfg config.VPNClient) error {
	if !cfg.IsShootClient {
		return EnableIPv6Networking(log)
	}

	// Enable IPv4 forwarding on the system.
	if err := sysctl.Enable([]string{"net", "ipv4", "ip_forward"}); err != nil {
		return err
	}
	// Enable IPv6 forwarding on the system.
	if err := sysctl.Enable([]string{"net", "ipv6", "conf", "all", "forwarding"}); err != nil {
		return err
	}
	return nil
}
