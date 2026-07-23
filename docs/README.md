# VPN2 Networking Documentation

## Architecture Overview

vpn2 is a Go-based VPN infrastructure component for the Gardener. It establishes encrypted network tunnels between Kubernetes **Shoot clusters** (workload clusters) and their **Seed cluster** (control plane).

The architecture uses a **reversed VPN** design: the VPN server runs in the Seed, while the clients run in the Shoot,  initiating an **outbound** connection, as well as side-car containers in the kube-apiserver pods. This allows Shoot clusters behind restrictive firewalls or NAT to connect without requiring inbound access.
Reference: [GEP-0014: Reversed Cluster VPN](https://github.com/gardener/enhancements/blob/main/geps/0014-reversed-cluster-vpn/README.md)

### System Components

| Component | Location | Purpose |
|-----------|----------|---------|
| Standalone VPN Client (OpenVPN) | Shoot cluster | Outbound VPN tunnel to Seed |
| VPN Client as kube-apiserver sidecar (OpenVPN) | Seed cluster | Enables kube-apiserver to access nodes/services/pods running in the Shoot   |
| VPN Server (OpenVPN) | Seed cluster (standalone Pod) | Accepts inbound VPN tunnels |
| Tunnel Controller | Shoot cluster | Receives UDP from seed clients, creates ip6tnl tunnels |
| Path Controller | Seed cluster (kube-apiserver side-car/only in HA case) | Configure routes for service/node/pod CIDR in kube-apiserver pod based on healthy shoot VPN clients |

## Contents

- [Communication Flow](./communication-flow.md)
- [Double NAT](./double-nat.md)
- [MTU](./mtu.md)
- [Kernel settings](./kernel-settings.md)
- [Firewall setup](./firewall.md)
- [Metrics](./metrics.md)

### High availability

- [Tunnel controller](./high-availability/tunnel-controller.md)
- [IPAM for VPN clients running in kube-apiserver pods](./high-availability/ipam.md)
- [Failover mechanisms](./high-availability/failover-mechanisms.md)
