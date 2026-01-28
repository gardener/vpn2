// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/openvpn/health"
	"github.com/gardener/vpn2/pkg/utils"
)

func livenessCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "liveness",
		Short: "liveness",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			log, err := utils.InitRun(cmd, Name+"-liveness")
			if err != nil {
				return err
			}
			return runLiveness(log)
		},
	}

	return cmd
}

func runLiveness(log logr.Logger) error {
	cfg, err := config.GetVPNServerConfig(log)
	if err != nil {
		return fmt.Errorf("could not parse environment")
	}
	healthCfg := health.NewDefaultConfig()
	healthCfg.OpenVPNStatusPath = cfg.StatusPath
	healthCfg.IsHA = cfg.IsHA

	if !health.IsAlive(healthCfg, log) {
		os.Exit(1)
	}
	return nil
}
