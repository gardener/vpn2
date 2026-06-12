# Tunnel Controller

The Tunnel Controller is a core component that bridges the OpenVPN tunnel and the Kubernetes networking layer. It runs on port **5400** (UDP6) and dynamically creates/destroys `ip6tnl` tunnel devices.

## Protocol

### Message Format

**Client -> Tunnel Controller (UDP6 port 5400):**

| Field | Value |
|-------|-------|
| Direction | Client -> Server |
| Protocol | UDP6 |
| Content | Plain text IPv4 or IPv6 address string (e.g., `10.0.0.1`) |
| Max length | ~46 bytes (maximum IPv6 address length) |
| Buffer size | 1024 bytes |
| Read deadline | 4 seconds (`TunnelControllerUpdateTimeout` = 2 x `PathControllerUpdateInterval`) |

**Tunnel Controller -> Client:**

- No explicit response. The controller processes the message and creates a Linux `ip6tnl` tunnel device.

### Tunnel Lifecycle

When a new IP is received:

1. Check if this IP is already known and hasn't failed recently
2. Delete any existing `ip6tnl` link with that name
3. Create a new `ip6tnl` tunnel linking the local bond IP to the remote client IP
4. Install a route directing traffic to the client IP through the new tunnel device
5. Mark the tunnel as creation-complete

When a tunnel is received again with the same IP:

- If creation was previously completed and no errors occurred: skip update
- If creation failed within the last 30 seconds: back off and retry after the backoff period
- If backoff has passed: retry the update

### Tunnel Cleanup

| Parameter | Value | Purpose |
|-----------|-------|---------|
| `cleanUpPeriod` | 15 minutes | How often garbage collection runs |
| `expirationDuration` | 10 minutes | Tunnels inactive for 10+ minutes are cleaned up |
| `creationFailureBackoff` | 30 seconds | Minimum wait before retrying a failed tunnel creation |

The tunnel controller maintains a map of known kube-apiservers keyed by remote IP address, each tracking pod IP, creation status, failures, and timestamps.

### Readiness Check

- At least one kube/apiserver configured
- All tunnels must have been created without errors

### ip6tnl Tunnel Routes

The tunnel controller installs host routes to individual pod IPs:

```
ip route replace <pod-ip>/32 dev bond0-ip6tnl<suffix>
```

Link names use the last two bytes of the remote address as suffix (must be <= 15 chars in Linux):

```
bond0-ip6tnl<last-byte-2><last-byte-1>
```
