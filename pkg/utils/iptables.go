// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/coreos/go-iptables/iptables"
	"github.com/go-logr/logr"
)

// NewIPTables wraps the creation of IPTables to patch the path to the correct implementation binary.
// It has been introduced to avoid the risk that the command doesn't work due to missing kernel modules.
func NewIPTables(log logr.Logger, proto iptables.Protocol) (*iptables.IPTables, error) {
	for _, suffix := range []string{"legacy", "nft"} {
		if path := "/sbin/iptables-" + suffix; iptablesWorks(path) {
			log.Info("using iptables backend " + suffix)
			return iptables.New(iptables.IPFamily(proto), iptables.Path(adjustPath(path, proto)))
		}
	}

	return nil, fmt.Errorf("could not find iptables backend")
}

func adjustPath(path string, proto iptables.Protocol) string {
	if proto == iptables.ProtocolIPv6 {
		return strings.ReplaceAll(path, "iptables-", "ip6tables-")
	}
	return path
}

func iptablesWorks(path string) bool {
	// check both iptables and ip6tables
	return exec.Command(path, "-L").Run() == nil &&
		exec.Command(adjustPath(path, iptables.ProtocolIPv6), "-L").Run() == nil
}
