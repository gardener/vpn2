// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"github.com/gardener/vpn2/pkg/shoot_client/tunnel"
	"github.com/gardener/vpn2/pkg/utils"
	"github.com/spf13/cobra"
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
			c := tunnel.NewController()
			return c.Run(log)
		},
	}

	return cmd
}
