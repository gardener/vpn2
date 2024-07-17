// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"
)

func TestGetVPNNetworkDefault(t *testing.T) {
	actualCid, err := getVPNNetworkDefault()
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if actualCid.String() != defaultIPV6VpnNetwork {
		t.Errorf("Expected CIDR: %v, Actual CIDR: %v", defaultIPV6VpnNetwork, actualCid)
	}
}
