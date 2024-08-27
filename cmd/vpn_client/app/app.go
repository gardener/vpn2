// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/gardener/vpn2/cmd/vpn_client/app/pathcontroller"
	"github.com/gardener/vpn2/cmd/vpn_client/app/setup"
	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/openvpn"
	"github.com/gardener/vpn2/pkg/pprof"
	"github.com/gardener/vpn2/pkg/utils"
	"github.com/gardener/vpn2/pkg/vpn_client"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"k8s.io/component-base/version/verflag"
)

// Name is a const for the name of this component.
const Name = "vpn-client"

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
	cmd.PersistentFlags().BoolVar(&pprofEnabled, "enable-pprof", false, "enable pprof for profiling")
	cmd.AddCommand(pathcontroller.NewCommand())
	cmd.AddCommand(setup.NewCommand())
	return cmd
}

func vpnConfig(log logr.Logger, cfg config.VPNClient) openvpn.ClientValues {
	v := openvpn.ClientValues{
		Device:            "tun0",
		IPFamily:          cfg.PrimaryIPFamily(),
		ReversedVPNHeader: cfg.ReversedVPNHeader,
		Endpoint:          cfg.Endpoint,
		OpenVPNPort:       cfg.OpenVPNPort,
		VPNClientIndex:    cfg.VPNClientIndex,
		IsShootClient:     cfg.IsShootClient,
		IsHA:              cfg.IsHA,
		SeedPodNetwork:    cfg.SeedPodNetwork.String(),
	}
	vpnSeedServer := "vpn-seed-server"

	if cfg.VPNServerIndex != "" {
		vpnSeedServer = fmt.Sprintf("vpn-seed-server-%s", cfg.VPNServerIndex)
		v.Device = fmt.Sprintf("tap%s", cfg.VPNServerIndex)
	}

	log.Info("Built config values", "vpn-seed-sever", vpnSeedServer, "values", v)
	return v
}

func run(_ context.Context, log logr.Logger) error {
	cfg, err := config.GetVPNClientConfig()
	if err != nil {
		return err
	}
	log.Info("config parsed", "config", cfg)

	err = vpn_client.SetIPTableRules(log, cfg)
	if err != nil {
		return err
	}

	values := vpnConfig(log, cfg)

	return openvpn.WriteClientConfigFile(values)
}
