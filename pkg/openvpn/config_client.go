package openvpn

import (
	"bytes"
	"fmt"
	"os"
)

var clientTemplate = `
# don't cache authorization information in memory
auth-nocache

# Additonal optimizations
txqueuelen 1000
# get all routing information from server
pull
data-ciphers AES-256-GCM
tls-client

auth SHA256
tls-auth "/srv/secrets/tlsauth/vpn.tlsauth" 1

# https://openvpn.net/index.php/open-source/documentation/howto.html#mitm
remote-cert-tls server

{{ if (eq .IPFamilies "IPv6") }}
proto tcp6-client
{{- end }}

{{ if (eq .IPFamilies "IPv4") }}
proto tcp4-client
{{- end }}

{{ if (eq .VPNClientIndex -1) }}
key /srv/secrets/vpn-client/tls.key
cert /srv/secrets/vpn-client/tls.crt
ca /srv/secrets/vpn-client/ca.crt
{{ else }}
key /srv/secrets/vpn-client-{{.VPNClientIndex}}/tls.key
cert /srv/secrets/vpn-client-{{.VPNClientIndex}}/tls.crt
ca /srv/secrets/vpn-client-{{.VPNClientIndex}}/ca.crt
{{- end }}

{{ if .IsShootClient }}
http-proxy {{ .Endpoint }} {{.OpenVPNPort}}
http-proxy-option CUSTOM-HEADER Reversed-VPN {{ .ReversedVPNHeader }}
{{- end }}

dev {{ .Device }}
remote {{ .Endpoint }}
`

type ClientValues struct {
	IPFamilies        string
	ReversedVPNHeader string
	Endpoint          string
	OpenVPNPort       int
	VPNClientIndex    int
	IsShootClient     bool
	Device            string
}

func generateClientConfig(cfg ClientValues) (string, error) {
	buf := &bytes.Buffer{}
	if err := executeTemplate("openvpn.cfg", buf, clientTemplate, &cfg); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func writeClientConfigFile(v ClientValues) error {
	openvpnConfig, err := generateClientConfig(v)
	if err != nil {
		return fmt.Errorf("error %w: Could not generate openvpn config from %v", err, v)
	}
	return os.WriteFile(defaultOpenVPNConfigFile, []byte(openvpnConfig), 0o644)
}
