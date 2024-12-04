// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pathcontroller

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/gardener/vpn2/pkg/utils"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
)

const Name = "path-controller"

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
	cfg, err := config.GetPathControllerConfig(log)
	if err != nil {
		return err
	}

	checkNetworks := cfg.NodeNetworks
	if len(checkNetworks) == 0 {
		checkNetworks = cfg.ServiceNetworks
	}
	if len(checkNetworks) == 0 {
		return errors.New("network to check is undefined")
	}

	netlinkRouter := &netlinkRouter{
		podNetworks:     cfg.PodNetworks,
		serviceNetworks: cfg.ServiceNetworks,
		log:             log,
	}
	if len(cfg.NodeNetworks) != 0 {
		netlinkRouter.nodeNetworks = cfg.NodeNetworks
	}

	podIP := os.Getenv("POD_IP")
	if podIP == "" {
		return fmt.Errorf("POD_IP environment variable not set")
	}
	router := &clientRouter{
		pinger: &icmpPinger{
			log:     log.WithName("ping"),
			timeout: 2 * time.Second,
			retries: 1,
		},
		ticker:             time.NewTicker(2 * time.Second),
		kubeAPIServerPodIP: podIP,
		netRouter:          netlinkRouter,
		checkedNet:         checkNetworks[0].ToIPNet(),
		goodIPs:            make(map[string]struct{}),
		log:                log.WithName("pingRouter"),
	}

	// acquired ip is not necessary here, because we don't care about the subnet
	clientIPs := network.AllBondingShootClientIPs(cfg.VPNNetwork.ToIPNet(), cfg.HAVPNClients)
	return router.Run(ctx, clientIPs)
}
