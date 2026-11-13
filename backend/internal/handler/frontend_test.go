package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/0x2E/fusion/internal/config"
	"github.com/gin-gonic/gin"
)

func TestSetupRouterServesEmbeddedFrontend(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}
	r := h.SetupRouter()

	t.Run("serves index for root", func(t *testing.T) {
		w := performRequest(r, http.MethodGet, "/", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		if contentType := w.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
			t.Fatalf("expected text/html content type, got %q", contentType)
		}
		if csp := w.Header().Get("Content-Security-Policy"); csp == "" {
			t.Fatal("expected Content-Security-Policy header")
		}
	})

	t.Run("serves index for client-side route", func(t *testing.T) {
		w := performRequest(r, http.MethodGet, "/feeds", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		if contentType := w.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
			t.Fatalf("expected text/html content type, got %q", contentType)
		}
	})

	t.Run("returns 404 for unknown api path", func(t *testing.T) {
		w := performRequest(r, http.MethodGet, "/api/not-found", nil, nil)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("returns 404 for missing asset", func(t *testing.T) {
		w := performRequest(r, http.MethodGet, "/assets/does-not-exist-0db74db9a5.js", nil, nil)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", w.Code)
		}
	})
}

func TestSetupRouterServesFrontendUnderBasePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{config: &config.Config{BasePath: "/fusion"}}
	r := h.SetupRouter()

	t.Run("redirects domain root to base path", func(t *testing.T) {
		w := performRequest(r, http.MethodGet, "/", nil, nil)
		if w.Code != http.StatusFound {
			t.Fatalf("expected status 302, got %d", w.Code)
		}
		if location := w.Header().Get("Location"); location != "/fusion/" {
			t.Fatalf("expected Location %q, got %q", "/fusion/", location)
		}
	})

	t.Run("redirects base path without trailing slash", func(t *testing.T) {
		w := performRequest(r, http.MethodGet, "/fusion", nil, nil)
		if w.Code != http.StatusMovedPermanently {
			t.Fatalf("expected status 301, got %d", w.Code)
		}
		if location := w.Header().Get("Location"); location != "/fusion/" {
			t.Fatalf("expected Location %q, got %q", "/fusion/", location)
		}
	})

	t.Run("serves index with injected base for base path root", func(t *testing.T) {
		w := performRequest(r, http.MethodGet, "/fusion/", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		if contentType := w.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
			t.Fatalf("expected text/html content type, got %q", contentType)
		}

		body := w.Body.String()
		if !strings.Contains(body, `<base href="/fusion/">`) {
			t.Fatalf("expected injected base tag, got %q", body)
		}
		if !strings.Contains(body, `<meta name="fusion-base-path" content="/fusion/">`) {
			t.Fatalf("expected injected base path meta tag, got %q", body)
		}
	})

	t.Run("serves index for client-side route under base path", func(t *testing.T) {
		w := performRequest(r, http.MethodGet, "/fusion/feeds", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		if contentType := w.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
			t.Fatalf("expected text/html content type, got %q", contentType)
		}
	})

	t.Run("injects base into index.html requests", func(t *testing.T) {
		w := performRequest(r, http.MethodGet, "/fusion/index.html", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), `<base href="/fusion/">`) {
			t.Fatalf("expected injected base tag, got %q", w.Body.String())
		}
	})

	t.Run("returns 404 outside base path", func(t *testing.T) {
		for _, target := range []string{"/feeds", "/api/not-found", "/assets/does-not-exist.js"} {
			w := performRequest(r, http.MethodGet, target, nil, nil)
			if w.Code != http.StatusNotFound {
				t.Fatalf("expected status 404 for %q, got %d", target, w.Code)
			}
		}
	})

	t.Run("returns 404 for unknown api path under base path", func(t *testing.T) {
		w := performRequest(r, http.MethodGet, "/fusion/api/not-found", nil, nil)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("returns 404 for missing asset under base path", func(t *testing.T) {
		w := performRequest(r, http.MethodGet, "/fusion/assets/does-not-exist-0db74db9a5.js", nil, nil)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", w.Code)
		}
	})
}
