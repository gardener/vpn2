
# MTU Management

## Auto MTU Detection

By default, the OpenVPN tunnel uses the OpenVPN default MTU (1500 bytes). In environments where the underlying
network supports a larger MTU (e.g. clusters connected via a high-speed backbone with jumbo frames), configuring
a larger tunnel MTU can significantly improve throughput and reduce latency — in particular for geographically
distant seed/shoot pairs.

When `OPENVPN_AUTO_MTU=true`, the client discovers the optimal MTU:

1. Get the MTU of the default route interface (typically `eth0` in a container)
2. Subtract `130` bytes overhead (IPv6 header + TCP header + OpenVPN framing)
3. Enforce a minimum of `1280` bytes (IPv6 MTU viability threshold, RFC 8200)

```
tunnelMTU = max(1280, defaultMTU - 130)
```

The overhead accounts for:

- IPv6 base header: 40 bytes
- TCP header (with options): ~40 bytes
- OpenVPN framing/authentication overhead: ~50 bytes

If `OPENVPN_AUTO_MTU` is not enabled, the tunnel MTU defaults to 0 (unspecified), letting OpenVPN use its default behavior.
