# VPN Shoot Client

The VPN Shoot Client is a tool that is used for [Shoot clusters](https://github.com/gardener/documentation/wiki/Architecture). It connects to the vpn server running in the seed cluster, but allows contacting any IP address within its network and routes the packets back to the caller (usually through the [vpn-seed-server](../seed-server)). By that, it connects the components running in the Seed cluster with those running in the Shoot cluster.

## Constraints

The `seed-server` requires a load balancer pointing to it which must be reachable from the Shoot cluster network (usually a public load balancer). The `shoot-client` connects to the seed cluster through this load balancer.

## How to build it?

:warning: Please don't forget to update the `$VERSION` variable in the `Makefile` before creating a new release:

```bash
make release
```

This will create a new Docker image with the tag you specified in the `Makefile`, push it to our image registry, and clean up afterwards.

## Example manifests

Please find an example Kubernetes manifest within the [`/example`](example) directory.

These are a few sample commands for generating keys and certificates

```bash
# generate the DH parameters file for the server
openssl dhparam -out dh2048.pem 2048

# generate ca
openssl genrsa -out rootCA.key 4096
openssl req -x509 -new -nodes -key rootCA.key -sha256 -days 1024 -out rootCA.crt -subj "/CN=mycat"

# create keys for client and server
openssl genrsa -out client-key.pem 2048
openssl genrsa -out server-key.pem 2048

# create a signing request and sign the client key
openssl req -new -key client-key.pem -out client-key.csr -subj "/CN=myclient"
openssl x509 -req -in client-key.csr -CA rootCA.crt -CAkey rootCA.key -CAcreateserial -out client-cert.pem -days 500 -sha256

# create a signing request and sign the server key
openssl req -new -key server-key.pem -out server-key.csr -subj "/CN=myserver"
openssl x509 -req -in server-key.csr -CA rootCA.crt -CAkey rootCA.key -CAcreateserial -out server-cert.pem -days 500 -sha256

# generate tls auth key (see )
openvpn --genkey --secret vpn.tlsauth
```
