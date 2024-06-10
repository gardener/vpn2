// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package openvpn

import (
	"bytes"
	"fmt"
	"os"
	"path"

	"github.com/gardener/vpn2/pkg/network"
)

var seedServerConfigTemplate = `
mode server
tls-server
topology subnet

# Additional optimizations
txqueuelen 1000

data-ciphers AES-256-GCM:AES-256-CBC

# port can always be 1194 here as it is not visible externally. A different
# port can be configured for the external load balancer in the service
# manifest
port 1194

keepalive 10 60

# client-config-dir to push client specific configuration
client-config-dir /client-config-dir

key "/srv/secrets/vpn-server/tls.key"
cert "/srv/secrets/vpn-server/tls.crt"
ca "/srv/secrets/vpn-server/ca.crt"
dh none

auth SHA256
tls-auth "/srv/secrets/tlsauth/vpn.tlsauth" 0

{{ if (eq .IPFamilies "IPv4") }}
proto tcp4-server
server {{ printf "%s" .OpenVPNNetwork.IP }} {{ cidrMask .OpenVPNNetwork }} nopool
ifconfig-pool {{ .IPv4PoolStartIP }} {{ .IPv4PoolEndIP }}

{{- range .ShootNetworks }}
route {{ printf "%s" .IP }} {{ cidrMask . }}
{{- end }}
{{- end }}

{{- if (eq .IPFamilies "IPv6") }} proto tcp6-server
server-ipv6 {{ printf "%s" .OpenVPNNetwork }}

{{- range .ShootNetworks }}
route-ipv6 {{ printf "%s" . }}
{{- end }}
{{- end }}

{{- if .IsHA }}

client-to-client
duplicate-cn
{{- end }}

dev {{ .Device }}

{{/* Add firewall rules to block all traffic originating from the shoot cluster.
     The scripts are run after the tun device has been created (up) or removed (down). */ -}}
script-security 2
up "/bin/seed-server firewall --mode up --device {{ .Device }}"
down "/bin/seed-server firewall --mode down --device {{ .Device }}"

{{ if not (eq .StatusPath "") -}}
status {{ .StatusPath }} 15
status-version 2
{{- end -}}
`

var configFromServerForClientTemplate = `
{{- if (eq .IPFamilies  "IPv4") }}
{{- range .ShootNetworks }}
iroute {{ printf "%s" .IP }} {{ cidrMask . }}
{{- end }}
{{- end }}

{{- if (eq .IPFamilies "IPv6") }}
{{- range .ShootNetworks }}
iroute-ipv6 {{ printf "%s" . }}
{{- end }}
{{- end }}
`

var configFromServerForClientHATemplate = `
ifconfig-push {{ .StartIP }} {{ cidrMask .OpenVPNNetwork }}
`

type SeedServerValues struct {
	Device          string
	IPv4PoolStartIP string
	IPv4PoolEndIP   string
	IPFamilies      string
	StatusPath      string
	OpenVPNNetwork  network.CIDR
	ShootNetworks   []network.CIDR
	HAVPNClients    int
	IsHA            bool
	VPNIndex        int
	LocalNodeIP     string
}

func generateSeedServerConfig(cfg SeedServerValues) (string, error) {
	buf := &bytes.Buffer{}
	if err := executeTemplate("openvpn.cfg", buf, seedServerConfigTemplate, &cfg); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// generateConfigForClientFromServer generates the config that the server sends to non HA shoot vpn clients
func generateConfigForClientFromServer(cfg SeedServerValues) (string, error) {
	buf := &bytes.Buffer{}
	if err := executeTemplate("vpn-shoot-client", buf, configFromServerForClientTemplate, &cfg); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// generateConfigForClientHAFromServer generates the config that the server sends to HA shoot vpn clients
func generateConfigForClientHAFromServer(cfg SeedServerValues, startIP string) (string, error) {
	buf := &bytes.Buffer{}
	data := map[string]any{"OpenVPNNetwork": cfg.OpenVPNNetwork, "StartIP": startIP}
	if err := executeTemplate("vpn-shoot-client-ha", buf, configFromServerForClientHATemplate, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

const (
	openvpnClientConfigDir    = "/client-config-dir"
	openvpnClientConfigPrefix = "vpn-shoot-client"
)

func WriteServerConfigFiles(v SeedServerValues) error {
	openvpnConfig, err := generateSeedServerConfig(v)
	if err != nil {
		return fmt.Errorf("error %w: Could not generate openvpn config from %v", err, v)
	}
	if err := os.WriteFile(defaultOpenVPNConfigFile, []byte(openvpnConfig), 0o644); err != nil {
		return err
	}

	vpnShootClientConfig, err := generateConfigForClientFromServer(v)
	if err != nil {
		return fmt.Errorf("error %w: Could not generate shoot client config from %v", err, v)
	}
	err = os.Mkdir(openvpnClientConfigDir, 0750)
	if err != nil && !os.IsExist(err) {
		return err
	}
	if err := os.WriteFile(path.Join(openvpnClientConfigDir, openvpnClientConfigPrefix), []byte(vpnShootClientConfig), 0o644); err != nil {
		return err
	}

	if v.IsHA {
		for i := 0; i < v.HAVPNClients; i++ {
			startIP := v.OpenVPNNetwork.IP
			startIP[3] = byte(v.VPNIndex*64 + i + 2)
			vpnShootClientConfigHA, err := generateConfigForClientHAFromServer(v, startIP.String())
			if err != nil {
				return fmt.Errorf("error %w: Could not generate ha shoot client config %d from %v", err, i, v)
			}
			if err := os.WriteFile(fmt.Sprintf("%s-%d", path.Join(openvpnClientConfigDir, openvpnClientConfigPrefix), i), []byte(vpnShootClientConfigHA), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}
