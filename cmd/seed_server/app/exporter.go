// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/openvpn/exporter"
	"github.com/gardener/vpn2/pkg/utils"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
)

func exporterCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exporter",
		Short: "exporter",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			log, err := utils.InitRun(cmd, Name+"-exporter")
			if err != nil {
				return err
			}
			return runExporter(log)
		},
	}

	return cmd
}

func runExporter(log logr.Logger) error {
	cfg, err := config.GetSeedServerConfig(log)
	if err != nil {
		return fmt.Errorf("could not parse environment")
	}
	exporterConfig := exporter.NewDefaultConfig()
	exporterConfig.OpenvpnStatusPaths = cfg.StatusPath
	exporterConfig.ListenAddress = fmt.Sprintf(":%d", metricsPort)
	if err := exporter.Start(log, exporterConfig); err != nil {
		return fmt.Errorf("starting metrics exporter failed: %w", err)
	}
	return nil
}
