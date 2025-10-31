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
			ready, msg := c.IsReady()
			Expect(ready).To(BeFalse())
			Expect(msg).To(Equal("no kube apiservers configured"))
		})

		It("returns false if any kube apiserver is not ready", func() {
			c.kubeApiservers = map[string]*kubeApiserverData{
				"fd8f:6d53:b97a:1::a:6987": {creationComplete: true, lastError: nil, podIP: "10.10.0.1"},
				"fd8f:6d53:b97a:1::a:36c3": {creationComplete: false, lastError: nil, podIP: "10.10.0.2"},
			}
			ready, msg := c.IsReady()
			Expect(ready).To(BeFalse())
			Expect(msg).To(Equal("no route to kube apiserver with pod IP 10.10.0.2 (no error)"))
		})

		It("returns false if any kube apiserver has an error", func() {
			c.kubeApiservers = map[string]*kubeApiserverData{
				"fd8f:6d53:b97a:1::a:fa56": {creationComplete: true, lastError: errors.New("fail"), podIP: "10.10.0.3"},
			}
			ready, msg := c.IsReady()
			Expect(ready).To(BeFalse())
			Expect(msg).To(Equal("no route to kube apiserver with pod IP 10.10.0.3 (fail)"))
		})

		It("returns true if all kube apiservers are ready and error-free", func() {
			c.kubeApiservers = map[string]*kubeApiserverData{
				"fd8f:6d53:b97a:1::a:6987": {creationComplete: true, lastError: nil, podIP: "10.10.0.4"},
				"fd8f:6d53:b97a:1::a:36c3": {creationComplete: true, lastError: nil, podIP: "10.10.0.5"},
			}
			ready, msg := c.IsReady()
			Expect(ready).To(BeTrue())
			Expect(msg).To(Equal("ok"))
		})
	})

	Describe("NewReadinessServer", func() {
		It("serves /readyz with 200 when ready", func() {
			c.kubeApiservers = map[string]*kubeApiserverData{
				"fd8f:6d53:b97a:1::a:fa56": {creationComplete: true, lastError: nil, podIP: "10.10.0.6"},
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
			Expect(w.Body.String()).To(Equal("no kube apiservers configured"))
		})
	})
})
