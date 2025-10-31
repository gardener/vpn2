package tunnel

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestReadiness(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Readiness Suite")
}

var _ = Describe("Controller Readiness", func() {
	var c *Controller

	BeforeEach(func() {
		c = &Controller{}
	})

	Describe("IsReady", func() {
		It("returns false if no kube apiservers are configured", func() {
			c.kubeApiservers = nil
			Expect(c.IsReady()).To(BeFalse())
		})

		It("returns false if any kube apiserver is not ready", func() {
			c.kubeApiservers = map[string]*kubeApiserverData{
				"10.10.0.1": {creationComplete: true, lastError: nil},
				"10.10.0.2": {creationComplete: false, lastError: nil},
			}
			Expect(c.IsReady()).To(BeFalse())
		})

		It("returns false if any kube apiserver has an error", func() {
			c.kubeApiservers = map[string]*kubeApiserverData{
				"10.10.0.3": {creationComplete: true, lastError: errors.New("fail")},
			}
			Expect(c.IsReady()).To(BeFalse())
		})

		It("returns true if all kube apiservers are ready and error-free", func() {
			c.kubeApiservers = map[string]*kubeApiserverData{
				"10.10.0.4": {creationComplete: true, lastError: nil},
				"10.10.0.5": {creationComplete: true, lastError: nil},
			}
			Expect(c.IsReady()).To(BeTrue())
		})
	})

	Describe("NewReadinessServer", func() {
		It("serves /readyz with 200 when ready", func() {
			c.kubeApiservers = map[string]*kubeApiserverData{
				"10.10.0.6": {creationComplete: true, lastError: nil},
			}
			server := c.NewReadinessServer()

			req := httptest.NewRequest("GET", "/readyz", nil)
			w := httptest.NewRecorder()
			server.Handler.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(Equal("ok"))
		})

		It("serves /readyz with 503 when not ready", func() {
			c.kubeApiservers = nil
			server := c.NewReadinessServer()

			req := httptest.NewRequest("GET", "/readyz", nil)
			w := httptest.NewRecorder()
			server.Handler.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusServiceUnavailable))
			Expect(w.Body.String()).To(Equal("not ready"))
		})
	})
})
