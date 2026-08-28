package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSchemaCatalogParsingErrorPaths(t *testing.T) {
	if _, _, err := rootElementOf(filepath.Join(t.TempDir(), "missing.xsd")); err == nil {
		t.Fatal("rootElementOf accepted a missing file")
	}
	if _, _, err := rootElementFromXSD([]byte("<schema>")); err == nil {
		t.Fatal("rootElementFromXSD accepted malformed XML")
	}
	root, namespace, err := rootElementFromXSD([]byte(`<schema targetNamespace="urn:test"><element/><element name="Invoice"/></schema>`))
	if err != nil || root != "Invoice" || namespace != "urn:test" {
		t.Fatalf("parsed root=%q namespace=%q err=%v", root, namespace, err)
	}
	root, namespace, err = rootElementFromXSD([]byte(`<schema targetNamespace="urn:test"/>`))
	if err != nil || root != "" || namespace != "" {
		t.Fatalf("empty schema root=%q namespace=%q err=%v", root, namespace, err)
	}
}

func TestMalformedXMLIsAStandardsInvalidXSDOutcome(t *testing.T) {
	schemaPath := filepath.Join("..", "XSD", "maindoc", "UBL-Invoice-2.1.xsd")
	err := validateReaderAgainstSchema(context.Background(), schemaPath, strings.NewReader("<Invoice>"), time.Second)
	if !errors.Is(err, errSchemaStandardsInvalid) || !strings.Contains(err.Error(), "parser error") {
		t.Fatalf("malformed XML classification = %v", err)
	}
}

func TestXMLLintParserErrorExitCodeIsStandardsInvalid(t *testing.T) {
	originalErr, originalPath := xmllintErr, xmllintPath
	t.Cleanup(func() {
		xmllintOnce, xmllintErr, xmllintPath = sync.Once{}, originalErr, originalPath
		xmllintOnce.Do(func() {})
	})

	fakeXMLLint := filepath.Join(t.TempDir(), "xmllint")
	if err := os.WriteFile(fakeXMLLint, []byte("#!/bin/sh\necho 'input.xml:1: parser error : malformed document' >&2\nexit 4\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	xmllintOnce, xmllintErr, xmllintPath = sync.Once{}, nil, fakeXMLLint
	xmllintOnce.Do(func() {})
	err := validateReaderAgainstSchema(context.Background(), "ignored.xsd", strings.NewReader("<Invoice>"), time.Second)
	if !errors.Is(err, errSchemaStandardsInvalid) || !strings.Contains(err.Error(), "parser error") {
		t.Fatalf("parser error exit classification = %v", err)
	}
}

func TestXMLLintDiscoveryAndStartFailures(t *testing.T) {
	originalErr, originalPath := xmllintErr, xmllintPath
	t.Cleanup(func() {
		xmllintOnce, xmllintErr, xmllintPath = sync.Once{}, originalErr, originalPath
		xmllintOnce.Do(func() {})
	})

	t.Setenv("PATH", t.TempDir())
	xmllintOnce, xmllintErr, xmllintPath = sync.Once{}, nil, ""
	if err := ensureXMLLint(); !errors.Is(err, errXMLLintUnavailable) {
		t.Fatalf("ensureXMLLint error = %v", err)
	}
	if err := ensureXMLLint(); !errors.Is(err, errXMLLintUnavailable) {
		t.Fatalf("cached ensureXMLLint error = %v", err)
	}

	xmllintOnce, xmllintErr, xmllintPath = sync.Once{}, nil, filepath.Join(t.TempDir(), "missing-xmllint")
	xmllintOnce.Do(func() {})
	if err := validateReaderAgainstSchema(context.Background(), "schema.xsd", strings.NewReader("<x/>"), 0); err == nil || !strings.Contains(err.Error(), "start schema validation") {
		t.Fatalf("start failure = %v", err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := validateReaderAgainstSchema(canceled, "schema.xsd", strings.NewReader("<x/>"), time.Second); err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("canceled start failure = %v", err)
	}

	expired, cancelDeadline := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelDeadline()
	if err := validateReaderAgainstSchema(expired, "schema.xsd", strings.NewReader("<x/>"), time.Second); !errors.Is(err, errValidationTimeout) {
		t.Fatalf("deadline start failure = %v", err)
	}
}
