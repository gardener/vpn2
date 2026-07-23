// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tunnelcontroller

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/shoot_client/tunnel"
	"github.com/gardener/vpn2/pkg/utils"
)

const Name = "tunnel-controller"

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
			ctx := cmd.Context()
			return run(ctx, log)
		},
	}

	return cmd
}

func runReadinessServer(c *tunnel.Controller, log logr.Logger) {
	go func() {
		log.Info("Starting readiness server", "port", tunnel.ReadinessPort)
		err := c.NewReadinessServer().ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error(err, "readiness server stopped with error")
		}
	}()
}

func run(ctx context.Context, log logr.Logger) error {
	cfg, err := config.GetTunnelControllerConfig(log)
	if err != nil {
		return err
	}

	c := tunnel.NewController(cfg)
	runReadinessServer(c, log)

	return c.Run(ctx, log)
}
