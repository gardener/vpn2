# Failover mechanisms

The High-availability setup provides failover mechanisms across two distinct layers: **L2 (vpn-servers)** and **L3 (vpn-shoots)**.

## Summary

| Component | Layer | Primary Device | Failover Mechanism | Trigger |
| --- | --- | --- |  --- | --- |
| **vpn-server** | L2 |  `bond0` (via `tap` devices) | Native device bonding | Link state change (e.g., vpn-server restarts) |
| **vpn-shoot** | L3 |  `ip6tnl` devices | Route table swap via Path Controller | Ping timeout (e.g., vpn-shoot restarts) |

## Layer 2 HA: `vpn-servers`

The `vpn-servers` operate at Layer 2 (Data Link Layer) and function similarly to network switches.

* **Connection Mechanism:** VPN clients connect to these servers by plugging in a virtual "cable" (represented by TAP devices, such as `tap0` or `tap1`).
* **Bonding (`bond0`):** To achieve high availability, we use network bonding. Both `vpn-servers` are aggregated into a single L2 logical device (`bond0`), allowing them to be used interchangeably.
* **Failover Behavior:** L2 failover relies **strictly on link state**.
* If a tap device goes down (e.g., `tap0` down), traffic immediately fails over to the alternate tap device (e.g., `tap1`).
* *Note:* This layer does not rely on active health checks, pings, or heartbeats to trigger a failover.

## Layer 3 HA: `vpn-shoots`

The `vpn-shoots` operate at Layer 3 (Network Layer) and function similarly to routers.

* **Connection Mechanism:** These "routers" are represented by corresponding `ip6tnl` (IPv6 tunnel) devices.
* **Health Checks:** Unlike the L2 tap devices, `ip6tnl` devices do not have native failover or link-state health checks.
* **Failover Behavior:** To manage L3 HA, we utilize **Path Controller**.
* The Path Controller actively monitors the health of the `vpn-shoots` via ping.
* If the Path Controller cannot ping a `vpn-shoot` for an extended period, a failover is triggered.
* The failing `ip6tnl` device is swapped out for the alternate `vpn-shoot`'s device directly within the route table.
