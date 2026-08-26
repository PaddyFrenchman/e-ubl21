// Package regression defines the stable corpus and result contracts used by
// regression generators and runners.
package regression

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schemas/fixture-manifest.schema.json
var fixtureManifestSchemaDocument []byte

var (
	fixtureManifestSchemaOnce sync.Once
	fixtureManifestSchema     *jsonschema.Schema
	fixtureManifestSchemaErr  error
)

// ValidateFixtureManifest validates a complete manifest against the versioned
// JSON schema and the cross-entry fixture-ID uniqueness rule that JSON Schema
// cannot express.
func ValidateFixtureManifest(data []byte) error {
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("decode fixture manifest: %w", err)
	}

	schema, err := compiledFixtureManifestSchema()
	if err != nil {
		return err
	}
	if err := schema.Validate(document); err != nil {
		return fmt.Errorf("validate fixture manifest schema: %w", err)
	}

	var manifest struct {
		Fixtures []struct {
			ID string `json:"id"`
		} `json:"fixtures"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("decode validated fixture manifest: %w", err)
	}
	seen := make(map[string]struct{}, len(manifest.Fixtures))
	for _, fixture := range manifest.Fixtures {
		if _, duplicate := seen[fixture.ID]; duplicate {
			return fmt.Errorf("duplicate fixture id %q", fixture.ID)
		}
		seen[fixture.ID] = struct{}{}
	}
	return nil
}

func compiledFixtureManifestSchema() (*jsonschema.Schema, error) {
	fixtureManifestSchemaOnce.Do(func() {
		var document any
		if err := json.Unmarshal(fixtureManifestSchemaDocument, &document); err != nil {
			fixtureManifestSchemaErr = fmt.Errorf("decode embedded fixture manifest schema: %w", err)
			return
		}
		compiler := jsonschema.NewCompiler()
		compiler.AssertFormat()
		const resourceName = "fixture-manifest.schema.json"
		if err := compiler.AddResource(resourceName, document); err != nil {
			fixtureManifestSchemaErr = fmt.Errorf("add embedded fixture manifest schema: %w", err)
			return
		}
		fixtureManifestSchema, fixtureManifestSchemaErr = compiler.Compile(resourceName)
		if fixtureManifestSchemaErr != nil {
			fixtureManifestSchemaErr = fmt.Errorf("compile embedded fixture manifest schema: %w", fixtureManifestSchemaErr)
		}
	})
	return fixtureManifestSchema, fixtureManifestSchemaErr
}
