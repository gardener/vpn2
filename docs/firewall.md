# Firewall Rules and Routing

## Server Firewall (OpenVPN up/down scripts)

The server config runs `vpn-server firewall --mode up` when the tun/tap device is created:

**IPv4 filter INPUT:**

```
iptables -A filter INPUT -i <device> -m state --state RELATED,ESTABLISHED -j ACCEPT
iptables -A filter INPUT -i <device> -j DROP
```

**IPv6 filter INPUT:**

```
ip6tables -A filter INPUT -i <device> -m state --state RELATED,ESTABLISHED -j ACCEPT
ip6tables -A filter INPUT -i <device> -j DROP
```

This blocks all traffic on the VPN device except established/related connections, effectively isolating the VPN tunnel from the Seed's other interfaces.

**Route setup:**

```
ip route replace <shoot-pod-network> dev <tun0/tap0>
ip route replace <shoot-service-network> dev <tun0/tap0>
ip route replace <shoot-node-network> dev <tun0/tap0>
```

**Non-HA mode NAT:**

```
iptables -t nat -A PREROUTING -i tun0 -d 240.241.0.0/16 -j NETMAP --to <seed-pod-network>
iptables -t nat -A POSTROUTING -o tun0 -s <seed-pod-network> -j NETMAP --to 240.241.0.0/16
```

## Client Firewall

The client-side firewall depends on whether this is a Shoot-side VPN client (`IS_SHOOT_CLIENT=true`) or a Seed-side VPN client (only in HA mode, `IS_SHOOT_CLIENT=false`).

**Shoot client (always, `IS_SHOOT_CLIENT=true`):**

```
# Forward traffic through tunnel device (tun0 or bond0+)
iptables -A FORWARD -i <tun0|bond0+> -j ACCEPT

# Double NAT if not HA, or HA with overlapping networks
iptables -t nat -A PREROUTING -i <device> -d <mapped-ip> -j NETMAP --to <original-ip>
iptables -t nat -A POSTROUTING -o <device> -s <original-ip> -j NETMAP --to <mapped-ip>

# MASQUERADE for outbound traffic
iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE

# TCP MSS clamping to prevent MTU black holes
iptables -t mangle -A POSTROUTING -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu
```

**Seed client (HA only, `IS_SHOOT_CLIENT=false`, runs on Envoy pods):**

```
# Double NAT for Shoot networks (only if overlapping with seed pod network)
iptables -t nat -A OUTPUT -o <bond0+> -m owner --gid-owner <envoy-vpn-group-id> \
  -d <shoot-pod-mapped> -j NETMAP --to <shoot-pod-original>
iptables -t nat -A OUTPUT -o <bond0+> -m owner --gid-owner <envoy-vpn-group-id> \
  -d <shoot-svc-mapped> -j NETMAP --to <shoot-svc-original>
iptables -t nat -A PREROUTING -i <bond0+> -d <seed-pod-mapped> -j NETMAP --to <seed-pod-original>
iptables -t nat -A POSTROUTING -o <bond0+> -s <seed-pod-original> -j NETMAP --to <seed-pod-mapped>

# ICMPv6 for Neighbor Discovery
ip6tables -A INPUT -i bond0+ -p icmpv6 -j ACCEPT

# Accept established/related
iptables -A INPUT -i bond0+ -m state --state RELATED,ESTABLISHED -j ACCEPT

# Drop all other inbound
iptables -A INPUT -i bond0+ -j DROP
```

The Seed client also applies reverse NETMAP via IPSET with owner match so that Envoy pods generate traffic that gets automatically mapped before it enters the VPN tunnel. This is done via the `OUTPUT` chain in the NAT table with `-m owner --gid-owner <envoy-vpn-group-id>` selector.

## Reverse Path Routing (non-HA)

A custom OpenVPN `up` script on the Shoot client installs a route for the Seed pod network:

```
ip route replace <seed-pod-network> dev tun0
```
