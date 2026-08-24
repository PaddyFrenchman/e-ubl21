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

// validInvoiceXML is a minimal but genuinely schema-valid UBL 2.1 Invoice,
// satisfying every element UBL-Invoice-2.1.xsd requires (see
// createsamples/samples/Invoice for a richer, generator-produced example).
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

// invalidInvoiceXML is well-formed XML, and its root element ("Invoice") is
// a recognized UBL 2.1 document type, but it omits required elements (e.g.
// IssueDate, LegalMonetaryTotal, InvoiceLine), so it must fail XSD
// validation.
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

	// No worker is running in this test, so the record must still be
	// "pending" - this is the queued-but-not-yet-processed state the async
	// upload flow introduces (see worker_test.go for the full
	// upload -> worker -> validated pipeline).
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
	if stored["content"] != validInvoiceXML {
		t.Fatalf("stored content mismatch: got %v", stored["content"])
	}
	if _, hasValidatedAt := stored["validatedAt"]; hasValidatedAt {
		t.Fatal("validatedAt should not be set before the worker has processed the document")
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

// TestNewHandlerRejectsRecognizedLocalNameInWrongNamespace proves the
// TODO-005 namespace-recognition requirement: a root element whose local
// name matches a recognized UBL 2.1 document type but whose namespace does
// not match that type's UBL 2.1 XSD is rejected up front rather than
// accepted and left to fail asynchronously in the worker (see
// SERVER-DATAFLOW-2026-08-24.md section 11.11).
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

// TestNewHandlerRejectsOversizedUploadDeterministically proves the
// TODO-005 acceptance criterion that oversized uploads are rejected
// deterministically (413) rather than silently truncated to the size
// limit and processed as if that truncated prefix were the whole document.
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

// TestNewHandlerUploadErrorsAreJSON guards against a regression where
// POST /api/v1/ubl/upload's 400/500 responses were plain text (via
// http.Error) instead of the documented application/json ErrorResponse
// schema, unlike every other error response in the API.
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
	if _, ok := paths["/api/v1/ubl/{id}"]; !ok {
		t.Fatal("missing retrieval path in specification")
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

	// A CounterVec/HistogramVec only emits samples for label combinations
	// that have actually been observed at least once, so make one request
	// first to guarantee e_ubl21_http_requests_total has a sample by the
	// time /metrics itself is scraped.
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
