// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"context"

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
		err = vpn_client.ConfigureBonding(ctx, log, &cfg)
		if err != nil {
			return err
		}
	}
	return nil
}
