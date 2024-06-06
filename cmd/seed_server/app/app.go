// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/openvpn"
	"github.com/gardener/vpn2/pkg/pprof"
	"github.com/gardener/vpn2/pkg/seed_server"
	"github.com/gardener/vpn2/pkg/seed_server/openvpn_exporter"
	"github.com/gardener/vpn2/pkg/utils"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"k8s.io/component-base/version/verflag"
)

// Name is a const for the name of this component.
const Name = "seed-server"

const (
	metricsPort = 15000
)

var pprofEnabled bool

// NewCommand creates a new cobra.Command for running gardener-node-agent.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   Name,
		Short: "Launch the " + Name,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			log, err := utils.InitRun(cmd, Name)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if pprofEnabled {
				go pprof.Serve(ctx, log.WithName("pprof"))
			}
			return run(ctx, log)
		},
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	cmd.AddCommand(firewallCommand())
	cmd.PersistentFlags().BoolVar(&pprofEnabled, "enable-pprof", false, "enable pprof for profiling")
	return cmd
}

func run(ctx context.Context, log logr.Logger) error {
	cfg, err := config.GetSeedServerConfig(log)
	if err != nil {
		return fmt.Errorf("could not parse environment")
	}

	v, err := seed_server.BuildValues(cfg)
	if err != nil {
		return err
	}

	log.Info("using openvpn network", "openVPNNetwork", v.OpenVPNNetwork)
	openVPN, err := openvpn.NewServer(v)
	if err != nil {
		return fmt.Errorf("error creating openvpn server: %w", err)
	}

	if cfg.StatusPath != "" {
		exporterConfig := openvpn_exporter.NewDefaultConfig()
		exporterConfig.OpenvpnStatusPaths = cfg.StatusPath
		exporterConfig.ListenAddress = fmt.Sprintf(":%d", metricsPort)
		if err := openvpn_exporter.Start(log, exporterConfig); err != nil {
			return fmt.Errorf("starting metrics exporter failed: %w", err)
		}
	}

	return openVPN.Run(ctx)
}
