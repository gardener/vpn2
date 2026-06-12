# Kernel Configuration

The client applies several kernel parameters for optimal VPN performance:

## Sysctl Settings

```bash
# Ensure IPv6 is enabled
net.ipv6.conf.all.disable_ipv6 = 0

# Disable martian packet logging (noise reduction)
net.ipv4.conf.all.log_martians = 0
net.ipv4.conf.default.log_martians = 0

# Disable reverse path filtering (required for multi-path routing through tunnels)
net.ipv4.conf.all.rp_filter = 0
net.ipv4.conf.default.rp_filter = 0

# Enable IP forwarding (required on Shoot side to route tunnel traffic to pods)
net.ipv4.ip_forward = 1
net.ipv6.conf.all.forwarding = 1
```

## Conntrack Optimization

Optimized for high connection churn and NAT scenarios (Shoot clients only):

| Parameter | Value | Default | Purpose |
|-----------|-------|---------|---------|
| `net.ipv4.ip_local_port_range` | `1024 65535` | System default | Wider port range for connections |
| `net.ipv4.tcp_tw_reuse` | `1` | 0 | Reuse time-wait sockets |
| `net.ipv4.tcp_fin_timeout` | `30` | 60 | Reduce FIN timeout |
| `net.netfilter.nf_conntrack_tcp_timeout_established` | `600` | 432000 (5 days) | Faster cleanup of stale connections |
| `net.netfilter.nf_conntrack_tcp_timeout_syn_sent` | `30` | 120 | Faster SYN retry |
| `net.netfilter.nf_conntrack_tcp_timeout_syn_recv` | `30` | 120 | Faster SYN-ACK retry |
| `net.netfilter.nf_conntrack_tcp_timeout_fin_wait` | `30` | 120 | Faster FIN wait cleanup |
| `net.netfilter.nf_conntrack_tcp_timeout_time_wait` | `30` | 60 | Faster TIME-WAIT cleanup |
| `net.netfilter.nf_conntrack_tcp_timeout_close_wait` | `30` | 120 | Faster CLOSE-WAIT cleanup |
| `net.netfilter.nf_conntrack_tcp_timeout_unacknowledged` | `60` | 300 | Faster retransmission |
| `net.netfilter.nf_conntrack_tcp_timeout_max_retrans` | `120` | 300 | Faster max retransmission |
