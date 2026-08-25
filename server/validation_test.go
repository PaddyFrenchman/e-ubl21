package server

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

type boundedReadSizeReader struct {
	reader  io.Reader
	maxRead int
}

func (r *boundedReadSizeReader) Read(p []byte) (int, error) {
	if len(p) > r.maxRead {
		p = p[:r.maxRead]
	}
	return r.reader.Read(p)
}

func TestValidateAgainstSchemaUsesStdinWithoutPayloadTempFile(t *testing.T) {
	catalog, err := loadValidationCatalog()
	if err != nil {
		t.Fatal(err)
	}
	schemaPath, ok := catalog.schemaPathFor("Invoice")
	if !ok {
		t.Fatal("Invoice schema missing")
	}
	tempRoot := t.TempDir()
	t.Setenv("TMPDIR", tempRoot)
	if err := validateAgainstSchema(context.Background(), schemaPath, []byte(validInvoiceXML), 5*time.Second); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(tempRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("validation wrote per-document temp files: %v", entries)
	}
}

func TestValidateReaderAgainstSchemaStreamsInput(t *testing.T) {
	catalog, err := loadValidationCatalog()
	if err != nil {
		t.Fatal(err)
	}
	schemaPath, _ := catalog.schemaPathFor("Invoice")
	reader := &boundedReadSizeReader{reader: strings.NewReader(validInvoiceXML), maxRead: 7}
	if err := validateReaderAgainstSchema(context.Background(), schemaPath, reader, 5*time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestValidateAgainstSchemaHonorsTimeout(t *testing.T) {
	catalog, err := loadValidationCatalog()
	if err != nil {
		t.Fatal(err)
	}
	schemaPath, _ := catalog.schemaPathFor("Invoice")
	err = validateAgainstSchema(context.Background(), schemaPath, []byte(validInvoiceXML), time.Nanosecond)
	if !errors.Is(err, errValidationTimeout) {
		t.Fatalf("validation error = %v, want timeout", err)
	}
}

func TestValidateAgainstSchemaHonorsParentCancellation(t *testing.T) {
	catalog, err := loadValidationCatalog()
	if err != nil {
		t.Fatal(err)
	}
	schemaPath, _ := catalog.schemaPathFor("Invoice")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = validateAgainstSchema(ctx, schemaPath, []byte(validInvoiceXML), time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("validation error = %v, want cancellation", err)
	}
}

func TestNormalizeValidationDiagnostics(t *testing.T) {
	if got := normalizeValidationDiagnostics(nil); got != "xmllint exited without diagnostics" {
		t.Fatalf("empty diagnostic = %q", got)
	}
	if got := normalizeValidationDiagnostics([]byte("  useful error  \n")); got != "useful error" {
		t.Fatalf("trimmed diagnostic = %q", got)
	}
	buffer := &boundedDiagnosticBuffer{}
	payload := []byte(strings.Repeat("x", maxValidationDiagnosticBytes*2))
	written, err := buffer.Write(payload)
	if err != nil || written != len(payload) {
		t.Fatalf("bounded write = %d, %v", written, err)
	}
	if written, err := buffer.Write([]byte("discarded")); err != nil || written != len("discarded") {
		t.Fatalf("saturated write = %d, %v", written, err)
	}
	got := buffer.String()
	if len(got) != maxValidationDiagnosticBytes || !strings.HasSuffix(got, "... [truncated]") {
		t.Fatalf("bounded diagnostic length/suffix = %d / %q", len(got), got[len(got)-20:])
	}
}

func BenchmarkValidateAgainstSchemaProcess(b *testing.B) {
	catalog, err := loadValidationCatalog()
	if err != nil {
		b.Fatal(err)
	}
	if err := ensureXMLLint(); err != nil {
		b.Skip(err)
	}
	schemaPath, _ := catalog.schemaPathFor("Invoice")
	body := []byte(validInvoiceXML)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := validateAgainstSchema(context.Background(), schemaPath, body, 5*time.Second); err != nil {
			b.Fatal(err)
		}
	}
}
