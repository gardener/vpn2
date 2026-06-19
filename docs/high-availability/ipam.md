# IP Address Management and Bonding

## IP Address Assignment

In HA mode, IPs are assigned using a distributed IP pool broker backed by Kubernetes pod annotations on the kube-apiserver pods.
This ensures conflict-free allocation across multiple VPN clients which are running as side-cars containers in the kube-apiserver pod.

### Broker Algorithm (`pkg/ippool/broker.go`)

1. **Acquire**: Requests an IP from the pod pool manager using label selectors
2. **Reserve**: Writes a reservation to the pod annotation (two-phase commit)
3. **Confirm**: Marks the IP as "used" in the annotation
4. **Conflict handling**: If another pod claims the same IP, retries with backoff and random delay
5. **Max retries**: 30 iterations with exponential backoff

### IP Assignment Functions

**Shoot client IP (deterministic):**

```
IP = vpnNetwork.IP with:
  byte[13] = 0xb
  byte[15] = client_index
```

**Seed client IP (distributed broker):**

```
IP = vpnNetwork.IP with:
  byte[13] = 0xa
  byte[14] = 0x00
  byte[15] = range 0x0001 to 0xFFFF (acquired from broker)
```

### VPN Network Subnet Calculation

For HA mode, the VPN server's /112 subnet is derived from its index:

```
base = vpnNetwork.IP
base[12] = 0x01
base[13] = vpn_index
base[14] = 0x00
base[15] = 0x00
```

This means VPN server 0 gets `fd8f:6d53:b97a:1::100::/112` and server 1 gets `fd8f:6d53:b97a:1::101::/112`.
