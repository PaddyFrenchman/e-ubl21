// This file resolves an uploaded UBL document's root element to its
// authoritative main-document XSD (embedded via the XSD/embed.go package)
// and performs real schema validation against it using the xmllint CLI
// (part of libxml2), the same independent validation oracle used by
// createsamples (see .github/create-samples.md section 7). xmllint operates
// on real files, so the embedded schema tree is extracted to a temporary
// directory once at startup.
package server

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	xsdschemas "github.com/PaddyFrenchman/e-ubl21/XSD"
)

// schemaCatalog resolves UBL 2.1 root element names to the on-disk path of
// their main-document XSD, once the embedded schema tree has been extracted.
type schemaCatalog struct {
	dir           string
	pathByElement map[string]string
}

type xsdRootElement struct {
	XMLName  xml.Name        `xml:"schema"`
	Elements []xsdElementRef `xml:"element"`
}

type xsdElementRef struct {
	Name string `xml:"name,attr"`
}

var (
	catalogOnce  sync.Once
	catalogValue *schemaCatalog
	catalogErr   error
	xmllintOnce  sync.Once
	xmllintErr   error
	xmllintPath  string
)

// errXMLLintUnavailable is wrapped into the error returned by
// validateAgainstSchema when the xmllint binary cannot be found, so callers
// can distinguish "server misconfigured" (500) from "document is invalid"
// (400) with errors.Is.
var errXMLLintUnavailable = errors.New("xmllint (libxml2) is required for UBL 2.1 schema validation; install it and restart the server")

// loadSchemaCatalog extracts the embedded XSD tree to a temporary directory
// and indexes every maindoc/*.xsd by its declared root element, memoizing
// the result for the lifetime of the process.
func loadSchemaCatalog() (*schemaCatalog, error) {
	catalogOnce.Do(func() {
		catalogValue, catalogErr = buildSchemaCatalog()
	})
	return catalogValue, catalogErr
}

func buildSchemaCatalog() (*schemaCatalog, error) {
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

	mainDir := filepath.Join(dir, "maindoc")
	entries, err := os.ReadDir(mainDir)
	if err != nil {
		return nil, fmt.Errorf("list extracted maindoc schemas: %w", err)
	}

	catalog := &schemaCatalog{dir: dir, pathByElement: map[string]string{}}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".xsd") {
			continue
		}
		path := filepath.Join(mainDir, entry.Name())
		root, err := rootElementOf(path)
		if err != nil {
			return nil, fmt.Errorf("parse root element of %s: %w", entry.Name(), err)
		}
		if root != "" {
			catalog.pathByElement[root] = path
		}
	}
	return catalog, nil
}

func rootElementOf(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var doc xsdRootElement
	if err := xml.Unmarshal(content, &doc); err != nil {
		return "", err
	}
	for _, element := range doc.Elements {
		if element.Name != "" {
			return element.Name, nil
		}
	}
	return "", nil
}

// schemaPathFor returns the extracted main-document XSD path for rootElement,
// or false if rootElement is not a recognized UBL 2.1 document type.
func (c *schemaCatalog) schemaPathFor(rootElement string) (string, bool) {
	path, ok := c.pathByElement[rootElement]
	return path, ok
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

// validateAgainstSchema writes body to a temporary file and validates it
// with `xmllint --noout --schema schemaPath`, returning a trimmed,
// human-readable error combining stdout/stderr on failure.
func validateAgainstSchema(schemaPath string, body []byte) error {
	if err := ensureXMLLint(); err != nil {
		return err
	}

	tmp, err := os.CreateTemp("", "e-ubl21-upload-*.xml")
	if err != nil {
		return fmt.Errorf("create temp upload file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp upload file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp upload file: %w", err)
	}

	cmd := exec.Command(xmllintPath, "--noout", "--schema", schemaPath, tmp.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schema validation failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}
