// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/coreos/go-iptables/iptables"
	"github.com/gardener/vpn2/cmd/vpn_server/app/setup"
	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/gardener/vpn2/pkg/openvpn"
	"github.com/gardener/vpn2/pkg/pprof"
	"github.com/gardener/vpn2/pkg/utils"
	"github.com/gardener/vpn2/pkg/vpn_server"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"k8s.io/component-base/version/verflag"
)

// Name is a const for the name of this component.
const Name = "vpn-server"

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
	cmd.AddCommand(exporterCommand())
	cmd.AddCommand(setup.NewCommand())
	cmd.PersistentFlags().BoolVar(&pprofEnabled, "enable-pprof", false, "enable pprof for profiling")
	return cmd
}

func run(_ context.Context, log logr.Logger) error {
	cfg, err := config.GetVPNServerConfig(log)
	if err != nil {
		return fmt.Errorf("could not parse environment")
	}

	v, err := vpn_server.BuildValues(cfg)
	if err != nil {
		return err
	}

	if !cfg.IsHA {
		log.Info("setting up iptables rules for Envoy")
		ipTable, err := network.NewIPTables(log, iptables.ProtocolIPv4)
		if err != nil {
			return err
		}

		for _, nw := range cfg.PodNetworks {
			if nw.IsIPv4() {
				err = ipTable.AppendUnique("nat", "OUTPUT", "-m", "owner", "--uid-owner", constants.EnvoyNonRootUserId, "-d", nw.String(), "-j", "NETMAP", "--to", constants.ShootPodNetworkMapped)
				if err != nil {
					return err
				}
			}
		}
		for _, nw := range cfg.ServiceNetworks {
			if nw.IsIPv4() {
				err = ipTable.AppendUnique("nat", "OUTPUT", "-m", "owner", "--uid-owner", constants.EnvoyNonRootUserId, "-d", nw.String(), "-j", "NETMAP", "--to", constants.ShootSvcNetworkMapped)
				if err != nil {
					return err
				}
			}
		}
		for _, nw := range cfg.NodeNetworks {
			if nw.IsIPv4() {
				err = ipTable.AppendUnique("nat", "OUTPUT", "-m", "owner", "--uid-owner", constants.EnvoyNonRootUserId, "-d", nw.String(), "-j", "NETMAP", "--to", constants.ShootNodeNetworkMapped)
				if err != nil {
					return err
				}
			}
		}

	}

	log.Info("writing openvpn config file", "values", v)
	return openvpn.WriteServerConfigFiles(v)
}
