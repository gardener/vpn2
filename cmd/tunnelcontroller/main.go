// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/gardener/vpn2/cmd/tunnelcontroller/app"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func main() {
	if err := app.NewCommand().ExecuteContext(signals.SetupSignalHandler()); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
