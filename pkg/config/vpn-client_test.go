package config_test

import (
	"maps"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/network"
)

func TestVPNClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VPNClient Suite")
}

var _ = Describe("GetVPNClientConfig", func() {
	var (
		defaultEnvVars map[string]string
	)

	BeforeEach(func() {
		// Set default environment variables
		defaultEnvVars = map[string]string{
			"TCP_KEEPALIVE_TIME":     "7200",
			"TCP_KEEPALIVE_INTVL":    "75",
			"TCP_KEEPALIVE_PROBES":   "9",
			"IP_FAMILIES":            "IPv4",
			"ENDPOINT":               "endpoint",
			"OPENVPN_PORT":           "8132",
			"VPN_NETWORK":            "fd8f:6d53:b97a:1::/96",
			"SEED_POD_NETWORK_V4":    "10.0.0.0/8",
			"SHOOT_SERVICE_NETWORKS": "100.64.0.0/13",
			"SHOOT_POD_NETWORKS":     "100.96.0.0/11",
			"SHOOT_NODE_NETWORKS":    "100.128.0.0/10",
			"IS_SHOOT_CLIENT":        "true",
			"POD_NAME":               "test-pod-1",
			"NAMESPACE":              "test-namespace",
			"VPN_SERVER_INDEX":       "1",
			"IS_HA":                  "true",
			"REVERSED_VPN_HEADER":    "invalid-host",
			"HA_VPN_CLIENTS":         "3",
			"HA_VPN_SERVERS":         "3",
			"POD_LABEL_SELECTOR":     "app=kubernetes,role=apiserver",
			"WAIT_TIME":              "2s",
		}
	})

	AfterEach(func() {
		// Clean up environment variables after each test
		Expect(os.Unsetenv("TCP_KEEPALIVE_TIME")).To(Succeed())
		Expect(os.Unsetenv("TCP_KEEPALIVE_INTVL")).To(Succeed())
		Expect(os.Unsetenv("TCP_KEEPALIVE_PROBES")).To(Succeed())
		Expect(os.Unsetenv("IP_FAMILIES")).To(Succeed())
		Expect(os.Unsetenv("ENDPOINT")).To(Succeed())
		Expect(os.Unsetenv("OPENVPN_PORT")).To(Succeed())
		Expect(os.Unsetenv("VPN_NETWORK")).To(Succeed())
		Expect(os.Unsetenv("SEED_POD_NETWORK_V4")).To(Succeed())
		Expect(os.Unsetenv("SHOOT_SERVICE_NETWORKS")).To(Succeed())
		Expect(os.Unsetenv("SHOOT_POD_NETWORKS")).To(Succeed())
		Expect(os.Unsetenv("SHOOT_NODE_NETWORKS")).To(Succeed())
		Expect(os.Unsetenv("IS_SHOOT_CLIENT")).To(Succeed())
		Expect(os.Unsetenv("POD_NAME")).To(Succeed())
		Expect(os.Unsetenv("NAMESPACE")).To(Succeed())
		Expect(os.Unsetenv("VPN_SERVER_INDEX")).To(Succeed())
		Expect(os.Unsetenv("IS_HA")).To(Succeed())
		Expect(os.Unsetenv("REVERSED_VPN_HEADER")).To(Succeed())
		Expect(os.Unsetenv("HA_VPN_CLIENTS")).To(Succeed())
		Expect(os.Unsetenv("HA_VPN_SERVERS")).To(Succeed())
		Expect(os.Unsetenv("POD_LABEL_SELECTOR")).To(Succeed())
		Expect(os.Unsetenv("WAIT_TIME")).To(Succeed())
	})

	type testCase struct {
		envVars         map[string]string
		expectedMatcher types.GomegaMatcher
		expectedError   bool
	}

	DescribeTable("should parse the configuration correctly",
		func(tc testCase) {

			// Merge default and test environment variables
			envVars := maps.Clone(defaultEnvVars)
			maps.Copy(envVars, tc.envVars)

			// Set environment variables for the test
			for key, value := range envVars {
				Expect(os.Setenv(key, value)).To(Succeed())
			}

			// Call the function
			cfg, err := config.GetVPNClientConfig()

			if tc.expectedError {
				Expect(err).To(HaveOccurred())
				return
			}

			Expect(err).NotTo(HaveOccurred())
			if tc.expectedMatcher != nil {
				Expect(cfg).To(tc.expectedMatcher)
			}
		},
		Entry("default configuration", testCase{
			envVars:       defaultEnvVars,
			expectedError: false,
		}),
		Entry("invalid HA configuration should fail", testCase{
			envVars: map[string]string{
				"IS_HA":            "banana",
				"VPN_SERVER_INDEX": "",
				"POD_NAME":         "",
			},
			expectedError: true,
		}),
		Entry("HA configuration should fail if POD_NAME is missing", testCase{
			envVars: map[string]string{
				"IS_HA":            "true",
				"VPN_SERVER_INDEX": "1",
				"POD_NAME":         "",
			},
			expectedError: true,
		}),
		Entry("HA configuration should fail if VPN_SERVER_INDEX is missing", testCase{
			envVars: map[string]string{
				"IS_HA":            "true",
				"VPN_SERVER_INDEX": "",
				"POD_NAME":         "test-pod-1",
			},
			expectedError: true,
		}),
		Entry("non-HA configuration should not require VPN_SERVER_INDEX", testCase{
			envVars: map[string]string{
				"IS_HA":            "false",
				"VPN_SERVER_INDEX": "",
			},
			expectedError: false,
		}),
		Entry("non-HA configuration with random pod name should yield VPNClientIndex of -1", testCase{
			envVars: map[string]string{
				"IS_HA":    "false",
				"POD_NAME": "test-pod-6f7c79b87f-kpqcl",
			},
			expectedMatcher: MatchFields(IgnoreExtras, Fields{"VPNClientIndex": Equal(-1)}),
		}),
		Entry("HA configuration with non-random pod name should yield correct VPNClientIndex", testCase{
			envVars: map[string]string{
				"POD_NAME": "test-pod-2",
			},
			expectedMatcher: MatchFields(IgnoreExtras, Fields{"VPNClientIndex": Equal(2)}),
		}),
		Entry("empty TCP config should yield default values", testCase{
			envVars: map[string]string{
				"TCP_KEEPALIVE_TIME":   "",
				"TCP_KEEPALIVE_INTVL":  "",
				"TCP_KEEPALIVE_PROBES": "",
			},
			expectedMatcher: MatchFields(IgnoreExtras, Fields{
				"TCP": MatchFields(IgnoreExtras, Fields{
					"KeepAliveTime":     Equal(uint64(7200)),
					"KeepAliveInterval": Equal(uint64(75)),
					"KeepAliveProbes":   Equal(uint64(9)),
				}),
			}),
		}),
		Entry("bad TCP config should fail", testCase{
			envVars: map[string]string{
				"TCP_KEEPALIVE_TIME":   "potato",
				"TCP_KEEPALIVE_INTVL":  "banana",
				"TCP_KEEPALIVE_PROBES": "apple",
			},
			expectedError: true,
		}),
		Entry("negative TCP config values should fail", testCase{
			envVars: map[string]string{
				"TCP_KEEPALIVE_TIME":   "-100",
				"TCP_KEEPALIVE_INTVL":  "-10",
				"TCP_KEEPALIVE_PROBES": "-30",
			},
			expectedError: true,
		}),
		Entry("empty IP_FAMILIES should yield IPv4 default", testCase{
			envVars: map[string]string{
				"IP_FAMILIES": "",
			},
			expectedMatcher: MatchFields(IgnoreExtras, Fields{"IPFamilies": ContainElement("IPv4")}),
		}),
		Entry("IP_FAMILIES with more than two entries should fail", testCase{
			envVars: map[string]string{
				"IP_FAMILIES": "IPv4,IPv6,IPv4",
			},
			expectedError: true,
		}),
		Entry("IP_FAMILIES with values other than IPv4 or IPv6 should fail", testCase{
			envVars: map[string]string{
				"IP_FAMILIES": "IPv4,IPv7",
			},
			expectedError: true,
		}),
		Entry("IP_FAMILIES with duplicate values are ignored", testCase{
			envVars: map[string]string{
				"IP_FAMILIES": "IPv4,IPv4",
			},
			expectedMatcher: MatchFields(IgnoreExtras, Fields{"IPFamilies": ConsistOf("IPv4")}),
		}),
		Entry("negative OPENVPN_PORT value should fail", testCase{
			envVars: map[string]string{
				"OPENVPN_PORT": "-1234",
			},
			expectedError: true,
		}),
		Entry("non-number OPENVPN_PORT value should fail", testCase{
			envVars: map[string]string{
				"OPENVPN_PORT": "banana",
			},
			expectedError: true,
		}),
		Entry("non-CIDR VPN_NETWORK value should fail", testCase{
			envVars: map[string]string{
				"VPN_NETWORK": "banana",
			},
			expectedError: true,
		}),
		Entry("missing VPN_NETWORK value should yield default", testCase{
			envVars: map[string]string{
				"VPN_NETWORK": "",
			},
			expectedMatcher: MatchFields(IgnoreExtras, Fields{
				"VPNNetwork": Equal(network.ParseIPNetIgnoreError(constants.DefaultVPNRangeV6))}),
		}),
		Entry("multiple vpn networks should fail", testCase{
			envVars: map[string]string{
				"VPN_NETWORK": "fd8f:6d53:b97a:1::/96,fd8f:6d53:b97a:2::/96",
			},
			expectedError: true,
		}),
		Entry("multiple seed pod networks should fail", testCase{
			envVars: map[string]string{
				"SEED_POD_NETWORK_V4": "10.0.0.0/8,11.0.0.0/8,12.0.0.0/8",
			},
			expectedError: true,
		}),
		Entry("multiple pod networks", testCase{
			envVars: map[string]string{
				"SHOOT_POD_NETWORKS": "100.96.0.0/11,100.97.0.0/11,100.98.0.0/11",
			},
			expectedMatcher: MatchFields(IgnoreExtras, Fields{
				"ShootPodNetworks": ConsistOf(
					network.ParseIPNetIgnoreError("100.96.0.0/11"),
					network.ParseIPNetIgnoreError("100.97.0.0/11"),
					network.ParseIPNetIgnoreError("100.98.0.0/11")),
			}),
		}),
		Entry("multiple service networks", testCase{
			envVars: map[string]string{
				"SHOOT_SERVICE_NETWORKS": "100.96.0.0/11,100.97.0.0/11,100.98.0.0/11",
			},
			expectedMatcher: MatchFields(IgnoreExtras, Fields{
				"ShootServiceNetworks": ConsistOf(
					network.ParseIPNetIgnoreError("100.96.0.0/11"),
					network.ParseIPNetIgnoreError("100.97.0.0/11"),
					network.ParseIPNetIgnoreError("100.98.0.0/11")),
			}),
		}),
		Entry("multiple node networks", testCase{
			envVars: map[string]string{
				"SHOOT_NODE_NETWORKS": "100.96.0.0/11,100.97.0.0/11,100.98.0.0/11",
			},
			expectedMatcher: MatchFields(IgnoreExtras, Fields{
				"ShootNodeNetworks": ConsistOf(
					network.ParseIPNetIgnoreError("100.96.0.0/11"),
					network.ParseIPNetIgnoreError("100.97.0.0/11"),
					network.ParseIPNetIgnoreError("100.98.0.0/11")),
			}),
		}),
		Entry("non-boolean IS_SHOOT_CLIENT value should fail", testCase{
			envVars: map[string]string{
				"IS_SHOOT_CLIENT": "banana",
			},
			expectedError: true,
		}),
		Entry("non-number HA_VPN_CLIENTS value should fail", testCase{
			envVars: map[string]string{
				"HA_VPN_CLIENTS": "banana",
			},
			expectedError: true,
		}),
		Entry("negative HA_VPN_CLIENTS value should fail", testCase{
			envVars: map[string]string{
				"HA_VPN_CLIENTS": "-1",
			},
			expectedError: true,
		}),
		Entry("non-number HA_VPN_SERVERS value should fail", testCase{
			envVars: map[string]string{
				"HA_VPN_SERVERS": "banana",
			},
			expectedError: true,
		}),
		Entry("negative HA_VPN_SERVERS value should fail", testCase{
			envVars: map[string]string{
				"HA_VPN_SERVERS": "-1",
			},
			expectedError: true,
		}),
		Entry("negative WAIT_TIME value should fail", testCase{
			envVars: map[string]string{
				"WAIT_TIME": "-1s",
			},
			expectedError: true,
		}),
		Entry("missing WAIT_TIME value should yield the default", testCase{
			envVars: map[string]string{
				"WAIT_TIME": "",
			},
			expectedMatcher: MatchFields(IgnoreExtras, Fields{"WaitTime": Equal(2 * time.Second)}),
		}),
		Entry("missing POD_LABEL_SELECTOR value should yield the default", testCase{
			envVars: map[string]string{
				"POD_LABEL_SELECTOR": "",
			},
			expectedMatcher: MatchFields(IgnoreExtras, Fields{"PodLabelSelector": Equal("app=kubernetes,role=apiserver")}),
		}),
	)
})
