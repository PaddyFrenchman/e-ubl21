package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentCatalogBuildsEntirelyInMemory(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv("TMPDIR", tempRoot)

	catalog, err := buildDocumentCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.entryByElement) == 0 {
		t.Fatal("document catalog is empty")
	}
	entries, err := os.ReadDir(tempRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("API document catalog wrote temporary files: %v", entries)
	}
}

func TestDocumentAndValidationCatalogsAgree(t *testing.T) {
	documents, err := buildDocumentCatalog()
	if err != nil {
		t.Fatal(err)
	}
	validation, err := buildValidationCatalog(documents)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(validation.dir) })

	if validation.documentCatalog != documents {
		t.Fatal("validation catalog does not share the authoritative document catalog")
	}
	for root, entry := range documents.entryByElement {
		namespace, ok := validation.expectedNamespaceFor(root)
		if !ok || namespace != entry.namespace {
			t.Errorf("namespace disagreement for %s: %q, %v", root, namespace, ok)
		}
		path, ok := validation.schemaPathFor(root)
		if !ok {
			t.Errorf("validation schema missing for %s", root)
			continue
		}
		if filepath.Base(path) != entry.file {
			t.Errorf("schema for %s = %s, want %s", root, path, entry.file)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("extracted schema for %s: %v", root, err)
		}
	}
	if path, ok := validation.schemaPathFor("NotAUBLDocument"); ok || path != "" {
		t.Fatalf("unknown schema = %q, %v", path, ok)
	}
}

func BenchmarkBuildDocumentCatalog(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := buildDocumentCatalog(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuildValidationCatalog(b *testing.B) {
	documents, err := buildDocumentCatalog()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		catalog, err := buildValidationCatalog(documents)
		if err != nil {
			b.Fatal(err)
		}
		if err := os.RemoveAll(catalog.dir); err != nil {
			b.Fatal(err)
		}
	}
}
