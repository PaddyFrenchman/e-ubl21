package server

import (
	"encoding/xml"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRootElementOfPayloadReturnsLocalNameAndNamespace(t *testing.T) {
	t.Parallel()

	local, ns, err := rootElementOfPayload([]byte(validInvoiceXML))
	if err != nil {
		t.Fatalf("rootElementOfPayload: unexpected error: %v", err)
	}
	if local != "Invoice" {
		t.Fatalf("local name = %q, want %q", local, "Invoice")
	}
	if want := "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"; ns != want {
		t.Fatalf("namespace = %q, want %q", ns, want)
	}
}

func TestRootElementOfPayloadRejectsMalformedInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
	}{
		{"empty", ""},
		{"whitespace only", "   \n\t"},
		{"not xml", "this is not xml"},
		{"no root element", "<?xml version=\"1.0\"?>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := rootElementOfPayload([]byte(tc.body)); err == nil {
				t.Fatalf("rootElementOfPayload(%q): expected an error, got none", tc.body)
			}
		})
	}
}

// TestRootElementOfPayloadStopsAtFirstStartElement proves the acceptance
// criterion that root-document detection does not parse the entire XML
// tree: the payload's root element is well-formed, but everything after it
// is deliberately broken (mismatched/unclosed tags that would fail a
// full-document parse). rootElementOfPayload must still succeed, because it
// stops at the first xml.StartElement token rather than decoding the rest
// of the document.
func TestRootElementOfPayloadStopsAtFirstStartElement(t *testing.T) {
	t.Parallel()

	body := `<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2">` +
		`<cbc:ID xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">INV-1</cbc:ID` + // deliberately unclosed/malformed tail
		`<<< not valid xml at all, mismatched and unterminated`

	local, ns, err := rootElementOfPayload([]byte(body))
	if err != nil {
		t.Fatalf("rootElementOfPayload: unexpected error on a document with a malformed tail: %v", err)
	}
	if local != "Invoice" {
		t.Fatalf("local name = %q, want %q", local, "Invoice")
	}
	if want := "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"; ns != want {
		t.Fatalf("namespace = %q, want %q", ns, want)
	}

	// Confirm the malformed tail really would fail a full-document parse,
	// so this test is actually exercising the "stops early" behavior and
	// not merely tolerant XML.
	var full struct{}
	if err := xml.Unmarshal([]byte(body), &full); err == nil {
		t.Fatal("test setup invalid: the malformed tail should fail a full-document parse")
	}
}

func TestReadUploadBodyRejectsDeclaredOversizedContentLengthWithoutReading(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/api/v1/ubl/upload", strings.NewReader("<Invoice/>"))
	req.Header.Set("Content-Type", "application/xml")
	req.ContentLength = maxUploadBytes + 1

	rec := httptest.NewRecorder()
	if _, err := readUploadBody(rec, req); !errors.Is(err, errPayloadTooLarge) {
		t.Fatalf("readUploadBody error = %v, want %v", err, errPayloadTooLarge)
	}
}

func TestReadUploadBodyRejectsActualOversizedBodyWithUnknownContentLength(t *testing.T) {
	t.Parallel()

	oversized := strings.Repeat("a", maxUploadBytes+1)
	req := httptest.NewRequest("POST", "/api/v1/ubl/upload", strings.NewReader(oversized))
	req.Header.Set("Content-Type", "application/xml")
	// Simulate a body whose true size is not known up front (e.g.
	// chunked transfer encoding), so readUploadBody cannot reject it via
	// the cheap Content-Length pre-check and must rely on
	// http.MaxBytesReader while actually reading.
	req.ContentLength = -1

	rec := httptest.NewRecorder()
	if _, err := readUploadBody(rec, req); !errors.Is(err, errPayloadTooLarge) {
		t.Fatalf("readUploadBody error = %v, want %v", err, errPayloadTooLarge)
	}
}

func TestReadUploadBodyAcceptsBodyAtExactlyTheLimit(t *testing.T) {
	t.Parallel()

	exact := strings.Repeat("a", maxUploadBytes)
	req := httptest.NewRequest("POST", "/api/v1/ubl/upload", strings.NewReader(exact))
	req.Header.Set("Content-Type", "application/xml")

	rec := httptest.NewRecorder()
	body, err := readUploadBody(rec, req)
	if err != nil {
		t.Fatalf("readUploadBody: unexpected error for a body exactly at the limit: %v", err)
	}
	if len(body) != maxUploadBytes {
		t.Fatalf("body length = %d, want %d", len(body), maxUploadBytes)
	}
}
