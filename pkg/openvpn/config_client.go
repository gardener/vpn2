// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package openvpn

import (
	_ "embed"

	"bytes"
	"fmt"
	"os"
)

//go:embed assets/client-config.template
var clientTemplate string

type ClientValues struct {
	IPFamily          string
	ReversedVPNHeader string
	Endpoint          string
	OpenVPNPort       int
	VPNClientIndex    int
	IsShootClient     bool
	IsHA              bool
	Device            string
	SeedPodNetwork    string
}

func generateClientConfig(cfg ClientValues) (string, error) {
	buf := &bytes.Buffer{}
	if err := executeTemplate("openvpn.cfg", buf, clientTemplate, &cfg); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func WriteClientConfigFile(v ClientValues) error {
	openvpnConfig, err := generateClientConfig(v)
	if err != nil {
		return fmt.Errorf("error %w: Could not generate openvpn config from %v", err, v)
	}
	return os.WriteFile(defaultOpenVPNConfigFile, []byte(openvpnConfig), 0o644)
}
