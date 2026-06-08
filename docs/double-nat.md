# Double NAT using NETMAP

Because Shoot clusters use standard private IPv4 ranges (e.g., `10.0.0.0/8`, `100.64.0.0/10`) that may conflict with the Seed's network space, all IPv4 traffic is mapped to reserved ranges in the `240.0.0.0/8` space.
In non-HA mode, we always use NETMAP. In HA mode, we only map if there is a network overlap.

### Mapped Ranges

| Source | Source Range | Mapped Range | Purpose |
|--------|-------------|--------------|---------|
| Shoot pod networks | e.g., `10.96.0.0/11` | `242.0.0.0/8` | Pod-to-pod traffic |
| Shoot service networks | e.g., `100.64.0.0/13` | `240.240.0.0/12` | Service traffic |
| Shoot node networks | e.g., `10.100.0.0/16` | `241.0.0.0/16` | Node traffic |
| Seed pod network | Client-specified | `240.241.0.0/16` | Seed pod traffic |

### Mapping Algorithm (netmap.go)

The mapping uses a bitwise XOR-based algorithm that preserves subnet sizes:

```go
func netmapIP(ip net.IP, subnet net.IPNet) net.IP {
    ipv4 := ip.To4()
    subnetBase := subnet.IP.To4()
    mappedIP := make(net.IP, len(ipv4))
    for i := range ipv4 {
        mappedIP[i] = (ipv4[i] & ^subnet.Mask[i]) | subnetBase[i]
    }
    return mappedIP
}
```

This extracts the host bits from the source IP and places them into the mapped subnet while preserving the subnet mask size.

### NETMAP Rules

The tunnel device is **tun0** in non-HA and the **bond device** setup by the tunnel controller in the HA case.

**On the Shoot side (client):**

```
# PREROUTING: inbound traffic from VPN gets unmapped
iptables -t nat -A PREROUTING -i <tunl-device> -d <mapped-ip> -j NETMAP --to <original-ip>

# POSTROUTING: outbound traffic to VPN gets mapped
iptables -t nat -A POSTROUTING -o <tunl-device> -s <original-ip> -j NETMAP --to <mapped-ip>
```

**On the Seed side (server):**

- Same pattern, reverse direction
- Maps seed pod network: `240.241.0.0/16` <-> `<actual-seed-pod-network>`
- In HA mode with Envoy, OUTPUT chain maps with owner match (`--gid-owner <envoy-gid>`)

### When NETMAP Is Applied

| Mode/Location | IPv4 Mapped Networks | Condition | Where Applied |
|---|---|---|---|
| Non-HA server | Shoot networks (pod/service/node) and seed pod network | Always | iptables: PREROUTING/POSTROUTING (firewall subcommand) + OUTPUT chain for Envoy (SetIPTableRules) |
| HA server | None | Server in HA mode does NOT apply NETMAP (`!cfg.IsHA` guard in SetIPTableRules) | N/A |
| Non-HA client | Shoot services (pod/service/node) | Always | iptables: PREROUTING/POSTROUTING (SetIPTableRules) |
| HA client | Shoot services (pod/service/node) and seed pod network | Only if seed pod network overlaps with any shoot network | PREROUTING only via owner match (`--gid-owner <envoy-gid>`) in OUTPUT chain |

## Reference

```
NETMAP
       This target allows you to statically map a whole network of
       addresses onto another network of addresses.  It can only be used
       from rules in the nat table.

       --to address[/mask]
              Network address to map to.  The resulting address will be
              constructed in the following way: All 'one' bits in the
              mask are filled in from the new `address'.  All bits that
              are zero in the mask are filled in from the original
              address.

       IPv6 support available since Linux kernels >= 3.7.
```

<https://man7.org/linux/man-pages/man8/iptables-extensions.8.html>
