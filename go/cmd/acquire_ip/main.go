/*
 * SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/gardener/vpn2/ippool"
)

func mustGetEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic(fmt.Errorf("missing env variable '%s'", name))
	}
	return value
}

func optionalGetEnv(name, defaultValue string) string {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	return value
}

func optionalGetEnvInt(name string, defaultValue, min, max int) int {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(value)
	if err != nil || v < min || v > max {
		panic(fmt.Errorf("invalid value for %s: %s (min=%d,max=%d)", name, value, min, max))
	}
	return v
}

// newIPAddressBrokerFromEnv initialises the broker with values from env and for in-cluster usage.
func newIPAddressBrokerFromEnv() (ippool.IPAddressBroker, error) {
	podName := mustGetEnv("POD_NAME")
	namespace := mustGetEnv("NAMESPACE")

	vpnNetworkString := optionalGetEnv("VPN_NETWORK", "192.168.123.0/24")
	base, _, err := net.ParseCIDR(vpnNetworkString)
	if err != nil {
		return nil, fmt.Errorf("invalid VPN_NETWORK: %w", err)
	}
	if base.To4() == nil {
		return nil, fmt.Errorf("invalid VPN_NETWORK %q, must be an IPv4 network", vpnNetworkString)
	}
	if base.To4()[3] != 0 {
		return nil, fmt.Errorf("invalid VPN_NETWORK %q, last octet must be 0", vpnNetworkString)
	}

	startIndex := optionalGetEnvInt("START_INDEX", 200, 2, 254)
	endIndex := optionalGetEnvInt("END_INDEX", 254, startIndex, 254)
	labelSelector := optionalGetEnv("POD_LABEL_SELECTOR", "app=kubernetes,role=apiserver")
	waitSeconds := optionalGetEnvInt("WAIT_SECONDS", 2, 1, 30)
	manager, err := ippool.NewPodIPPoolManager(namespace, labelSelector)
	if err != nil {
		return nil, err
	}
	return ippool.NewIPAddressBroker(manager, base, startIndex, endIndex, podName, time.Duration(waitSeconds)*time.Second)
}

func main() {
	broker, err := newIPAddressBrokerFromEnv()
	if err != nil {
		panic(err)
	}

	ctx := context.TODO()
	ip, err := broker.AcquireIP(ctx)
	if err != nil {
		panic(err)
	}
	if output := os.Getenv("OUTPUT"); output != "" {
		err = os.WriteFile(output, []byte(ip), 0420)
		if err != nil {
			panic(err)
		}
	}
}
