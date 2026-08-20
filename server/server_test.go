package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

func TestNewHandlerMatchesUploadSpecification(t *testing.T) {
	t.Parallel()

	issuer := "https://issuer.example.com"
	audience := "e-ubl21"
	handler := NewHandler(Config{OpenAPIPath: "/spec", OAuthIssuer: issuer, OAuthAudience: audience})
	token := signedJWT(t, issuer, audience)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ubl/upload", stringsReader("<Invoice><cbc:ID>INV-001</cbc:ID><cbc:IssueDate>2026-08-20</cbc:IssueDate></Invoice>"))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/xml")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("upload status = %d, want %d", res.Code, http.StatusAccepted)
	}

	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "accepted" {
		t.Fatalf("status = %v, want %q", payload["status"], "accepted")
	}
	if payload["document"] != "ubl-2.1-xml" {
		t.Fatalf("document = %v, want %q", payload["document"], "ubl-2.1-xml")
	}
	if _, ok := payload["receivedAt"]; !ok {
		t.Fatal("missing receivedAt field")
	}

	specReq := httptest.NewRequest(http.MethodGet, "/spec", nil)
	specReq.Header.Set("Authorization", "Bearer "+token)
	specRes := httptest.NewRecorder()
	handler.ServeHTTP(specRes, specReq)
	if specRes.Code != http.StatusOK {
		t.Fatalf("spec status = %d, want %d", specRes.Code, http.StatusOK)
	}

	var spec map[string]any
	if err := json.Unmarshal(specRes.Body.Bytes(), &spec); err != nil {
		t.Fatalf("decode spec: %v", err)
	}
	if spec["openapi"] != "3.0.3" {
		t.Fatalf("openapi = %v, want %q", spec["openapi"], "3.0.3")
	}
	if spec["info"].(map[string]any)["title"] != "e-ubl21 API" {
		t.Fatalf("title = %v, want %q", spec["info"].(map[string]any)["title"], "e-ubl21 API")
	}
	if _, ok := spec["security"].([]any); !ok {
		t.Fatal("missing security section")
	}
	paths := spec["paths"].(map[string]any)
	if _, ok := paths["/api/v1/ubl/upload"]; !ok {
		t.Fatal("missing upload path in specification")
	}
	components := spec["components"].(map[string]any)
	securitySchemes := components["securitySchemes"].(map[string]any)
	if securitySchemes["oauth2"].(map[string]any)["type"] != "oauth2" {
		t.Fatal("oauth2 security scheme missing")
	}
}

func TestNewHandlerRejectsMissingBearerToken(t *testing.T) {
	t.Parallel()

	handler := NewHandler(Config{OpenAPIPath: "/spec", OAuthIssuer: "https://issuer.example.com", OAuthAudience: "e-ubl21"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ubl/upload", stringsReader("<Invoice />"))
	req.Header.Set("Content-Type", "application/xml")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func stringsReader(value string) *strings.Reader {
	return strings.NewReader(value)
}

func signedJWT(t *testing.T, issuer, audience string) string {
	t.Helper()

	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Audience:  jwt.ClaimStrings{audience},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}
