package health

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func newServerWithFixture(fixture string, updateInterval int, isHA bool) (*http.Server, Config) {
	cfg := NewDefaultConfig()
	cfg.OpenVPNStatusPath = filepath.Join("test", fixture)
	cfg.OpenVPNStatusUpdateInterval = updateInterval
	cfg.IsHA = isHA
	return NewReadinessServer(cfg, logr.Discard()), cfg
}

func doRequest(h http.Handler, path string) (int, string) {
	req := httptest.NewRequest("GET", path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

// assertFixtureResponses runs health and readiness assertions for both HA modes.
func assertFixtureResponses(fixture string, updateInterval int, expectedHealthCode int, expectedHealthBody string, expectedReadiness func(isHA bool) (int, string)) {
	for _, isHA := range []bool{false, true} {
		server, cfg := newServerWithFixture(fixture, updateInterval, isHA)

		code, body := doRequest(server.Handler, cfg.HealthPath)
		Expect(code).To(Equal(expectedHealthCode))
		Expect(body).To(HavePrefix(expectedHealthBody))

		expCode, expBody := expectedReadiness(isHA)
		code, body = doRequest(server.Handler, cfg.ReadinessPath)
		Expect(code).To(Equal(expCode))
		Expect(body).To(HavePrefix(expBody))
	}
}

var _ = Describe("OpenVPN Readiness/Liveness (fixtures)", func() {
	It("serves expected responses for `test/openvpn-ready.status` in HA and non-HA", func() {
		largeInterval := int((24 * time.Hour).Seconds() * 365)
		assertFixtureResponses("openvpn-ready.status", largeInterval, http.StatusOK, StatusOK,
			func(isHA bool) (int, string) { return http.StatusOK, StatusReady })
	})

	It("serves expected responses for `test/openvpn-not-ready.status` in HA and non-HA", func() {
		largeInterval := int((24 * time.Hour).Seconds() * 365)
		assertFixtureResponses("openvpn-not-ready.status", largeInterval, http.StatusOK, StatusOK,
			func(isHA bool) (int, string) { return http.StatusServiceUnavailable, StatusNotReady })
	})

	It("serves expected responses for `test/openvpn-nonha-ready.status` in HA and non-HA", func() {
		largeInterval := int((24 * time.Hour).Seconds() * 365)
		assertFixtureResponses("openvpn-nonha-ready.status", largeInterval, http.StatusOK, StatusOK,
			func(isHA bool) (int, string) {
				if isHA {
					return http.StatusServiceUnavailable, StatusNotReady
				}
				return http.StatusOK, StatusReady
			})
	})

	It("serves expected responses for `test/openvpn-empty.status` depending on HA mode", func() {
		// Small interval so health is ServiceUnavailable due to outdated timestamp.
		assertFixtureResponses("openvpn-empty.status", 1, http.StatusServiceUnavailable, StatusNotOK,
			func(isHA bool) (int, string) {
				if isHA {
					return http.StatusServiceUnavailable, StatusNotReady
				}
				return http.StatusOK, StatusReady
			})
	})

	It("serves 500 for both health and readiness when status file is not found (both HA modes)", func() {
		assertFixtureResponses("does-not-exist.status", 1, http.StatusInternalServerError, StatusInternalError,
			func(isHA bool) (int, string) { return http.StatusInternalServerError, StatusInternalError })
	})
})
