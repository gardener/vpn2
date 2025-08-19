// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pathcontroller

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"slices"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/gardener/vpn2/pkg/utils"
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

	checkNetworks := cfg.ShootNodeNetworks
	if len(checkNetworks) == 0 {
		checkNetworks = cfg.ShootServiceNetworks
	}
	if len(checkNetworks) == 0 {
		return errors.New("network to check is undefined")
	}

	netlinkRouter := &netlinkRouter{
		seedPodNetwork:       cfg.SeedPodNetwork,
		shootPodNetworks:     cfg.ShootPodNetworks,
		shootServiceNetworks: cfg.ShootServiceNetworks,
		log:                  log,
	}
	if len(cfg.ShootNodeNetworks) != 0 {
		netlinkRouter.shootNodeNetworks = cfg.ShootNodeNetworks
	}

	podIP := os.Getenv("POD_IP")
	if podIP == "" {
		return fmt.Errorf("POD_IP environment variable not set")
	}

	// Check if there is an overlap between the seed pod network and shoot networks.
	overlap := network.OverLapAny(cfg.SeedPodNetwork, slices.Concat(cfg.ShootPodNetworks, cfg.ShootServiceNetworks, cfg.ShootNodeNetworks)...)

	// map pod IP to 241/8 range if needed
	if net.ParseIP(podIP).To4() != nil && overlap {
		mappedIP, err := network.NetmapIP(podIP, constants.SeedPodNetworkMapped)
		if err != nil {
			log.Info("error mapping pod IP to 241/8 range", "podIP", podIP, "error", err)
			return err
		}
		podIP = mappedIP
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
