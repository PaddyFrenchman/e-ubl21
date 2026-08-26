package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

type en16931ValidatorFunc func(context.Context, io.Reader) (queue.ValidationStageResult, error)

func (validate en16931ValidatorFunc) Validate(ctx context.Context, xml io.Reader) (queue.ValidationStageResult, error) {
	return validate(ctx, xml)
}

type memoryValidationContentSource struct {
	content []byte
	opens   int
	readers []*memoryValidationContentReader
	err     error
}

func (source *memoryValidationContentSource) OpenUploadContent(context.Context, string) (queue.UploadContentReader, error) {
	if source.err != nil {
		return nil, source.err
	}
	source.opens++
	reader := &memoryValidationContentReader{Reader: strings.NewReader(string(source.content))}
	source.readers = append(source.readers, reader)
	return reader, nil
}

type memoryValidationContentReader struct {
	*strings.Reader
	closed   bool
	err      error
	closeErr error
}

func (reader *memoryValidationContentReader) Close() error {
	reader.closed = true
	return reader.closeErr
}

func TestDocumentValidationPipelineDefaultsAndGuards(t *testing.T) {
	pipeline := newDocumentValidationPipelineWithTimeouts(&memoryValidationContentSource{}, nil, 0, 0)
	if pipeline.overallTimeout != defaultValidationTimeout || pipeline.xsdTimeout != defaultValidationTimeout {
		t.Fatalf("default timeouts = %s/%s", pipeline.overallTimeout, pipeline.xsdTimeout)
	}
	for name, candidate := range map[string]*documentValidationPipeline{
		"nil pipeline":      nil,
		"missing content":   {},
		"missing validator": {content: &memoryValidationContentSource{}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := candidate.Validate(context.Background(), "id", "Order", "xsd"); err == nil {
				t.Fatal("Validate accepted an unconfigured pipeline")
			}
		})
	}
	if _, err := pipeline.Validate(nil, "id", "Order", "xsd"); err == nil {
		t.Fatal("Validate accepted a nil context")
	}
}

func TestValidationContentStreamErrorsArePreserved(t *testing.T) {
	validationFailure := errors.New("validation failed")
	for _, test := range []struct {
		name       string
		reader     *memoryValidationContentReader
		validate   error
		wantJoined bool
	}{
		{name: "reader error", reader: &memoryValidationContentReader{Reader: strings.NewReader("x"), err: errors.New("read failed")}},
		{name: "close error", reader: &memoryValidationContentReader{Reader: strings.NewReader("x"), closeErr: errors.New("close failed")}},
		{name: "joined validation and stream errors", reader: &memoryValidationContentReader{Reader: strings.NewReader("x"), err: errors.New("read failed")}, validate: validationFailure, wantJoined: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := &singleValidationReaderSource{reader: test.reader}
			pipeline := &documentValidationPipeline{content: source}
			err := pipeline.withContent(context.Background(), "id", func(io.Reader) error { return test.validate })
			if !errors.Is(err, errValidationContentStream) || test.wantJoined && !errors.Is(err, validationFailure) {
				t.Fatalf("withContent error = %v", err)
			}
		})
	}
}

type singleValidationReaderSource struct{ reader queue.UploadContentReader }

func (source *singleValidationReaderSource) OpenUploadContent(context.Context, string) (queue.UploadContentReader, error) {
	return source.reader, nil
}

func TestEN16931InvalidMessageFallbacks(t *testing.T) {
	tests := []struct {
		issue queue.ValidationIssue
		want  string
	}{
		{issue: queue.ValidationIssue{Kind: queue.ValidationIssueFailedAssert, Severity: "error", Rule: "BR-1"}, want: "EN 16931 rule BR-1 failed"},
		{issue: queue.ValidationIssue{Kind: queue.ValidationIssueFailedAssert, Severity: "fatal", Message: "bad value"}, want: "EN 16931 validation failed: bad value"},
		{issue: queue.ValidationIssue{Kind: queue.ValidationIssueSuccessfulReport, Severity: "error", Message: "ignored"}, want: "EN 16931 validation failed"},
	}
	for _, test := range tests {
		if got := en16931InvalidMessage(queue.ValidationStageResult{Issues: []queue.ValidationIssue{test.issue}}); got != test.want {
			t.Errorf("en16931InvalidMessage(%+v) = %q, want %q", test.issue, got, test.want)
		}
	}
}

func (reader *memoryValidationContentReader) Error() error { return reader.err }

func TestDocumentValidationPipelineOrdersStagesAndReopensContent(t *testing.T) {
	source := &memoryValidationContentSource{content: []byte(`<Invoice/>`)}
	var calls []string
	pipeline := newDocumentValidationPipeline(source, en16931ValidatorFunc(func(_ context.Context, xml io.Reader) (queue.ValidationStageResult, error) {
		calls = append(calls, "en16931")
		if body, err := io.ReadAll(xml); err != nil || string(body) != `<Invoice/>` {
			t.Fatalf("EN input = %q, %v", body, err)
		}
		return queue.ValidationStageResult{Layer: queue.ValidationLayerEN16931, Version: "1.3.16", Valid: true, ValidatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
	}), time.Second)
	pipeline.xsdValidate = func(_ context.Context, _ string, xml io.Reader, _ time.Duration) error {
		calls = append(calls, "xsd")
		if body, err := io.ReadAll(xml); err != nil || string(body) != `<Invoice/>` {
			t.Fatalf("XSD input = %q, %v", body, err)
		}
		return nil
	}

	result, err := pipeline.Validate(context.Background(), "upload", "Invoice", "invoice.xsd")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != queue.StatusValid || len(result.Stages) != 2 ||
		result.Stages[0].Layer != queue.ValidationLayerUBL21XSD ||
		result.Stages[1].Layer != queue.ValidationLayerEN16931 {
		t.Fatalf("result = %+v", result)
	}
	if strings.Join(calls, ",") != "xsd,en16931" || source.opens != 2 {
		t.Fatalf("calls=%v opens=%d, want ordered stages with two independent streams", calls, source.opens)
	}
	for index, reader := range source.readers {
		if !reader.closed {
			t.Fatalf("content reader %d was not closed", index)
		}
	}
}

func TestDocumentValidationPipelineShortCircuitsAfterXSDInvalid(t *testing.T) {
	source := &memoryValidationContentSource{content: []byte(`<Invoice/>`)}
	enCalled := false
	pipeline := newDocumentValidationPipeline(source, en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		enCalled = true
		return queue.ValidationStageResult{}, nil
	}), time.Second)
	pipeline.xsdValidate = func(context.Context, string, io.Reader, time.Duration) error {
		return fmt.Errorf("%w: missing required element", errSchemaStandardsInvalid)
	}

	result, err := pipeline.Validate(context.Background(), "upload", "Invoice", "invoice.xsd")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != queue.StatusInvalid || enCalled || source.opens != 1 || len(result.Stages) != 1 || result.Stages[0].Valid {
		t.Fatalf("result=%+v enCalled=%t opens=%d", result, enCalled, source.opens)
	}
	if len(result.Stages[0].Issues) != 1 || result.Stages[0].Issues[0].Kind != queue.ValidationIssueXSDReport {
		t.Fatalf("XSD issues = %+v", result.Stages[0].Issues)
	}
}

func TestDocumentValidationPipelineRetainsENInvalidIssues(t *testing.T) {
	source := &memoryValidationContentSource{content: []byte(`<Invoice/>`)}
	pipeline := newDocumentValidationPipeline(source, en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		return queue.ValidationStageResult{
			Layer: queue.ValidationLayerEN16931, Version: "1.3.16", Valid: false,
			Issues:      []queue.ValidationIssue{{Kind: queue.ValidationIssueFailedAssert, Layer: queue.ValidationLayerEN16931, Rule: "BR-01", Severity: "error", Message: "Identifier missing"}},
			ValidatedAt: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}), time.Second)
	pipeline.xsdValidate = func(context.Context, string, io.Reader, time.Duration) error { return nil }

	result, err := pipeline.Validate(context.Background(), "upload", "Invoice", "invoice.xsd")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != queue.StatusInvalid || len(result.Stages) != 2 || result.Error != "EN 16931 rule BR-01 failed: Identifier missing" {
		t.Fatalf("result = %+v", result)
	}
}

func TestDocumentValidationPipelineSkipsENForStructuralUBL(t *testing.T) {
	source := &memoryValidationContentSource{content: []byte(`<Order/>`)}
	enCalled := false
	pipeline := newDocumentValidationPipeline(source, en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		enCalled = true
		return queue.ValidationStageResult{}, nil
	}), time.Second)
	pipeline.xsdValidate = func(context.Context, string, io.Reader, time.Duration) error { return nil }

	result, err := pipeline.Validate(context.Background(), "upload", "Order", "order.xsd")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != queue.StatusValid || len(result.Stages) != 1 || enCalled || source.opens != 1 {
		t.Fatalf("result=%+v enCalled=%t opens=%d", result, enCalled, source.opens)
	}
}

func TestDocumentValidationPipelineRequiresENForBillingDocuments(t *testing.T) {
	for _, documentType := range []string{"Invoice", "CreditNote"} {
		t.Run(documentType, func(t *testing.T) {
			source := &memoryValidationContentSource{content: []byte("<" + documentType + "/>")}
			pipeline := newDocumentValidationPipeline(source, nil, time.Second)
			pipeline.xsdValidate = func(context.Context, string, io.Reader, time.Duration) error { return nil }

			result, err := pipeline.Validate(context.Background(), "upload", documentType, "billing.xsd")
			if err == nil || !strings.Contains(err.Error(), "EN 16931 validator is required for "+documentType) || !reflect.DeepEqual(result, documentValidationResult{}) {
				t.Fatalf("result/error = %+v / %v", result, err)
			}
			if source.opens != 1 || !source.readers[0].closed {
				t.Fatalf("XSD stream lifecycle: opens=%d readers=%+v", source.opens, source.readers)
			}
		})
	}

	t.Run("Order remains XSD-only", func(t *testing.T) {
		source := &memoryValidationContentSource{content: []byte(`<Order/>`)}
		pipeline := newDocumentValidationPipeline(source, nil, time.Second)
		pipeline.xsdValidate = func(context.Context, string, io.Reader, time.Duration) error { return nil }
		result, err := pipeline.Validate(context.Background(), "upload", "Order", "order.xsd")
		if err != nil || result.Status != queue.StatusValid || len(result.Stages) != 1 {
			t.Fatalf("result/error = %+v / %v", result, err)
		}
	})
}

func TestDocumentValidationPipelineReturnsOperationalFailures(t *testing.T) {
	t.Run("XSD process failure", func(t *testing.T) {
		source := &memoryValidationContentSource{content: []byte(`<Invoice/>`)}
		pipeline := newDocumentValidationPipeline(source, nil, time.Second)
		pipeline.xsdValidate = func(context.Context, string, io.Reader, time.Duration) error {
			return errors.New("xmllint crashed")
		}
		result, err := pipeline.Validate(context.Background(), "upload", "Invoice", "invoice.xsd")
		if err == nil || !strings.Contains(err.Error(), "xmllint crashed") || !reflect.DeepEqual(result, documentValidationResult{}) {
			t.Fatalf("result/error = %+v / %v", result, err)
		}
	})

	t.Run("content unavailable", func(t *testing.T) {
		pipeline := newDocumentValidationPipeline(&memoryValidationContentSource{err: errors.New("object store offline")}, nil, time.Second)
		_, err := pipeline.Validate(context.Background(), "upload", "Order", "order.xsd")
		if !errors.Is(err, errValidationContentStream) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("EN runtime failure", func(t *testing.T) {
		source := &memoryValidationContentSource{content: []byte(`<Invoice/>`)}
		pipeline := newDocumentValidationPipeline(source, en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
			return queue.ValidationStageResult{}, errors.New("Saxon crashed")
		}), time.Second)
		pipeline.xsdValidate = func(context.Context, string, io.Reader, time.Duration) error { return nil }
		result, err := pipeline.Validate(context.Background(), "upload", "Invoice", "invoice.xsd")
		if err == nil || !strings.Contains(err.Error(), "Saxon crashed") || !reflect.DeepEqual(result, documentValidationResult{}) {
			t.Fatalf("result/error = %+v / %v", result, err)
		}
		if source.opens != 2 || !source.readers[1].closed {
			t.Fatalf("EN content stream was not reopened and closed: opens=%d", source.opens)
		}
	})
}

func TestDocumentValidationPipelineUsesDistinctXSDAndOverallTimeouts(t *testing.T) {
	source := &memoryValidationContentSource{content: []byte(`<Invoice/>`)}
	var receivedXSDTimeout time.Duration
	pipeline := newDocumentValidationPipelineWithTimeouts(source, en16931ValidatorFunc(func(ctx context.Context, _ io.Reader) (queue.ValidationStageResult, error) {
		<-ctx.Done()
		return queue.ValidationStageResult{}, ctx.Err()
	}), 30*time.Millisecond, 7*time.Millisecond)
	pipeline.xsdValidate = func(_ context.Context, _ string, _ io.Reader, timeout time.Duration) error {
		receivedXSDTimeout = timeout
		return nil
	}
	started := time.Now()
	result, err := pipeline.Validate(context.Background(), "upload", "Invoice", "invoice.xsd")
	if err == nil || !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(result, documentValidationResult{}) {
		t.Fatalf("result/error = %+v / %v", result, err)
	}
	if receivedXSDTimeout != 7*time.Millisecond {
		t.Fatalf("XSD timeout = %s, want 7ms", receivedXSDTimeout)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("overall pipeline timeout returned after %s", elapsed)
	}
}
