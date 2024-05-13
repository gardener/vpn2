# VPN2
[![REUSE status](https://api.reuse.software/badge/github.com/gardener/vpn2)](https://api.reuse.software/info/github.com/gardener/vpn2)

This repository contains components to establish network connectivity for Shoot clusters.

## What's inside

[VPN Seed Server](seed-server) - a component that serves an endpoint for incoming connections and allows contacting any IP address within the networks of a Shoot cluster (which are usually private).

[VPN Shoot Client](shoot-client) - a component that establishes connectivity from a Shoot cluster to the endpoint in the Seed cluster allowing contacting any IP address within its network and routes the packets back to the caller.

## Local build

```bash
$ make docker-images
```

## Troubleshoot

### HA Setup

#### vpn-client-init container is crashing
```
failed to create bond0 link device: operation not supported
``` 

Check if you're kernel supports bond devices. You can check on nodes running docker with the following command: \
`docker run -it --rm --privileged --pid=host ubuntu nsenter -t 1 -m -u -n -i sh -c 'cat /proc/config.gz | gunzip | grep CONFIG_BONDING'`

`CONFIGURE_BONDING` must be set to either "m" or "y". 

For more information, see https://www.kernelconfig.io/config_bonding?q=&kernelversion=6.1.90&arch=x86
