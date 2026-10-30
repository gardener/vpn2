// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/openvpn/exporter"
	"github.com/gardener/vpn2/pkg/utils"
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
			ctx := cmd.Context()
			return runExporter(ctx, log)
		},
	}

	return cmd
}

func runExporter(ctx context.Context, log logr.Logger) error {
	cfg, err := config.GetVPNServerConfig(log)
	if err != nil {
		return fmt.Errorf("could not parse environment")
	}
	exporterConfig := exporter.NewDefaultConfig()
	exporterConfig.OpenvpnStatusPaths = cfg.StatusPath
	exporterConfig.ListenAddress = fmt.Sprintf(":%d", metricsPort)
	if err := exporter.Start(ctx, log, exporterConfig); err != nil {
		return fmt.Errorf("starting metrics exporter failed: %w", err)
	}
	return nil
}
