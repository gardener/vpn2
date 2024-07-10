// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"os/exec"
	"strings"
	"unsafe"

	"github.com/coreos/go-iptables/iptables"
	"github.com/go-logr/logr"
)

type ipTables struct {
	path              string
	proto             iptables.Protocol
	hasCheck          bool
	hasWait           bool
	waitSupportSecond bool
	hasRandomFully    bool
	v1                int
	v2                int
	v3                int
	mode              string // the underlying iptables operating mode, e.g. nf_tables
	timeout           int    // time to wait for the iptables lock, default waits forever
}

// NewIPTables wraps the creation of IPTables to patch the path to the correct implementation binary.
// It has been introduced to avoid the risk that the command doesn't work due to missing kernel modules.
func NewIPTables(log logr.Logger, proto iptables.Protocol) (*iptables.IPTables, error) {
	t, err := iptables.New()
	if err != nil {
		return nil, err
	}

	t2 := (*ipTables)(unsafe.Pointer(t))
	if path := t2.path + "-legacy"; iptablesWorks(path) {
		log.Info("using iptables backend legacy")
		t2.mode = "legacy"
		t2.path = adjustPath(path, proto)
		t2.proto = proto
		return t, nil
	}

	if path := t2.path + "-nft"; iptablesWorks(path) {
		log.Info("using iptables backend nf_tables")
		t2.mode = "nf_tables"
		t2.path = adjustPath(path, proto)
		t2.proto = proto
		return t, nil
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
	return exec.Command(path, "-L").Run() == nil &&
		exec.Command(adjustPath(path, iptables.ProtocolIPv6), "-L").Run() == nil
}
