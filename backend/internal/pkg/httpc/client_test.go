package httpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A self-redirecting origin must fail fast: the custom CheckRedirect
// replaces Go's default 10-redirect cap, so without an explicit limit the
// client follows the loop until the request timeout.
func TestClientBoundedRedirectLoop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusFound)
	}))
	defer server.Close()

	client, err := NewClient(2*time.Second, "", true)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	SetDefaultHeaders(req)

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected redirect loop to fail")
	}
	if resp != nil {
		resp.Body.Close()
	}
	if !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Fatalf("expected redirect cap error, got %v", err)
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("redirect loop ran until timeout (%v), expected fast failure", elapsed)
	}
}
