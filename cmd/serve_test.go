package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

func TestNewHandlerServesHealthAndOpenAPI(t *testing.T) {
	t.Parallel()

	issuer := "https://issuer.example.com"
	audience := "e-ubl21"
	handler := newHandler(&Config{
		OpenAPIPath:   "/spec",
		OAuthIssuer:   issuer,
		OAuthAudience: audience,
	})

	token := makeJWT(t, issuer, audience)

	healthRequest := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRequest.Header.Set("Authorization", "Bearer "+token)
	healthRecorder := httptest.NewRecorder()
	handler.ServeHTTP(healthRecorder, healthRequest)

	if healthRecorder.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want %d", healthRecorder.Code, http.StatusOK)
	}

	openAPIRequest := httptest.NewRequest(http.MethodGet, "/spec", nil)
	openAPIRequest.Header.Set("Authorization", "Bearer "+token)
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

func makeJWT(t *testing.T, issuer, audience string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    issuer,
		Audience:  jwt.ClaimStrings{audience},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
	})

	signed, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}
