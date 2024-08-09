// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

func TestGetVPNNetworkDefault(t *testing.T) {
	actualCid, err := getVPNNetworkDefault()
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if actualCid.String() != constants.DefaultVPNRangeV6 {
		t.Errorf("Expected CIDR: %v, Actual CIDR: %v", constants.DefaultVPNRangeV6, actualCid)
	}
}
