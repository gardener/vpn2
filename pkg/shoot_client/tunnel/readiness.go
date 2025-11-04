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
		ready, msg := c.IsReady()
		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		_, _ = w.Write([]byte(msg))
	})
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", ReadinessPort),
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}
	return server
}

// IsReady checks if there are routes to all configured kube apiservers.
// It returns a boolean indicating readiness and a message.
func (c *Controller) IsReady() (bool, string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	// If no kube apiservers are configured, we are not ready.
	if len(c.kubeApiservers) == 0 {
		return false, "no kube apiservers configured"
	}

	// If there are kube apiservers, check that all are ready.
	for _, data := range c.kubeApiservers {
		data.lock.Lock()
		podIP := data.podIP
		lastErrorStr := "no error"
		if data.lastError != nil {
			lastErrorStr = data.lastError.Error()
		}
		ready := data.creationComplete && data.lastError == nil
		data.lock.Unlock()
		if !ready {
			return false, fmt.Sprintf("no route to kube apiserver with pod IP %s (%s)", podIP, lastErrorStr)
		}
	}
	return true, "ok"
}
