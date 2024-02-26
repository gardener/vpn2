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
