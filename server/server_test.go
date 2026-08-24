package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	jwt "github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret"

const validInvoiceXML = `<?xml version="1.0" encoding="UTF-8"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
         xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
         xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
  <cbc:ID>INV-001</cbc:ID>
  <cbc:IssueDate>2026-08-20</cbc:IssueDate>
  <cac:AccountingSupplierParty/>
  <cac:AccountingCustomerParty/>
  <cac:LegalMonetaryTotal>
    <cbc:PayableAmount currencyID="USD">100.00</cbc:PayableAmount>
  </cac:LegalMonetaryTotal>
  <cac:InvoiceLine>
    <cbc:ID>1</cbc:ID>
    <cbc:LineExtensionAmount currencyID="USD">100.00</cbc:LineExtensionAmount>
    <cac:Item/>
  </cac:InvoiceLine>
</Invoice>`

const invalidInvoiceXML = `<?xml version="1.0" encoding="UTF-8"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
         xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
  <cbc:ID>INV-001</cbc:ID>
</Invoice>`

func testConfig(t *testing.T, overrides func(*Config)) Config {
	t.Helper()
	cfg := Config{
		OpenAPIPath:     "/spec",
		SwaggerPath:     "/docs/",
		OAuthIssuer:     "https://issuer.example.com",
		OAuthAudience:   "e-ubl21",
		OAuthHMACSecret: testSecret,
		NATSURL:         queuetest.StartServer(t),
	}
	if overrides != nil {
		overrides(&cfg)
	}
	return cfg
}

func newTestHandler(t *testing.T, overrides func(*Config)) http.Handler {
	t.Helper()
	handler, err := NewHandler(testConfig(t, overrides))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	return handler
}

func doRequest(handler http.Handler, method, path, token, body, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func TestNewHandlerQueuesUploadAndPollsMetadataByDefault(t *testing.T) {
	t.Parallel()
	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)

	res := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", token, validInvoiceXML, "application/xml")
	if res.Code != http.StatusAccepted {
		t.Fatalf("upload status = %d; body=%s", res.Code, res.Body.String())
	}
	var accepted map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	id, _ := accepted["id"].(string)
	if id == "" {
		t.Fatal("missing upload id")
	}

	poll := doRequest(handler, http.MethodGet, "/api/v1/ubl/"+id, token, "", "")
	if poll.Code != http.StatusOK {
		t.Fatalf("poll status = %d; body=%s", poll.Code, poll.Body.String())
	}
	var metadata map[string]any
	if err := json.Unmarshal(poll.Body.Bytes(), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["status"] != "pending" {
		t.Fatalf("status = %v", metadata["status"])
	}
	if _, ok := metadata["content"]; ok {
		t.Fatal("default status poll must not return raw XML content")
	}

	withContent := doRequest(handler, http.MethodGet, "/api/v1/ubl/"+id+"?includeContent=true", token, "", "")
	if withContent.Code != http.StatusOK {
		t.Fatalf("content status = %d; body=%s", withContent.Code, withContent.Body.String())
	}
	var full map[string]any
	if err := json.Unmarshal(withContent.Body.Bytes(), &full); err != nil {
		t.Fatal(err)
	}
	if full["content"] != validInvoiceXML {
		t.Fatal("explicit content retrieval did not return original XML")
	}
}

func TestGetUploadRejectsInvalidIncludeContent(t *testing.T) {
	t.Parallel()
	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)
	res := doRequest(handler, http.MethodGet, "/api/v1/ubl/00000000000000000000000000000000?includeContent=perhaps", token, "", "")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
}

func TestNewHandlerGetUnknownUploadReturnsNotFound(t *testing.T) {
	t.Parallel()
	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)
	res := doRequest(handler, http.MethodGet, "/api/v1/ubl/00000000000000000000000000000000", token, "", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d", res.Code)
	}
}

func TestNewHandlerRejectsRecognizedLocalNameInWrongNamespace(t *testing.T) {
	t.Parallel()
	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)
	for _, body := range []string{`<Invoice xmlns="urn:example:not-ubl"/>`, `<Invoice/>`} {
		res := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", token, body, "application/xml")
		if res.Code != http.StatusBadRequest {
			t.Fatalf("status = %d; body=%s", res.Code, res.Body.String())
		}
	}
}

func TestNewHandlerRejectsOversizedUploadDeterministically(t *testing.T) {
	t.Parallel()
	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)
	oversized := strings.Repeat("a", maxUploadBytes+1)
	res := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", token, oversized, "application/xml")
	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d", res.Code)
	}
}

func TestNewHandlerUploadErrorsAreJSON(t *testing.T) {
	t.Parallel()
	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)
	for _, body := range []string{"not xml", " ", "<NotAUBLDocument/>"} {
		res := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", token, body, "application/xml")
		if res.Code != http.StatusBadRequest || res.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("status=%d content-type=%q", res.Code, res.Header().Get("Content-Type"))
		}
		var payload map[string]any
		if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if _, ok := payload["error"].(string); !ok {
			t.Fatalf("missing error: %v", payload)
		}
	}
}

func TestNewHandlerMatchesOpenAPISpecification(t *testing.T) {
	t.Parallel()
	handler := newTestHandler(t, nil)
	res := doRequest(handler, http.MethodGet, "/spec", "", "", "")
	if res.Code != http.StatusOK {
		t.Fatalf("spec status = %d", res.Code)
	}
	var spec map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	paths := spec["paths"].(map[string]any)
	getUpload := paths["/api/v1/ubl/{id}"].(map[string]any)["get"].(map[string]any)
	params := getUpload["parameters"].([]any)
	found := false
	for _, raw := range params {
		p, ok := raw.(map[string]any)
		if ok && p["name"] == "includeContent" {
			found = true
		}
	}
	if !found {
		t.Fatal("OpenAPI GET upload operation missing includeContent parameter")
	}
}

func TestNewHandlerRejectsMissingBearerToken(t *testing.T) {
	t.Parallel()
	handler := newTestHandler(t, nil)
	res := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", "", validInvoiceXML, "application/xml")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.Code)
	}
}

func TestNewHandlerRejectsBadSignature(t *testing.T) {
	t.Parallel()
	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", "wrong-secret", time.Hour)
	if res := doRequest(handler, http.MethodGet, "/healthz", token, "", ""); res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.Code)
	}
}

func TestNewHandlerRejectsExpiredToken(t *testing.T) {
	t.Parallel()
	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, -time.Hour)
	if res := doRequest(handler, http.MethodGet, "/healthz", token, "", ""); res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.Code)
	}
}

func TestNewHandlerFailsClosedWithoutHMACSecret(t *testing.T) {
	t.Parallel()
	handler := newTestHandler(t, func(cfg *Config) { cfg.OAuthHMACSecret = "" })
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)
	if res := doRequest(handler, http.MethodGet, "/healthz", token, "", ""); res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", res.Code)
	}
}

func TestNewHandlerRateLimitsPerClient(t *testing.T) {
	t.Parallel()
	handler := newTestHandler(t, func(cfg *Config) { cfg.RateLimitRPS = 1; cfg.RateLimitBurst = 1 })
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)
	if res := doRequest(handler, http.MethodGet, "/healthz", token, "", ""); res.Code != http.StatusOK {
		t.Fatalf("first status = %d", res.Code)
	}
	if res := doRequest(handler, http.MethodGet, "/healthz", token, "", ""); res.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d", res.Code)
	}
}

func signedJWT(t *testing.T, issuer, audience, secret string, expiresIn time.Duration) string {
	t.Helper()
	claims := jwt.RegisteredClaims{Issuer: issuer, Audience: jwt.ClaimStrings{audience}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn))}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}
