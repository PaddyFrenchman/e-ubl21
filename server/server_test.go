package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	jwt "github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret"

// validInvoiceXML is a minimal but genuinely schema-valid UBL 2.1 Invoice.
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

// invalidInvoiceXML is well-formed and has a recognized root, but omits
// required UBL elements and must fail XSD validation in the worker.
const invalidInvoiceXML = `<?xml version="1.0" encoding="UTF-8"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
         xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
  <cbc:ID>INV-001</cbc:ID>
</Invoice>`

func testConfig(t *testing.T, overrides func(*Config)) Config {
	t.Helper()
	natsServer := queuetest.StartServer(t)
	cfg := Config{
		OpenAPIPath:     "/spec",
		SwaggerPath:     "/docs/",
		OAuthIssuer:     "https://issuer.example.com",
		OAuthAudience:   "e-ubl21",
		OAuthHMACSecret: testSecret,
		NATSURL:         natsServer.URL,
		NATSOptions:     natsServer.Options,
	}
	if overrides != nil {
		overrides(&cfg)
	}
	return cfg
}

func newTestHandler(t *testing.T, overrides func(*Config)) http.Handler {
	t.Helper()
	return newApplicationHandler(t, testConfig(t, overrides))
}

func newApplicationHandler(t *testing.T, cfg Config) http.Handler {
	t.Helper()
	application, err := NewApplication(cfg)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}
	t.Cleanup(application.Close)
	return application.Handler
}

func TestApplicationCloseReleasesNATSConnection(t *testing.T) {
	application, err := NewApplication(testConfig(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	connection := application.queue.NC
	application.Close()
	application.Close()
	if !connection.IsClosed() {
		t.Fatal("application Close did not close its NATS connection")
	}
}

func TestApplicationStartupReconcilesStaleUploadAcceptance(t *testing.T) {
	cfg := testConfig(t, nil)
	seed, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	if err := seed.EnsureInfra(context.Background()); err != nil {
		t.Fatal(err)
	}
	id, _ := queue.NewID()
	receivedAt := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	if err := seed.PutUploadRecord(context.Background(), queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusEnqueuePending, ReceivedAt: receivedAt}); err != nil {
		t.Fatal(err)
	}
	if err := seed.PutUploadContent(context.Background(), id, []byte(validInvoiceXML)); err != nil {
		t.Fatal(err)
	}
	seed.Close()

	application, err := NewApplication(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(application.Close)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		record, err := application.queue.GetUploadRecord(context.Background(), id)
		if err == nil && record.Status == queue.StatusPending {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("application startup did not reconcile stale enqueue_pending upload")
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

func TestNewHandlerQueuesUploadAndPersistsPendingRecord(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)

	res := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", token, validInvoiceXML, "application/xml")
	if res.Code != http.StatusAccepted {
		t.Fatalf("upload status = %d, want %d; body=%s", res.Code, http.StatusAccepted, res.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "pending" {
		t.Fatalf("status = %v, want %q", payload["status"], "pending")
	}
	if payload["document"] != "Invoice" {
		t.Fatalf("document = %v, want %q", payload["document"], "Invoice")
	}
	id, _ := payload["id"].(string)
	if id == "" {
		t.Fatal("missing id field")
	}

	// TODO-002: default polling is metadata-only. No worker is running, so
	// the record must remain pending and must not include the raw XML.
	getRes := doRequest(handler, http.MethodGet, "/api/v1/ubl/"+id, token, "", "")
	if getRes.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body=%s", getRes.Code, http.StatusOK, getRes.Body.String())
	}
	var stored map[string]any
	if err := json.Unmarshal(getRes.Body.Bytes(), &stored); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if stored["status"] != "pending" {
		t.Fatalf("stored status = %v, want %q", stored["status"], "pending")
	}
	if _, hasContent := stored["content"]; hasContent {
		t.Fatal("default status poll must not include raw XML content")
	}
	if _, hasValidatedAt := stored["validatedAt"]; hasValidatedAt {
		t.Fatal("validatedAt should not be set before the worker has processed the document")
	}

	// Explicit content retrieval preserves the original API capability
	// without burdening normal status polling.
	contentRes := doRequest(handler, http.MethodGet, "/api/v1/ubl/"+id+"?includeContent=true", token, "", "")
	if contentRes.Code != http.StatusOK {
		t.Fatalf("get content status = %d, want %d; body=%s", contentRes.Code, http.StatusOK, contentRes.Body.String())
	}
	var withContent map[string]any
	if err := json.Unmarshal(contentRes.Body.Bytes(), &withContent); err != nil {
		t.Fatalf("decode content response: %v", err)
	}
	if withContent["content"] != validInvoiceXML {
		t.Fatalf("stored content mismatch: got %v", withContent["content"])
	}
}

func TestNewHandlerGetRejectsInvalidIncludeContent(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)

	res := doRequest(handler, http.MethodGet, "/api/v1/ubl/00000000000000000000000000000000?includeContent=perhaps", token, "", "")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusBadRequest, res.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "includeContent must be true or false" {
		t.Fatalf("error = %v", payload["error"])
	}
}

func TestNewHandlerGetUnknownUploadReturnsNotFound(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)

	res := doRequest(handler, http.MethodGet, "/api/v1/ubl/00000000000000000000000000000000", token, "", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestNewHandlerRejectsUnsupportedDocumentType(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)

	res := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", token, "<NotAUBLDocument/>", "application/xml")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusBadRequest, res.Body.String())
	}
}

func TestNewHandlerRejectsRecognizedLocalNameInWrongNamespace(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)

	for _, tc := range []struct {
		name string
		body string
	}{
		{"wrong namespace", `<Invoice xmlns="urn:example:not-ubl"><cbc:ID>1</cbc:ID></Invoice>`},
		{"no namespace at all", `<Invoice><cbc:ID>1</cbc:ID></Invoice>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", token, tc.body, "application/xml")
			if res.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusBadRequest, res.Body.String())
			}
			var payload map[string]any
			if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			errMsg, _ := payload["error"].(string)
			if !strings.Contains(errMsg, "not a recognized main document") {
				t.Fatalf("error = %q, want it to mention the document type was not recognized", errMsg)
			}
		})
	}
}

func TestNewHandlerRejectsOversizedUploadDeterministically(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)

	oversized := strings.Repeat("a", maxUploadBytes+1)
	res := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", token, oversized, "application/xml")
	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusRequestEntityTooLarge, res.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("413 response body is not JSON: %v (body=%s)", err, res.Body.String())
	}
	if _, ok := payload["error"].(string); !ok {
		t.Fatalf("response body missing string \"error\" field: %v", payload)
	}
}

func TestNewHandlerUploadErrorsAreJSON(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)

	for _, tc := range []struct {
		name string
		body string
	}{
		{"invalid XML", "not xml"},
		{"empty body", " "},
		{"unsupported document type", "<NotAUBLDocument/>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", token, tc.body, "application/xml")
			if res.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusBadRequest, res.Body.String())
			}
			if ct := res.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("content-type = %q, want %q", ct, "application/json")
			}
			var payload map[string]any
			if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
				t.Fatalf("400 response body is not JSON: %v (body=%s)", err, res.Body.String())
			}
			if _, ok := payload["error"].(string); !ok {
				t.Fatalf("response body missing string \"error\" field: %v", payload)
			}
		})
	}
}

func TestNewHandlerReturnsMethodNotAllowedWithJSONBody(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{"upload wrong method", http.MethodPut, "/api/v1/ubl/upload"},
		{"get-upload wrong method", http.MethodPost, "/api/v1/ubl/some-id"},
		{"root wrong method", http.MethodPost, "/"},
		{"healthz wrong method", http.MethodPost, "/healthz"},
		{"metrics wrong method", http.MethodPost, "/metrics"},
		{"openapi wrong method", http.MethodPost, "/spec"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := doRequest(handler, tc.method, tc.path, token, "", "")
			if res.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusMethodNotAllowed, res.Body.String())
			}
			var payload map[string]any
			if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
				t.Fatalf("405 response body is not JSON: %v (body=%s)", err, res.Body.String())
			}
			if payload["error"] != "method not allowed" {
				t.Fatalf("error = %v, want %q", payload["error"], "method not allowed")
			}
		})
	}
}

func TestNewHandlerMatchesOpenAPISpecification(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, nil)
	specRes := doRequest(handler, http.MethodGet, "/spec", "", "", "")
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
	paths := spec["paths"].(map[string]any)
	if _, ok := paths["/api/v1/ubl/upload"]; !ok {
		t.Fatal("missing upload path in specification")
	}
	retrieval, ok := paths["/api/v1/ubl/{id}"].(map[string]any)
	if !ok {
		t.Fatal("missing retrieval path in specification")
	}
	get := retrieval["get"].(map[string]any)
	params := get["parameters"].([]any)
	foundIncludeContent := false
	for _, raw := range params {
		param, ok := raw.(map[string]any)
		if ok && param["name"] == "includeContent" {
			foundIncludeContent = true
		}
	}
	if !foundIncludeContent {
		t.Fatal("retrieval operation missing includeContent query parameter")
	}
	components := spec["components"].(map[string]any)
	securitySchemes := components["securitySchemes"].(map[string]any)
	if securitySchemes["oauth2"].(map[string]any)["type"] != "oauth2" {
		t.Fatal("oauth2 security scheme missing")
	}
}

func TestNewHandlerServesSwaggerUIPublicly(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, nil)
	res := doRequest(handler, http.MethodGet, "/docs/index.html", "", "", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if !strings.Contains(res.Body.String(), "/spec") {
		t.Fatal("swagger UI does not reference the configured OpenAPI path")
	}
}

func TestNewHandlerRejectsMissingBearerToken(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, nil)
	res := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", "", "<Invoice/>", "application/xml")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestNewHandlerRejectsBadSignature(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", "wrong-secret", time.Hour)
	res := doRequest(handler, http.MethodGet, "/healthz", token, "", "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestNewHandlerRejectsExpiredToken(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, -time.Hour)
	res := doRequest(handler, http.MethodGet, "/healthz", token, "", "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestNewHandlerFailsClosedWithoutHMACSecret(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, func(cfg *Config) { cfg.OAuthHMACSecret = "" })
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)
	res := doRequest(handler, http.MethodGet, "/healthz", token, "", "")
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusInternalServerError)
	}
}

func TestNewHandlerExposesMetrics(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, nil)
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)

	// Vector metrics only emit samples for observed label combinations.
	doRequest(handler, http.MethodGet, "/healthz", token, "", "")
	res := doRequest(handler, http.MethodGet, "/metrics", token, "", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if !strings.Contains(res.Body.String(), "e_ubl21_http_requests_total") {
		t.Fatal("metrics output missing expected counter")
	}
}

func TestNewHandlerRateLimitsPerClient(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, func(cfg *Config) {
		cfg.RateLimitRPS = 1
		cfg.RateLimitBurst = 1
	})
	token := signedJWT(t, "https://issuer.example.com", "e-ubl21", testSecret, time.Hour)

	first := doRequest(handler, http.MethodGet, "/healthz", token, "", "")
	if first.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want %d", first.Code, http.StatusOK)
	}
	second := doRequest(handler, http.MethodGet, "/healthz", token, "", "")
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
}

func signedJWT(t *testing.T, issuer, audience, secret string, expiresIn time.Duration) string {
	t.Helper()

	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Audience:  jwt.ClaimStrings{audience},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}
