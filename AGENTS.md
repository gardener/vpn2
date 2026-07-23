# AGENTS.md

This file provides guidance to AI agents when working with code in this repository.

## Project Overview

This is **Gardener VPN2** — a Kubernetes-native VPN solution that establishes network connectivity between Gardener Shoot clusters (tenant clusters) and their Seed cluster control planes. It produces two binaries/container images: `vpn-server` and `vpn-client`. Communication always uses IPv6 as the VPN transfer network and OpenVPN under the hood.

## Commands

```bash
# Build binaries (cross-compiled for Linux, CGO_ENABLED=0)
make build

# Run all tests (works on Linux, but not macOS)
make test

# Run tests requiring Linux network capabilities (netlink, iptables) run this on MacOS
make test-docker

# Run a single Ginkgo test suite or specific test
go test ./pkg/network/... -run TestNetwork
go test ./pkg/config/... -run TestConfig --ginkgo.focus="<test description>"

# Format code (uses goimports + goimports-reviser)
make format

# Lint and vet
make check

# Static security analysis (gosec)
make sast

# Tidy dependencies
make tidy

# Build Docker images
make docker-images

# Build and push to local Gardener kind cluster for testing
make vpn-server-to-gardener-local
make vpn-client-to-gardener-local
```

To debug-build images with `DEBUG=true`:
```bash
DEBUG=true make vpn-server-to-gardener-local
```

After pushing to a local cluster, patch the deployment to force a re-pull:
```bash
kubectl -n shoot--local--local patch deployment <name> -p '{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"'$(date +%s)'"}}}}}'
```

## Architecture

Both binaries are configured entirely via **environment variables** (parsed by `caarlos0/env`). Neither binary runs OpenVPN directly — they generate an OpenVPN config file and then the Docker `ENTRYPOINT` execs OpenVPN. The Go code handles:
1. Parsing env config (`pkg/config/`)
2. Setting iptables rules
3. Optionally detecting MTU
4. Writing the OpenVPN `.config` file to disk

### cmd/ — Binary Entrypoints

**`cmd/vpn_server/app/`** — Server-side cobra command tree:
- Root command: reads env, sets iptables, writes `/openvpn-server.config`
- `setup/` — initial server network setup
- `exporter.go` — Prometheus metrics exporter (wraps kumina/openvpn_exporter)
- `firewall.go` — firewall rule management subcommand
- `readiness.go`, `liveness.go` — health probe subcommands

**`cmd/vpn_client/app/`** — Client-side cobra command tree:
- Root command: reads env, sets iptables, writes `/openvpn-client.config`
- `setup/` — initial client setup (bond0, sysctl)
- `pathcontroller/` — pings VPN client IPs, routes traffic over the best path through `bond0`
- `tunnelcontroller/` — HA mode only: listens on UDP/IPv6 for kube-apiserver pod IPs, creates `ip6tnl` point-to-point tunnel devices

### pkg/ — Shared Packages

- **`pkg/config/`** — All config structs matching env var names. Client config: `vpn-client.go`; server config: `vpn-server.go`; HA path/tunnel controller configs in separate files.
- **`pkg/network/`** — Linux networking primitives: IP utilities, iptables wrappers, netlink link/route management, MTU detection, NAT-style IP range remapping (`netmap.go`).
- **`pkg/openvpn/`** — OpenVPN config file generation via `text/template`. `config_client.go` and `config_server.go` contain the templates and `Write*ConfigFile` functions. `health/` parses the OpenVPN status file for readiness checks.
- **`pkg/shoot_client/tunnel/`** — HA tunnel controller: a UDP server that receives kube-apiserver pod IPs from the path controller and manages `ip6tnl` devices. Includes a sliding-window watchdog (`watchdog.go`) that triggers OpenVPN `SIGHUP` on repeated failures.
- **`pkg/vpn_client/`**, **`pkg/vpn_server/`** — Side-specific helpers: bonding setup, cleanup of stale devices, iptables rules, sysctl settings, and `values.go` which computes the template data for the server OpenVPN config.

### HA Mode

In HA mode the client connects to two VPN servers and bonds the tunnels via a Linux `bond0` interface. The `path-controller` subcommand continuously pings endpoints and adjusts routing. The `tunnel-controller` manages `ip6tnl` devices for direct kube-apiserver pod connectivity. The `bond0` kernel module (`CONFIG_BONDING`) must be available on the host.

## Testing Notes

Most tests in `pkg/network/` and `pkg/vpn_client/` require Linux network capabilities (`NET_ADMIN`, `MKNOD`). These tests will fail or be skipped on macOS — use `make test-docker` for the full suite. Tests use **Ginkgo v2 + Gomega**; each package has a `*_suite_test.go` bootstrapping the suite runner.

## golangci-lint

Config is in `.golangci.yaml`. The `importas` linter enforces import aliases for k8s/gardener packages. The `gci` formatter enforces import grouping order: standard library → third-party → local module. Run `make format` before committing to avoid lint failures in CI.

## git
- Create commits on top of the branch that is currently checked out. Avoid rebasing or force-pushing to shared branches.
- Use `git commit --fixup <commit>` for fixup commits, and `git rebase -i --autosquash` to squash them before merging.
- Do not push to any remote. Only work locally with git. The user will handle pushing to the remote repository.