// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pathcontroller

import (
	"context"
	"errors"
	"net"
	"os"
	"time"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/shoot_client/tunnel"

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

	if err = network.ValidateCIDR(cfg.VPNNetwork, cfg.IPFamilies); err != nil {
		return err
	}
	checkNetwork := cfg.NodeNetwork
	if checkNetwork.String() == "" {
		checkNetwork = cfg.ServiceNetwork
	}
	if checkNetwork.String() == "" {
		return errors.New("network to check is undefined")
	}

	netlinkRouter := &netlinkRouter{
		podNetwork:     (*net.IPNet)(&cfg.PodNetwork),
		serviceNetwork: (*net.IPNet)(&cfg.ServiceNetwork),
	}
	if cfg.NodeNetwork.String() != "" {
		netlinkRouter.nodeNetwork = (*net.IPNet)(&cfg.NodeNetwork)
	}

	router := &clientRouter{
		pinger: icmpPinger{
			log:     log.WithName("ping"),
			timeout: 2 * time.Second,
			retries: 1,
		},
		ticker:       time.NewTicker(2 * time.Second),
		tunnelConfig: tunnel.IP6Tunnel{KubeAPIServerPodIP: os.Getenv("POD_IP")},
		netRouter:    netlinkRouter,
		checkedNet:   checkNetwork.ToIPNet(),
		goodIPs:      make(map[string]struct{}),
		log:          log.WithName("pingRouter"),
	}

	// acquired ip is not necessary here, because we don't care about the subnet
	_, clientIPs := network.GetBondAddressAndTargetsSeedClient(nil, cfg.VPNNetwork.ToIPNet(), cfg.HAVPNClients)
	return router.Run(ctx, clientIPs)
}
