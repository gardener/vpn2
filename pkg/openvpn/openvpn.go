// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package openvpn

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
)

type OpenVPN struct {
	configFilePath string
	filterFunc     func(stdout io.ReadCloser) error
}

func NewServer(values SeedServerValues) (*OpenVPN, error) {
	err := writeServerConfigFiles(values)
	if err != nil {
		return nil, err
	}
	return &OpenVPN{
		filterFunc:     healthcheckFilter(values.LocalNodeIP),
		configFilePath: defaultOpenVPNConfigFile,
	}, nil
}

func NewClient(values ClientValues) (*OpenVPN, error) {
	err := WriteClientConfigFile(values)
	if err != nil {
		return nil, err
	}
	return &OpenVPN{
		configFilePath: defaultOpenVPNConfigFile,
	}, nil
}

func (o *OpenVPN) Run(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "openvpn", "--config", o.configFilePath)
	// stderr and stdout need to be set before calling start
	var openvpnStdout io.ReadCloser
	if o.filterFunc == nil {
		cmd.Stdout = os.Stdout
	} else {
		var err error
		openvpnStdout, err = cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("could not connect to stdout of openvpn command")
		}
	}
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("could not start openvpn command, %w", err)
	}
	defer cmd.Wait()
	if o.filterFunc != nil {
		return o.filterFunc(openvpnStdout)
	}
	return nil
}

func healthcheckFilter(localNodeIP string) func(stdout io.ReadCloser) error {
	filterRegex := regexp.MustCompile(fmt.Sprintf(`(TCP connection established with \[AF_INET(6)?\]%s|)?%s(:[0-9]{1,5})? Connection reset, restarting`,
		localNodeIP, localNodeIP))
	return func(stdout io.ReadCloser) error {
		scanner := bufio.NewScanner(stdout)

		for scanner.Scan() {
			line := scanner.Bytes()
			if !filterRegex.Match(line) {
				_, _ = os.Stdout.Write(line)
				_, _ = os.Stdout.Write([]byte("\n"))
			}
		}
		return scanner.Err()
	}
}
