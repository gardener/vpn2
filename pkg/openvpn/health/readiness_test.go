package health

import (
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OpenVPN Server Readiness/Liveness", func() {
	type testCase struct {
		name           string
		updateInterval int
		isHA           bool
		expectAlive    bool
		expectReady    bool
	}

	largeInterval := int((24 * time.Hour).Seconds() * 365)

	// Define test cases for each fixture and mode
	cases := []testCase{
		// openvpn-ready.status
		{"openvpn-ready.status", largeInterval, false, true, true},
		{"openvpn-ready.status", largeInterval, true, true, true},
		// openvpn-not-ready.status
		{"openvpn-not-ready.status", largeInterval, false, true, false},
		{"openvpn-not-ready.status", largeInterval, true, true, false},
		// openvpn-nonha-ready.status
		{"openvpn-nonha-ready.status", largeInterval, false, true, true},
		{"openvpn-nonha-ready.status", largeInterval, true, true, false},
		// openvpn-empty.status
		{"openvpn-empty.status", 1, false, false, true},
		{"openvpn-empty.status", 1, true, false, false},
		// openvpn-ready-ipv6.status
		{"openvpn-ready-ipv6.status", largeInterval, false, true, true},
		{"openvpn-ready-ipv6.status", largeInterval, true, true, true},
		// openvpn27-ready-ipv6.status
		{"openvpn27-ready-ipv6.status", largeInterval, false, true, true},
		{"openvpn27-ready-ipv6.status", largeInterval, true, true, true},
		// does-not-exist.status
		{"does-not-exist.status", 1, false, false, false},
		{"does-not-exist.status", 1, true, false, false},
	}

	for _, c := range cases {
		tc := c // capture range variable
		It("checks IsAlive and IsReady for "+tc.name+" (HA="+func() string {
			if tc.isHA {
				return "true"
			}
			return "false"

		}()+")", func() {
			cfg := NewDefaultConfig()
			cfg.OpenVPNStatusPath = filepath.Join("test", tc.name)
			cfg.OpenVPNStatusUpdateInterval = tc.updateInterval
			cfg.IsHA = tc.isHA

			alive := IsAlive(cfg, logr.Discard())
			Expect(alive).To(Equal(tc.expectAlive), "IsAlive failed for "+tc.name)

			ready := IsReady(cfg, logr.Discard())
			Expect(ready).To(Equal(tc.expectReady), "IsReady failed for "+tc.name)
		})
	}
})
