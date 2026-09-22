//go:build compat

package compat

import (
	"net/http"
	"testing"
	"time"
)

// waitHTTPOK polls url until it returns 2xx or the timeout elapses.
//
// The compat suite allows a longer default than e2e does: a pinned run pulls
// released images rather than reusing locally-built ones, so first boot pays
// registry latency on top of the usual startup.
func waitHTTPOK(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return
			}
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("%s not healthy after %s", url, timeout)
}
