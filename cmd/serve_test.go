package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewHandlerServesHealthAndOpenAPI(t *testing.T) {
	t.Parallel()

	handler := newHandler(&Config{
		OpenAPIPath:   "/spec",
		OAuthIssuer:   "https://issuer.example.com",
		OAuthAudience: "e-ubl21",
	})

	healthRequest := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRecorder := httptest.NewRecorder()
	handler.ServeHTTP(healthRecorder, healthRequest)

	if healthRecorder.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want %d", healthRecorder.Code, http.StatusOK)
	}

	openAPIRequest := httptest.NewRequest(http.MethodGet, "/spec", nil)
	openAPIRecorder := httptest.NewRecorder()
	handler.ServeHTTP(openAPIRecorder, openAPIRequest)

	if openAPIRecorder.Code != http.StatusOK {
		t.Fatalf("spec status = %d, want %d", openAPIRecorder.Code, http.StatusOK)
	}

	var document map[string]any
	if err := json.Unmarshal(openAPIRecorder.Body.Bytes(), &document); err != nil {
		t.Fatalf("unmarshal OpenAPI document: %v", err)
	}

	if got := document["openapi"]; got != "3.0.3" {
		t.Fatalf("openapi version = %v, want %q", got, "3.0.3")
	}
}
