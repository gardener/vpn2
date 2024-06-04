// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/gardener/vpn2/pkg/openvpn"
	"github.com/gardener/vpn2/pkg/shoot_client"

	"github.com/gardener/vpn2/cmd/shoot_client/app/pathcontroller"
	"github.com/gardener/vpn2/cmd/shoot_client/app/setup"
	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/utils"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"k8s.io/component-base/version/verflag"
)

// Name is a const for the name of this component.
const Name = "shoot-client"

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
			ctx, cancel := context.WithCancel(cmd.Context())
			return run(ctx, cancel, log)
		},
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	cmd.AddCommand(pathcontroller.NewCommand())
	cmd.AddCommand(setup.NewCommand())
	return cmd
}

func vpnConfig(log logr.Logger, cfg config.ShootClient) openvpn.ClientValues {
	v := openvpn.ClientValues{
		Device:            "tun0",
		IPFamilies:        cfg.IPFamilies,
		ReversedVPNHeader: cfg.ReversedVPNHeader,
		Endpoint:          cfg.Endpoint,
		OpenVPNPort:       cfg.OpenVPNPort,
		VPNClientIndex:    cfg.VPNClientIndex,
		IsShootClient:     cfg.IsShootClient,
	}
	vpnSeedServer := "vpn-seed-server"

	if cfg.VPNServerIndex != "" {
		vpnSeedServer = fmt.Sprintf("vpn-seed-server-%s", cfg.VPNServerIndex)
		v.Device = fmt.Sprintf("tap%s", cfg.VPNServerIndex)
	}

	log.Info("Built config values", "vpn-seed-sever", vpnSeedServer, "values", v)
	return v
}

func run(_ context.Context, _ context.CancelFunc, log logr.Logger) error {
	cfg, err := config.GetShootClientConfig()
	if err != nil {
		return err
	}
	log.Info("config parsed", "config", cfg)

	err = shoot_client.SetIPTableRules(cfg)
	if err != nil {
		return err
	}

	values := vpnConfig(log, cfg)
	return openvpn.WriteClientConfigFile(values)
}
