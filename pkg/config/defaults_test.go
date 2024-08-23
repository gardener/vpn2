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

	want := "fd8f:6d53:b97a:1::/112"
	if actualCid.String() != want {
		t.Errorf("Expected CIDR: %v, Actual CIDR: %v", want, actualCid)
	}
}
