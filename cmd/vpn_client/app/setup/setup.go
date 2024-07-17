// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"context"
	"fmt"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/utils"
	"github.com/gardener/vpn2/pkg/vpn_client"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
)

const Name = "setup"

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   Name,
		Short: Name,
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

	return cmd
}

func run(ctx context.Context, _ context.CancelFunc, log logr.Logger) error {
	cfg, err := config.GetVPNClientConfig()
	if err != nil {
		return err
	}
	log.Info("config parsed", "config", cfg)

	err = vpn_client.KernelSettings(cfg)
	if err != nil {
		return err
	}

	if cfg.IsHA {
		if cfg.IPFamilies != "IPv4" {
			// TODO (martinweindel) HA VPN for IPv6 has issued with pings and neighbor solicitiation on the gardener local setup needs further investigations
			return fmt.Errorf("the highly-available VPN setup is only supported for IPv4 single-stack shoots")
		}
		err = vpn_client.ConfigureBonding(ctx, log, &cfg)
		if err != nil {
			return err
		}
	}
	return nil
}
