package config_test

import (
	"maps"
	"os"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/network"
)

var _ = Describe("GetVPNServerConfig", func() {
	var (
		logger         logr.Logger
		defaultEnvVars map[string]string
	)

	BeforeEach(func() {
		// Create a logger
		logger = funcr.New(func(prefix, args string) {
			GinkgoT().Logf("%s: %s", prefix, args)
		}, funcr.Options{})

		// Set default environment variables
		defaultEnvVars = map[string]string{
			"SERVICE_NETWORKS":    "100.64.0.0/13",
			"POD_NETWORKS":        "100.96.0.0/11",
			"NODE_NETWORKS":       "100.128.0.0/10",
			"VPN_NETWORK":         "fd8f:6d53:b97a:1::/96",
			"SEED_POD_NETWORK_V4": "10.0.0.0/8",
			"POD_NAME":            "test-pod",
			"OPENVPN_STATUS_PATH": "/status",
			"IS_HA":               "true",
			"HA_VPN_CLIENTS":      "3",
			"LOCAL_NODE_IP":       "192.168.1.1",
		}
	})

	AfterEach(func() {
		// Clean up environment variables after each test
		Expect(os.Unsetenv("SERVICE_NETWORKS")).To(Succeed())
		Expect(os.Unsetenv("POD_NETWORKS")).To(Succeed())
		Expect(os.Unsetenv("NODE_NETWORKS")).To(Succeed())
		Expect(os.Unsetenv("VPN_NETWORK")).To(Succeed())
		Expect(os.Unsetenv("SEED_POD_NETWORK_V4")).To(Succeed())
		Expect(os.Unsetenv("POD_NAME")).To(Succeed())
		Expect(os.Unsetenv("OPENVPN_STATUS_PATH")).To(Succeed())
		Expect(os.Unsetenv("IS_HA")).To(Succeed())
		Expect(os.Unsetenv("HA_VPN_CLIENTS")).To(Succeed())
		Expect(os.Unsetenv("LOCAL_NODE_IP")).To(Succeed())
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
			cfg, err := config.GetVPNServerConfig(logger)

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
				"IS_HA":               "banana",
				"HA_VPN_CLIENTS":      "-3",
				"POD_NAME":            "",
				"OPENVPN_STATUS_PATH": "",
			},
			expectedError: true,
		}),
		Entry("HA configuration should fail if HA_VPN_CLIENTS is missing", testCase{
			envVars: map[string]string{
				"IS_HA":               "true",
				"HA_VPN_CLIENTS":      "",
				"POD_NAME":            "test-pod",
				"OPENVPN_STATUS_PATH": "/status",
			},
			expectedError: true,
		}),
		Entry("HA configuration should fail if HA_VPN_CLIENTS is negative", testCase{
			envVars: map[string]string{
				"IS_HA":               "true",
				"HA_VPN_CLIENTS":      "-3",
				"POD_NAME":            "test-pod",
				"OPENVPN_STATUS_PATH": "/status",
			},
			expectedError: true,
		}),
		Entry("HA configuration should fail if POD_NAME is missing", testCase{
			envVars: map[string]string{
				"IS_HA":               "true",
				"HA_VPN_CLIENTS":      "3",
				"POD_NAME":            "",
				"OPENVPN_STATUS_PATH": "/status",
			},
			expectedError: true,
		}),
		Entry("HA configuration should fail if OPENVPN_STATUS_PATH is missing", testCase{
			envVars: map[string]string{
				"IS_HA":               "true",
				"HA_VPN_CLIENTS":      "3",
				"POD_NAME":            "test-pod",
				"OPENVPN_STATUS_PATH": "",
			},
			expectedError: true,
		}),
		Entry("missing VPN_NETWORK should yield default", testCase{
			envVars: map[string]string{
				"VPN_NETWORK": "",
			},
			expectedMatcher: MatchFields(IgnoreExtras, Fields{
				"VPNNetwork": Equal(network.ParseIPNetIgnoreError(constants.DefaultVPNRangeV6)),
			}),
		}),
		Entry("missing LOCAL_NODE_IP should yield env default", testCase{
			envVars: map[string]string{
				"LOCAL_NODE_IP": "",
			},
			expectedMatcher: MatchFields(IgnoreExtras, Fields{
				"LocalNodeIP": Equal("255.255.255.255"),
			}),
		}),
		Entry("multiple pod networks with spaces should fail", testCase{
			envVars: map[string]string{
				"POD_NETWORKS": "100.96.0.0/11, 100.97.0.0/11 , 100.98.0.0/11",
			},
			expectedError: true,
		}),
		Entry("multiple pod networks", testCase{
			envVars: map[string]string{
				"POD_NETWORKS": "100.96.0.0/11,100.97.0.0/11,100.98.0.0/11",
			},
			expectedMatcher: MatchFields(IgnoreExtras, Fields{
				"PodNetworks": ConsistOf(
					network.ParseIPNetIgnoreError("100.96.0.0/11"),
					network.ParseIPNetIgnoreError("100.97.0.0/11"),
					network.ParseIPNetIgnoreError("100.98.0.0/11")),
			}),
		}),
		Entry("multiple service networks", testCase{
			envVars: map[string]string{
				"SERVICE_NETWORKS": "100.64.0.0/13,100.65.0.0/13,100.66.0.0/13",
			},
			expectedMatcher: MatchFields(IgnoreExtras, Fields{
				"ServiceNetworks": ConsistOf(
					network.ParseIPNetIgnoreError("100.64.0.0/13"),
					network.ParseIPNetIgnoreError("100.65.0.0/13"),
					network.ParseIPNetIgnoreError("100.66.0.0/13")),
			}),
		}),
		Entry("multiple node networks", testCase{
			envVars: map[string]string{
				"NODE_NETWORKS": "100.128.0.0/10,101.128.0.0/10,102.128.0.0/10",
			},
			expectedMatcher: MatchFields(IgnoreExtras, Fields{
				"NodeNetworks": ConsistOf(
					network.ParseIPNetIgnoreError("100.128.0.0/10"),
					network.ParseIPNetIgnoreError("101.128.0.0/10"),
					network.ParseIPNetIgnoreError("102.128.0.0/10")),
			}),
		}),
		Entry("multiple seed pod networks should fail", testCase{
			envVars: map[string]string{
				"SEED_POD_NETWORK_V4": "10.0.0.0/8,11.0.0.0/8,12.0.0.0/8",
			},
			expectedError: true,
		}),
		Entry("multiple vpn networks should fail", testCase{
			envVars: map[string]string{
				"VPN_NETWORK": "fd8f:6d53:b97a:1::/96,fd8f:6d53:b97a:2::/96",
			},
			expectedError: true,
		}),
	)
})
