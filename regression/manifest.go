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

//go:embed schemas/normalized-result.schema.json
var normalizedResultSchemaDocument []byte

var (
	fixtureManifestSchemaOnce  sync.Once
	fixtureManifestSchema      *jsonschema.Schema
	fixtureManifestSchemaErr   error
	normalizedResultSchemaOnce sync.Once
	normalizedResultSchema     *jsonschema.Schema
	normalizedResultSchemaErr  error
)

type stageRuleIDs struct {
	Stage   string   `json:"stage"`
	RuleIDs []string `json:"ruleIds"`
}

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
			ID             string         `json:"id"`
			ExpectedStages []stageRuleIDs `json:"expectedStages"`
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
		if err := validateSortedRuleIDs("fixture "+fixture.ID+" expected", fixture.ExpectedStages); err != nil {
			return err
		}
	}
	return nil
}

// ValidateNormalizedResult validates one results.jsonl record against its
// JSON schema and the lexicographical rule-ID ordering invariant that JSON
// Schema cannot express.
func ValidateNormalizedResult(data []byte) error {
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("decode normalized result: %w", err)
	}
	schema, err := compiledNormalizedResultSchema()
	if err != nil {
		return err
	}
	if err := schema.Validate(document); err != nil {
		return fmt.Errorf("validate normalized result schema: %w", err)
	}
	var result struct {
		ExpectedStages []stageRuleIDs `json:"expectedStages"`
		ActualStages   []stageRuleIDs `json:"actualStages"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("decode validated normalized result: %w", err)
	}
	if err := validateSortedRuleIDs("expected", result.ExpectedStages); err != nil {
		return err
	}
	return validateSortedRuleIDs("actual", result.ActualStages)
}

func validateSortedRuleIDs(scope string, stages []stageRuleIDs) error {
	for _, stage := range stages {
		for index := 1; index < len(stage.RuleIDs); index++ {
			if stage.RuleIDs[index-1] >= stage.RuleIDs[index] {
				return fmt.Errorf("%s stage %s ruleIds must be unique and sorted lexicographically", scope, stage.Stage)
			}
		}
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

func compiledNormalizedResultSchema() (*jsonschema.Schema, error) {
	normalizedResultSchemaOnce.Do(func() {
		var document any
		if err := json.Unmarshal(normalizedResultSchemaDocument, &document); err != nil {
			normalizedResultSchemaErr = fmt.Errorf("decode embedded normalized result schema: %w", err)
			return
		}
		compiler := jsonschema.NewCompiler()
		compiler.AssertFormat()
		const resourceName = "normalized-result.schema.json"
		if err := compiler.AddResource(resourceName, document); err != nil {
			normalizedResultSchemaErr = fmt.Errorf("add embedded normalized result schema: %w", err)
			return
		}
		normalizedResultSchema, normalizedResultSchemaErr = compiler.Compile(resourceName)
		if normalizedResultSchemaErr != nil {
			normalizedResultSchemaErr = fmt.Errorf("compile embedded normalized result schema: %w", normalizedResultSchemaErr)
		}
	})
	return normalizedResultSchema, normalizedResultSchemaErr
}
