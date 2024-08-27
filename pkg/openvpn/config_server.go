// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package openvpn

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path"

	"github.com/gardener/vpn2/pkg/network"
)

var (
	//go:embed assets/server-config.template
	seedServerConfigTemplate string
	//go:embed assets/server-for-client-config.template
	configFromServerForClientTemplate string
)

type SeedServerValues struct {
	Device         string
	IPFamily       string
	StatusPath     string
	OpenVPNNetwork network.CIDR
	ShootNetworks  []network.CIDR
	HAVPNClients   int
	IsHA           bool
	VPNIndex       int
	LocalNodeIP    string
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
			if err := os.WriteFile(fmt.Sprintf("%s-%d", path.Join(openvpnClientConfigDir, openvpnClientConfigPrefix), i), []byte(""), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}
