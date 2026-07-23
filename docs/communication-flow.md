# Communication flow

The communication flow differs based on the needed availability of the VPN components. In general we distinguish between:

- **high availability**
- **non-high availability**

## Non-high availability

In non-HA mode, a single VPN tunnel connects the Shoot to the Seed.

### Connection Steps

```
Step 1: VPN Server starts in the Seed
  - Opens TCP6 listener (port 1194)
  - Creates tun0 device
  - Sets up iptables firewall rules for double NAT (filter INPUT, nat PREROUTING/POSTROUTING for NETMAP)
  - Sets up routes for shoot pod/service/node networks via tun0

Step 2: VPN Client connects from the Shoot
  - Connects to the Seed's OpenVPN endpoint 
  - Mutually authenticates with X.509 certificates
  - Establishes encrypted tunnel over tun0

Step 3: Traffic flows through the tunnel
  - Shoot pods send traffic -> MASQUERADE -> tun0 -> OpenVPN -> Seed pods
  - Seed receives -> NETMAP un-maps IPv4 -> Shoot pods
```

### Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `VPN_NETWORK` | `fd8f:6d53:b97a:1::/96` | IPv6 transfer network (must be /96) |
| `SHOOT_POD_NETWORKS` | `100.96.0.0/11` | Shoot pod CIDRs to route |
| `SHOOT_SERVICE_NETWORKS` | `100.64.0.0/13` | Shoot service CIDRs to route |
| `SHOOT_NODE_NETWORKS` | (none) | Shoot node CIDRs to route |
| `SEED_POD_NETWORK` | (required) | Seed pod network for reverse NAT |

### Non-HA Device Layout

```
Shoot Cluster:
  tun0  -- OpenVPN VPN tunnel device (layer 3)
  eth0  -- Kubernetes pod network interface

Seed Cluster:
  tun0  -- OpenVPN VPN tunnel device (layer 3)
```

---

## High availability

In High Availability mode, two VPN server pods in the Seed provide redundancy, with a **bond device** on the Shoot side managing failover.
Each kube-apiserver pod contains 2 VPN client sidecars connecting to the VPN server.

### Diagram

A detailed diagram about the VPN connection flow in the high-availability setup:

![diagram](https://raw.githubusercontent.com/gardener/gardener/refs/heads/master/docs/development/content/vpn-ha-architecture.png)

### Connection Steps

```
Step 1: Two VPN servers start in the Seed as StatefulSets (vpn-seed-server-0, vpn-seed-server-1)
  - Each server gets a unique /112 subnet within the VPN /96 network
  - Server 0 subnet:  fd8f:6d53:b97a:1::100::/112  (index 0)
  - Server 1 subnet:  fd8f:6d53:b97a:1::101::/112  (index 1)
  - Each uses tap0 (layer 2) device instead of tun0

Step 2: Kube-apiserver sidecar OpenVPN Client creates two tunnels
  - Connects outbound to both vpn-seed-server-0 and vpn-seed-server-1
  - Creates tap0 and tap1 devices (one per server)
  - Binds them to bond0 in active-backup (default) mode

Step 3: IP addresses are assigned
  - bond0 gets an IP from the /104 bonding subnet of the VPN network
  - Seed client bond address: fd8f:6d53:b97a:1::a:<index>/112
    - Index is acquired from the distributed IP pool broker (via Kubernetes pod annotations)
  - Two ip6tnl tunnels are created from the Seed client to each VPN server

Step 4: Shoot VPN client connects with bonded devices
  - Creates tap0 and tap1 devices (one per server)
  - Binds them to bond0 in active-backup mode
  - bond0 gets an IP: fd8f:6d53:b97a:1::b:<client-index>/112
  - Connects to both VPN servers simultaneously

Step 5: Tunnel Controller creates routes on shoot VPN client
  - Tunnel Controller on Shoot listens on bond0 IP:5400 (UDP6)
  - When Shoot client IP is received, creates ip6tnl + route in the kube-apiserver pod
  - When Seed client IP is received, creates ip6tnl + route on the Shoot VPN Client pod
  - Routes have 10-minute expiration; stale tunnels are cleaned up every 15 minutes

Step 6: Path Controller creates routes in Kube-apiserver VPN client
  - pings all shoot-side VPN clients regularly every few seconds. If the active routing path is not responsive anymore,
  the routing is switched to the other responsive routing path.
  - Send kube-apiserver IP to the tunnel-controller running on all clients
  - Setup routes for shoot service, node and pod CIDR to route through bond tunnel device

Using an IPv6 transport network for communication between the bonding devices of the VPN clients, additional
tunnel devices are needed on both ends to allow transport of both IPv4 and IPv6 packets.
For this purpose, `ip6tnl` type tunnel devices are in place (an IPv4/IPv6 over IPv6 tunnel interface).
```

### HA Addressing Scheme

Assuming the default VPN network `fd8f:6d53:b97a:1::/96`:

| Network | Prefix | Description |
|---------|--------|-------------|
| VPN network | `fd8f:6d53:b97a:1::/96` | Root IPv6 network |
| Bonding network | `fd8f:6d53:b97a:1::0/104` | /104 subnet for bond devices |
| VPN index 0 tunnel | `fd8f:6d53:b97a:1::100::/112` | Subnet for server-0 |
| VPN index 1 tunnel | `fd8f:6d53:b97a:1::101::/112` | Subnet for server-1 |
| Shoot client 0 | `fd8f:6d53:b97a:1::b:0/112` | IP of shoot client 0 |
| Seed clients | `fd8f:6d53:b97a:1::a:1` to `fd8f:6d53:b97a:1::a:ffff` | Range of seed client IPs |

### HA Device Layout

```
Shoot Cluster:
  bond0  -- Bonding device (active-backup or balance-rr)
  tap0   -- Slave to bond0 (connection to vpn-seed-server-0 via OpenVPN)
  tap1   -- Slave to bond0 (connection to vpn-seed-server-1 via OpenVPN)

Seed Cluster (client in kube-apiserver pod):
  bond0            -- Bonding device (active-backup or balance-rr)
  bond0-ip6tnl-xx  -- Bond Tunnel device

  tap0   -- Slave to bond0 (VPN tunnel 0)
  tap1   -- Slave to bond0 (VPN tunnel 1)
  eth0   -- Kubernetes pod network

Seed Cluster (server side):
  vpn-seed-server-0
    tap0  -- VPN tunnel device for server 0
  vpn-seed-server-1
    tap0  -- VPN tunnel device for server 1
```

#### Bonding Modes

| Mode | Description | Configuration |
|------|-------------|---------------|
| `active-backup` (default) | Primary/secondary failover | monitor every 100ms, primary = tap0, 5 gratuitous ARP on failover |
| `balance-rr` | Round-robin load balancing | monitor every 100ms |

---

### HTTP Reverse Proxy

Shoot OpenVPN clients (vpn-shoot-client) connect to the correct OpenVPN Server using the HTTP Proxy feature provided by OpenVPN.
This is needed to route the request arriving at the shared istio-ingressgateway LoadBalancer running in the Seed to route to the correct OpenVPN server.

```
http-proxy <KUBE_APISERVER INTERNAL_DNS_NAME> 8443
http-proxy-option CUSTOM-HEADER X-Gardener-Destination outbound|1194||vpn-seed-server-0.<SHOOT_TECHNICAL_ID>.svc.cluster.local
```
