// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// InitRun initializes the run command by creating and settings a logger,
// printing all command line flags, and configuring command settings.
func InitRun(cmd *cobra.Command, name string) (logr.Logger, error) {
	verflag.PrintAndExitIfRequested()

	logLevel := "info"
	logFormat := "text"
	log, err := logger.NewZapLogger(logLevel, logFormat)
	if err != nil {
		return logr.Discard(), fmt.Errorf("error instantiating zap logger: %w", err)
	}

	logf.SetLogger(log)
	klog.SetLogger(log)

	log.Info("Starting "+name, "version", version.Get()) //nolint:logcheck

	// don't output usage on further errors raised during execution
	cmd.SilenceUsage = true
	// further errors will be logged properly, don't duplicate
	cmd.SilenceErrors = true

	return log, nil
}
