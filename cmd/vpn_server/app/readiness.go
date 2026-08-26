// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
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

func readinessCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "readiness",
		Short: "readiness",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			log, err := utils.InitRun(cmd, Name+"-readiness")
			if err != nil {
				return err
			}
			return runReadiness(log)
		},
	}

	return cmd
}

func runReadiness(log logr.Logger) error {
	cfg, err := config.GetVPNServerConfig(log)
	if err != nil {
		return fmt.Errorf("could not parse environment")
	}
	healthCfg := health.NewDefaultConfig()
	healthCfg.OpenVPNStatusPath = cfg.StatusPath
	healthCfg.IsHA = cfg.IsHA

	if !health.IsReady(healthCfg, log) {
		os.Exit(1)
	}
	return nil
}
