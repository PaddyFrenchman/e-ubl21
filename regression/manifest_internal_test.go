package regression

import (
	"strings"
	"sync"
	"testing"
)

func TestEmbeddedManifestSchemaFailuresAreReported(t *testing.T) {
	originalDocument := append([]byte(nil), fixtureManifestSchemaDocument...)
	t.Cleanup(func() {
		fixtureManifestSchemaDocument = originalDocument
		fixtureManifestSchemaOnce = sync.Once{}
		fixtureManifestSchema = nil
		fixtureManifestSchemaErr = nil
	})

	reset := func(document string) {
		fixtureManifestSchemaDocument = []byte(document)
		fixtureManifestSchemaOnce = sync.Once{}
		fixtureManifestSchema = nil
		fixtureManifestSchemaErr = nil
	}
	reset("{")
	if _, err := compiledFixtureManifestSchema(); err == nil || !strings.Contains(err.Error(), "decode embedded") {
		t.Fatalf("malformed embedded schema error = %v", err)
	}
	reset(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":42}`)
	if _, err := compiledFixtureManifestSchema(); err == nil || !strings.Contains(err.Error(), "compile embedded") {
		t.Fatalf("invalid embedded schema error = %v", err)
	}
}
