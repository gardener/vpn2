# VPN2

[![REUSE status](https://api.reuse.software/badge/github.com/gardener/vpn2)](https://api.reuse.software/info/github.com/gardener/vpn2)

This repository contains components to establish network connectivity for Shoot clusters.

## What's inside

[VPN Seed Server](seed-server) - a component that serves an endpoint for incoming connections and allows contacting any IP address within the networks of a Shoot cluster (which are usually private).

[VPN Shoot Client](shoot-client) - a component that establishes connectivity from a Shoot cluster to the endpoint in the Seed cluster allowing contacting any IP address within its network and routes the packets back to the caller.

## Local test environment

- Set up a local Gardener environment following [Deploying Gardener Locally](https://github.com/gardener/gardener/blob/master/docs/deployment/getting_started_locally.md)
  
  Essential step:

  ```bash
  make kind-up gardener-up
  ```

- Follow [Creating a Shoot Cluster](https://github.com/gardener/gardener/blob/master/docs/deployment/getting_started_locally.md#creating-a-shoot-cluster) to create a shoot cluster

  Essential step:

  ```bash
  kubectl apply -f example/provider-local/shoot.yaml
  ```

- If you want to test the VPN in HA mode, annotate the shoot with

  ```bash
  kubectl -n garden-local annotate shoot local alpha.control-plane.shoot.gardener.cloud/high-availability-vpn=true
  ```

- build the docker images and push them to the local Gardener registry with

  **Shoot client image**

  ```bash
  make shoot-client-to-gardener-local
  ```

  or to build a debug image containing command line tools like ls, tcpdump and other net-tools use `make shoot-client-to-gardener-local DEBUG=true`

  *Note: Remember the image name reported at the end similar to*  

  ```txt
  shoot client image: localhost:5001/europe-docker_pkg_dev_gardener-project_public_gardener_vpn-shoot-client-go:0.26.0-dev
  ```

  **Seed server image**

  ```bash
  make seed-server-to-gardener-local
  ```

  or to build a debug image containing command line tools like ls, tcpdump and other net-tools use `make seed-server-to-gardener-local DEBUG=true`

  *Note: Remember the image name reported at the end similar to*

  ```txt
  seed server image: localhost:5001/europe-docker_pkg_dev_gardener-project_public_gardener_vpn-seed-server-go:0.26.0-dev
  ```

- adjust the image vector file `imagevector/images.yaml` in Gardener to image repositories and tags provided by the last step

  ```
  ...
  - name: vpn-seed-server
  sourceRepository: github.com/gardener/vpn2
  repository: localhost:5001/europe-docker_pkg_dev_gardener-project_public_gardener_vpn-seed-server-go
  tag: 0.26.0-dev
  ...
  - name: vpn-shoot-client
  sourceRepository: github.com/gardener/vpn2
  repository: localhost:5001/europe-docker_pkg_dev_gardener-project_public_gardener_vpn-shoot-client-go
  tag: 0.26.0-dev
  ...
  ```

  and rerun `make gardener-up`

- if you rebuild these images you may to have to ensure that the new images are loaded are pulled by the deployments
  - for the vpn-seed-server deployment in the control plane, execute

    ```bash
    kubectl -n shoot--local--local patch deploy vpn-seed-server --patch '{"spec":{"template":{"spec":{"containers":[{"name":"vpn-seed-server","imagePullPolicy":"Always"}]}}}}' 
    ```

  - for the shoot client deployment in the shoot, execute

    ```bash
    # first disable managed resource updates in the control plane
    kubectl -n shoot--local--local annotate managedresource shoot-core-vpn-shoot resources.gardener.cloud/ignore=true
    # then target the shoot cluster
    kubectl --kubeconfig admin-kubeconf.yaml -n kube-system patch deploy vpn-shoot --patch '{"spec":{"template":{"spec":{"initContainers":[{"name":"vpn-shoot-init","imagePullPolicy":"Always"}],"containers":[{"name":"vpn-shoot","imagePullPolicy":"Always"}]}}}}' 
    ```

  - for the HA deployments run

    ```bash
    kubectl -n shoot--local--local patch deploy kube-apiserver --patch '{"spec":{"template":{"spec":{"initContainers":[{"name":"vpn-client-init","imagePullPolicy":"Always"}]},"containers":[{"name":"vpn-client-0","imagePullPolicy":"Always"}, {"name":"vpn-client-1","imagePullPolicy":"Always"}, {"name":"vpn-path-controller","imagePullPolicy":"Always"}]}}}' 
    kubectl -n shoot--local--local patch sts vpn-seed-server --patch '{"spec":{"template":{"spec":{"containers":[{"name":"vpn-seed-server","imagePullPolicy":"Always"},{"name":"openvpn-exporter","imagePullPolicy":"Always"}]}}}}' 
    ```

    and

    ```bash
    # first disable managed resource updates in the control plane
    kubectl -n shoot--local--local annotate managedresource shoot-core-vpn-shoot resources.gardener.cloud/ignore=true
    # then target the shoot cluster
    kubectl --kubeconfig admin-kubeconf.yaml -n kube-system patch sts vpn-shoot --patch '{"spec":{"template":{"spec":{"initContainers":[{"name":"vpn-shoot-init","imagePullPolicy":"Always"}]}}}}' 
    ```

### Accessing shoot VPN container logs if VPN is down

In case the VPN inside the shoot is not able to connect, it's not possible to stream logs from the Kubernetes API.
To get the logs, you can query the machine of the cluster directly for the logs. To do so, use the following command:

```bash
MACHINE_POD=$(kubectl get machines -n shoot--local--local -l name=shoot--local--local-local -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n shoot--local--local pod/machine-$MACHINE_POD -ti -- bash -c 'tail -f /var/log/pods/kube-system_vpn-shoot-*/vpn-shoot-*/0.log'
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

For more information, see <https://www.kernelconfig.io/config_bonding?q=&kernelversion=6.1.90&arch=x86>
