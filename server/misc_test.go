package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func TestRateLimiterSweepAndHelpers(t *testing.T) {
	l, stop := newIPRateLimiter(100, 2)
	if !l.allow("old") {
		t.Fatal("first request denied")
	}
	l.mu.Lock()
	l.clients["old"].lastSeenAt = time.Now().Add(-rateLimiterIdleTTL - time.Second)
	l.clients["new"] = &clientLimiter{lastSeenAt: time.Now()}
	l.mu.Unlock()
	l.sweep()
	if _, ok := l.clients["old"]; ok {
		t.Error("idle limiter was not swept")
	}
	if _, ok := l.clients["new"]; !ok {
		t.Error("active limiter was swept")
	}
	stop()

	for input, want := range map[string]string{"": "/fallback", "spec": "/spec", "/spec": "/spec", "/docs": "/docs/"} {
		trailing := input == "/docs"
		if got := normalizePath(input, "/fallback", trailing); got != want {
			t.Errorf("normalizePath(%q) = %q, want %q", input, got, want)
		}
	}
	for status, want := range map[int]string{200: "2xx", 399: "3xx", 404: "4xx", 500: "5xx"} {
		if got := statusLabel(status); got != want {
			t.Errorf("statusLabel(%d) = %q", status, got)
		}
	}
	if statusLabel(0) != "unknown" {
		t.Error("unexpected label for unset status")
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "not-host-port"
	if clientIP(req) != "not-host-port" {
		t.Error("raw RemoteAddr fallback failed")
	}
}

func TestBearerTokenRejectsWrongScheme(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic credentials")
	if _, err := bearerToken(req); err == nil {
		t.Fatal("Basic authorization accepted as bearer")
	}
}

func TestWorkerMetricsMuxHealthStates(t *testing.T) {
	s := queuetest.StartServer(t)
	q, err := queue.ConnectWithOptions(s.URL, queue.DefaultNamespace, s.Options...)
	if err != nil {
		t.Fatal(err)
	}
	h := workerMetricsMux(q)
	for path, want := range map[string]int{"/healthz": http.StatusOK, "/metrics": http.StatusOK} {
		res := httptest.NewRecorder()
		h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
		if res.Code != want {
			t.Errorf("%s status = %d", path, res.Code)
		}
	}
	q.Close()
	res := httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("closed NATS health = %d", res.Code)
	}
}

func TestHandlerRootAndNotFoundRoutes(t *testing.T) {
	cfg := testConfig(t, nil)
	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	token := signedJWT(t, cfg.OAuthIssuer, cfg.OAuthAudience, testSecret, time.Hour)
	for path, want := range map[string]int{"/": http.StatusOK, "/missing": http.StatusNotFound} {
		res := doRequest(h, http.MethodGet, path, token, "", "")
		if res.Code != want {
			t.Errorf("GET %s = %d", path, res.Code)
		}
	}
}

func TestSchemaCatalogParsingAndValidationErrors(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.xsd")
	if err := os.WriteFile(valid, []byte(`<schema targetNamespace="urn:test"><element/><element name="Root"/></schema>`), 0o600); err != nil {
		t.Fatal(err)
	}
	root, ns, err := rootElementOf(valid)
	if err != nil || root != "Root" || ns != "urn:test" {
		t.Fatalf("rootElementOf = %q, %q, %v", root, ns, err)
	}
	empty := filepath.Join(dir, "empty.xsd")
	_ = os.WriteFile(empty, []byte(`<schema/>`), 0o600)
	if root, _, err := rootElementOf(empty); err != nil || root != "" {
		t.Fatalf("empty schema = %q, %v", root, err)
	}
	bad := filepath.Join(dir, "bad.xsd")
	_ = os.WriteFile(bad, []byte(`<schema`), 0o600)
	if _, _, err := rootElementOf(bad); err == nil {
		t.Fatal("expected malformed XSD error")
	}
	if _, _, err := rootElementOf(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected missing XSD error")
	}
	if err := validateAgainstSchema(valid, []byte(`<Wrong/>`)); err == nil {
		t.Fatal("expected validation failure")
	}
}
