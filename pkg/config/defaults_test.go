// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"
)

func TestGetVPNNetworkDefault(t *testing.T) {
	tests := []struct {
		name        string
		ipFamily    string
		expectedCid string
		expectedErr error
	}{
		{
			name:        "should return correct CIDR for IPv4",
			ipFamily:    IPv4Family,
			expectedCid: defaultIPV4VpnNetwork,
			expectedErr: nil,
		},
		{
			name:        "should return correct CIDR for IPv6",
			ipFamily:    IPv6Family,
			expectedCid: defaultIPV6VpnNetwork,
			expectedErr: nil,
		},
		{
			name:        "should return error for unknown IPFamily",
			ipFamily:    "unknown",
			expectedCid: "",
			expectedErr: ErrorInvalidIPFamily,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualCid, actualErr := getVPNNetworkDefault(test.ipFamily)
			if actualErr != test.expectedErr {
				t.Errorf("expected error %v, got %v", test.expectedErr, actualErr)
			}

			if actualCid.String() != test.expectedCid {
				t.Errorf("Expected CIDR: %v, Actual CIDR: %v", test.expectedCid, actualCid)
			}
		})
	}
}
