package tunnel

import (
	"fmt"
	"net/http"
	"time"
)

const (
	ReadinessPort = 8080
)

func (c *Controller) NewReadinessServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if !c.IsReady() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready"))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}
	})
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", ReadinessPort),
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}
	return server
}

// IsReady returns true if all listed kube apiservers have routes created successfully.
func (c *Controller) IsReady() bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	// If no kube apiservers are configured, we are not ready.
	if len(c.kubeApiservers) == 0 {
		return false
	}

	// If there are kube apiservers, check that all are ready.
	for _, data := range c.kubeApiservers {
		data.lock.Lock()
		ready := data.creationComplete && data.lastError == nil
		data.lock.Unlock()
		if !ready {
			return false
		}
	}
	return true
}
