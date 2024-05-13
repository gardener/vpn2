package shoot_client

import (
	"github.com/cilium/cilium/pkg/sysctl"
	"github.com/gardener/vpn2/pkg/config"
)

// KernelSettings sets the kernel parameters required for the VPN tunnel to function properly.
func KernelSettings(cfg config.ShootClient) error {
	if !cfg.IsShootClient {
		return nil
	}
	// Enable IPv4 forwarding on the system.
	if err := sysctl.Enable("net.ipv4.ip_forward"); err != nil {
		return err
	}
	// Enable IPv6 forwarding on the system.
	if err := sysctl.Enable("net.ipv6.conf.all.forwarding"); err != nil {
		return err
	}
	// Set the keepalive time for TCP connections.
	if err := sysctl.WriteInt("net.ipv4.tcp_keepalive_time", cfg.TCP.KeepAliveTime); err != nil {
		return err
	}
	// Set the keepalive interval for TCP connections.
	if err := sysctl.WriteInt("net.ipv4.tcp_keepalive_intvl", cfg.TCP.KeepAliveInterval); err != nil {
		return err
	}
	// Set the number of keepalive probes for TCP connections.
	if err := sysctl.WriteInt("net.ipv4.tcp_keepalive_probes", cfg.TCP.KeepAliveProbes); err != nil {
		return err
	}
	return nil
}
