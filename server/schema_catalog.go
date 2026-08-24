// This file resolves an uploaded UBL document's root element to its
// authoritative main-document XSD (embedded via the XSD/embed.go package)
// and performs real schema validation against it using the xmllint CLI.
// API startup reads only maindoc metadata into memory. Because xmllint needs
// real paths for XSD imports, only worker startup extracts the complete
// embedded tree to a temporary directory.
package server

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	xsdschemas "github.com/PaddyFrenchman/e-ubl21/XSD"
)

// documentCatalog is the API's lightweight, in-memory index of supported
// document roots. Building it reads only embedded maindoc metadata and does
// not create files or directories.
type documentCatalog struct {
	entryByElement map[string]schemaEntry
}

// validationCatalog is the worker-only view. It shares the authoritative
// document metadata and adds the extracted filesystem root needed by
// xmllint for resolving imported schemas.
type validationCatalog struct {
	*documentCatalog
	dir string
}

// schemaEntry is a single recognized UBL 2.1 main document type: its
// extracted XSD path and the target namespace that document's root element
// must be declared in (see schemaCatalog.expectedNamespaceFor).
type schemaEntry struct {
	file      string
	namespace string
}

type xsdRootElement struct {
	XMLName         xml.Name        `xml:"schema"`
	TargetNamespace string          `xml:"targetNamespace,attr"`
	Elements        []xsdElementRef `xml:"element"`
}

type xsdElementRef struct {
	Name string `xml:"name,attr"`
}

var (
	documentCatalogOnce    sync.Once
	documentCatalogValue   *documentCatalog
	documentCatalogErr     error
	validationCatalogOnce  sync.Once
	validationCatalogValue *validationCatalog
	validationCatalogErr   error
	xmllintOnce            sync.Once
	xmllintErr             error
	xmllintPath            string
)

// errXMLLintUnavailable is wrapped into the error returned by
// validateAgainstSchema when the xmllint binary cannot be found, so callers
// can distinguish "server misconfigured" (500) from "document is invalid"
// (400) with errors.Is.
var errXMLLintUnavailable = errors.New("xmllint (libxml2) is required for UBL 2.1 schema validation; install it and restart the server")
var errValidationTimeout = errors.New("schema validation timed out")

// loadDocumentCatalog indexes embedded maindoc metadata in memory. This is
// the only catalog the API loads, so API startup performs no XSD extraction.
func loadDocumentCatalog() (*documentCatalog, error) {
	documentCatalogOnce.Do(func() {
		documentCatalogValue, documentCatalogErr = buildDocumentCatalog()
	})
	return documentCatalogValue, documentCatalogErr
}

func buildDocumentCatalog() (*documentCatalog, error) {
	entries, err := fs.ReadDir(xsdschemas.FS, "maindoc")
	if err != nil {
		return nil, fmt.Errorf("list embedded maindoc schemas: %w", err)
	}
	catalog := &documentCatalog{entryByElement: map[string]schemaEntry{}}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".xsd") {
			continue
		}
		content, err := xsdschemas.FS.ReadFile(filepath.Join("maindoc", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read embedded %s: %w", entry.Name(), err)
		}
		root, namespace, err := rootElementFromXSD(content)
		if err != nil {
			return nil, fmt.Errorf("parse root element of %s: %w", entry.Name(), err)
		}
		if root != "" {
			catalog.entryByElement[root] = schemaEntry{file: entry.Name(), namespace: namespace}
		}
	}
	return catalog, nil
}

// loadValidationCatalog extracts the complete embedded schema tree only for
// workers that perform xmllint validation.
func loadValidationCatalog() (*validationCatalog, error) {
	validationCatalogOnce.Do(func() {
		documents, err := loadDocumentCatalog()
		if err != nil {
			validationCatalogErr = err
			return
		}
		validationCatalogValue, validationCatalogErr = buildValidationCatalog(documents)
	})
	return validationCatalogValue, validationCatalogErr
}

func buildValidationCatalog(documents *documentCatalog) (*validationCatalog, error) {
	dir, err := os.MkdirTemp("", "e-ubl21-xsd-*")
	if err != nil {
		return nil, fmt.Errorf("create schema extraction dir: %w", err)
	}

	if err := fs.WalkDir(xsdschemas.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dir, path), 0o755)
		}
		content, err := xsdschemas.FS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		dest := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dest, content, 0o644)
	}); err != nil {
		return nil, fmt.Errorf("extract embedded XSD tree: %w", err)
	}

	return &validationCatalog{documentCatalog: documents, dir: dir}, nil
}

// rootElementOf returns the UBL main-document XSD at path's declared root
// element local name and the schema's targetNamespace (the namespace that
// element must be qualified with).
func rootElementOf(path string) (localName, namespace string, err error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	return rootElementFromXSD(content)
}

func rootElementFromXSD(content []byte) (localName, namespace string, err error) {
	var doc xsdRootElement
	if err := xml.Unmarshal(content, &doc); err != nil {
		return "", "", err
	}
	for _, element := range doc.Elements {
		if element.Name != "" {
			return element.Name, doc.TargetNamespace, nil
		}
	}
	return "", "", nil
}

// schemaPathFor returns the extracted main-document XSD path for rootElement,
// or false if rootElement is not a recognized UBL 2.1 document type.
func (c *validationCatalog) schemaPathFor(rootElement string) (string, bool) {
	entry, ok := c.entryByElement[rootElement]
	if !ok {
		return "", false
	}
	return filepath.Join(c.dir, "maindoc", entry.file), ok
}

// expectedNamespaceFor returns the UBL 2.1 target namespace rootElement's
// XSD requires its root element to be qualified with (e.g.
// "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2" for "Invoice"),
// or false if rootElement is not a recognized UBL 2.1 document type. Used
// by the upload handler to reject a document whose root local name happens
// to match a recognized type but whose namespace does not - see
// SERVER-DATAFLOW-2026-08-24.md section 11.11.
func (c *documentCatalog) expectedNamespaceFor(rootElement string) (string, bool) {
	entry, ok := c.entryByElement[rootElement]
	return entry.namespace, ok
}

// ensureXMLLint verifies the xmllint binary is available on PATH, once.
func ensureXMLLint() error {
	xmllintOnce.Do(func() {
		path, err := exec.LookPath("xmllint")
		if err != nil {
			xmllintErr = fmt.Errorf("%w: %v", errXMLLintUnavailable, err)
			return
		}
		xmllintPath = path
	})
	return xmllintErr
}

const maxValidationDiagnosticBytes = 8 << 10

// validateAgainstSchema streams body to xmllint over stdin. CommandContext
// enforces a hard per-document deadline, while --nonet prevents schemas from
// fetching network resources during validation.
func validateAgainstSchema(ctx context.Context, schemaPath string, body []byte, timeout time.Duration) error {
	if err := ensureXMLLint(); err != nil {
		return err
	}
	if timeout <= 0 {
		timeout = defaultValidationTimeout
	}
	validationCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(validationCtx, xmllintPath, "--nonet", "--noout", "--schema", schemaPath, "-")
	cmd.Stdin = bytes.NewReader(body)
	diagnostics := &boundedDiagnosticBuffer{}
	cmd.Stdout = diagnostics
	cmd.Stderr = diagnostics
	err := cmd.Run()
	if errors.Is(validationCtx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%w after %s", errValidationTimeout, timeout)
	}
	if errors.Is(validationCtx.Err(), context.Canceled) {
		return fmt.Errorf("schema validation canceled: %w", validationCtx.Err())
	}
	if err != nil {
		return fmt.Errorf("schema validation failed: %s", diagnostics.String())
	}
	return nil
}

type boundedDiagnosticBuffer struct {
	data      []byte
	truncated bool
}

func (b *boundedDiagnosticBuffer) Write(p []byte) (int, error) {
	written := len(p)
	remaining := maxValidationDiagnosticBytes - len(b.data)
	if remaining <= 0 {
		b.truncated = true
		return written, nil
	}
	if len(p) > remaining {
		b.data = append(b.data, p[:remaining]...)
		b.truncated = true
		return written, nil
	}
	b.data = append(b.data, p...)
	return written, nil
}

func (b *boundedDiagnosticBuffer) String() string {
	diagnostic := normalizeValidationDiagnostics(b.data)
	if !b.truncated {
		return diagnostic
	}
	const suffix = "... [truncated]"
	if len(diagnostic) > maxValidationDiagnosticBytes-len(suffix) {
		diagnostic = diagnostic[:maxValidationDiagnosticBytes-len(suffix)]
	}
	return diagnostic + suffix
}

func normalizeValidationDiagnostics(output []byte) string {
	diagnostic := strings.TrimSpace(string(output))
	if diagnostic == "" {
		return "xmllint exited without diagnostics"
	}
	return diagnostic
}
