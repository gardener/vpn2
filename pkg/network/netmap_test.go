// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/vpn2/pkg/network"
)

func TestNetwork(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Netmap Suite")
}

var _ = Describe("Netmap", func() {
	type testCase struct {
		description string
		inputIP     string
		subnet      string
		expected    string
		expectErr   bool
		errorMsg    string
	}

	DescribeTable("Netmap function",
		func(tc testCase) {
			result, err := network.Netmap(tc.inputIP, tc.subnet)
			if tc.expectErr {
				Expect(err).To(HaveOccurred())
				if tc.errorMsg != "" {
					Expect(err.Error()).To(ContainSubstring(tc.errorMsg))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(tc.expected))
			}
		},
		Entry("should map IPv4 address correctly", testCase{
			description: "valid IPv4 mapping",
			inputIP:     "192.168.1.100",
			subnet:      "10.0.0.0/8",
			expected:    "10.168.1.100",
		}),
		Entry("should preserve host portion when mapping", testCase{
			description: "preserve host portion",
			inputIP:     "172.16.5.10",
			subnet:      "192.168.0.0/16",
			expected:    "192.168.5.10",
		}),
		Entry("should handle /24 subnet correctly", testCase{
			description: "handle /24 subnet",
			inputIP:     "10.0.1.5",
			subnet:      "172.16.5.0/24",
			expected:    "172.16.5.5",
		}),
		Entry("should fail with invalid IP", testCase{
			description: "invalid IP address",
			inputIP:     "invalid-ip",
			subnet:      "10.0.0.0/8",
			expectErr:   true,
			errorMsg:    "failed to parse ip",
		}),
		Entry("should fail with invalid subnet", testCase{
			description: "invalid subnet",
			inputIP:     "192.168.1.1",
			subnet:      "invalid-subnet",
			expectErr:   true,
			errorMsg:    "failed to parse subnet",
		}),
		Entry("should fail with IPv6 address", testCase{
			description: "IPv6 not supported",
			inputIP:     "2001:db8::1",
			subnet:      "10.0.0.0/8",
			expectErr:   true,
			errorMsg:    "only IPv4 is supported",
		}),
		Entry("should fail with IPv6 subnet", testCase{
			description: "IPv6 subnet not supported",
			inputIP:     "192.168.1.1",
			subnet:      "2001:db8::/32",
			expectErr:   true,
			errorMsg:    "only IPv4 is supported",
		}),
	)
})
