package regression_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/regression"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestContractExamplesValidate(t *testing.T) {
	tests := []struct {
		name     string
		schema   string
		instance string
	}{
		{name: "fixture manifest", schema: "fixture-manifest.schema.json", instance: "fixture-manifest.example.json"},
		{name: "normalized result", schema: "normalized-result.schema.json", instance: "normalized-result.example.json"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schema := compileSchema(t, test.schema)
			instance := readJSON(t, filepath.Join("testdata", test.instance))
			if err := schema.Validate(instance); err != nil {
				t.Fatalf("example does not satisfy %s: %v", test.schema, err)
			}
		})
	}
}

func TestFixtureManifestContractRejectsInvalidSemantics(t *testing.T) {
	schema := compileSchema(t, "fixture-manifest.schema.json")
	valid := readObject(t, filepath.Join("testdata", "fixture-manifest.example.json"))

	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{
			name: "unstable fixture id",
			mutate: func(document map[string]any) {
				firstFixture(document)["id"] = "EN16931/invoice/generated-at-2026-08-26"
			},
		},
		{
			name: "profile stage order mismatch",
			mutate: func(document map[string]any) {
				stages := firstFixture(document)["expectedStages"].([]any)
				stages[0].(map[string]any)["stage"] = "en16931"
			},
		},
		{
			name: "semantic failure without rule id",
			mutate: func(document map[string]any) {
				stages := firstFixture(document)["expectedStages"].([]any)
				delete(stages[1].(map[string]any), "ruleIds")
			},
		},
		{
			name: "valid final result with invalid stage",
			mutate: func(document map[string]any) {
				firstFixture(document)["expectedFinal"] = "valid"
			},
		},
		{
			name: "profile without pinned standard",
			mutate: func(document map[string]any) {
				firstFixture(document)["standards"].(map[string]any)["en16931"] = nil
			},
		},
		{
			name: "abbreviated generator commit",
			mutate: func(document map[string]any) {
				firstFixture(document)["provenance"].(map[string]any)["generatorVersion"] = "git:0123456"
			},
		},
		{
			name: "unstable mutation id",
			mutate: func(document map[string]any) {
				firstFixture(document)["mutation"] = map[string]any{
					"id": "Missing Required ID", "description": "remove ID",
				}
			},
		},
		{
			name: "import without immutable checksum",
			mutate: func(document map[string]any) {
				fixtures := document["fixtures"].([]any)
				delete(fixtures[1].(map[string]any)["provenance"].(map[string]any), "sourceSha256")
			},
		},
		{
			name: "import without extraction description",
			mutate: func(document map[string]any) {
				fixtures := document["fixtures"].([]any)
				delete(fixtures[1].(map[string]any)["provenance"].(map[string]any), "extraction")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneObject(t, valid)
			test.mutate(candidate)
			if err := schema.Validate(candidate); err == nil {
				t.Fatal("invalid manifest unexpectedly satisfied the schema")
			}
		})
	}
}

func TestManifestFixtureIDsAreUnique(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "fixture-manifest.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.ValidateFixtureManifest(data); err != nil {
		t.Fatalf("valid fixture manifest rejected: %v", err)
	}

	document := readObject(t, filepath.Join("testdata", "fixture-manifest.example.json"))
	fixtures := document["fixtures"].([]any)
	duplicate := cloneObject(t, fixtures[0].(map[string]any))
	document["fixtures"] = append(fixtures, duplicate)
	data, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	err = regression.ValidateFixtureManifest(data)
	if err == nil || !strings.Contains(err.Error(), `duplicate fixture id "en16931.invoice.br-co-15"`) {
		t.Fatalf("duplicate fixture ID error = %v", err)
	}
}

func TestManifestValidatorRejectsUnsortedRuleIDs(t *testing.T) {
	document := readObject(t, filepath.Join("testdata", "fixture-manifest.example.json"))
	stages := firstFixture(document)["expectedStages"].([]any)
	stages[1].(map[string]any)["ruleIds"] = []any{"BR-Z", "BR-A"}
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.ValidateFixtureManifest(data); err == nil || !strings.Contains(err.Error(), "sorted lexicographically") {
		t.Fatalf("unsorted fixture rule IDs error = %v", err)
	}
}

func TestManifestValidatorRejectsMalformedAndSchemaInvalidDocuments(t *testing.T) {
	if err := regression.ValidateFixtureManifest([]byte(`{`)); err == nil || !strings.Contains(err.Error(), "decode fixture manifest") {
		t.Fatalf("malformed manifest error = %v", err)
	}
	if err := regression.ValidateFixtureManifest([]byte(`{"schemaVersion":1,"fixtures":[]}`)); err == nil || !strings.Contains(err.Error(), "validate fixture manifest schema") {
		t.Fatalf("schema-invalid manifest error = %v", err)
	}
}

func TestNormalizedResultRejectsVolatileAndMisorderedFields(t *testing.T) {
	schema := compileSchema(t, "normalized-result.schema.json")
	valid := readObject(t, filepath.Join("testdata", "normalized-result.example.json"))

	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{
			name: "upload id",
			mutate: func(document map[string]any) {
				document["uploadId"] = "0123456789abcdef0123456789abcdef"
			},
		},
		{
			name: "duration",
			mutate: func(document map[string]any) {
				stages := document["actualStages"].([]any)
				stages[0].(map[string]any)["durationMs"] = float64(12)
			},
		},
		{
			name: "misordered actual stages",
			mutate: func(document map[string]any) {
				stages := document["actualStages"].([]any)
				stages[0], stages[1] = stages[1], stages[0]
			},
		},
		{
			name: "operational failure with standards rule ids",
			mutate: func(document map[string]any) {
				stages := document["actualStages"].([]any)
				stages[1].(map[string]any)["outcome"] = "operational-failure"
			},
		},
		{
			name: "valid final with incomplete stages",
			mutate: func(document map[string]any) {
				document["actualFinal"] = "valid"
				document["actualStages"] = document["actualStages"].([]any)[:1]
				document["result"] = "fail"
			},
		},
		{
			name: "invalid final with operational failure",
			mutate: func(document map[string]any) {
				stages := document["actualStages"].([]any)
				stage := stages[1].(map[string]any)
				stage["outcome"] = "operational-failure"
				delete(stage, "ruleIds")
			},
		},
		{
			name: "operational final with standards failure",
			mutate: func(document map[string]any) {
				document["actualFinal"] = "operational-failure"
				document["result"] = "fail"
			},
		},
		{
			name: "XSD failure followed by EN stage",
			mutate: func(document map[string]any) {
				stages := document["actualStages"].([]any)
				stages[0].(map[string]any)["outcome"] = "standards-invalid"
				stages[1].(map[string]any)["outcome"] = "passed"
				delete(stages[1].(map[string]any), "ruleIds")
			},
		},
		{
			name: "EN failure followed by Peppol stage",
			mutate: func(document map[string]any) {
				document["profile"] = "peppol-bis-billing"
				document["expectedFinal"] = "valid"
				document["expectedStages"] = []any{
					map[string]any{"stage": "ubl21-xsd", "outcome": "passed"},
					map[string]any{"stage": "en16931", "outcome": "passed"},
					map[string]any{"stage": "peppol-bis-billing", "outcome": "passed"},
				}
				document["actualStages"] = append(document["actualStages"].([]any),
					map[string]any{"stage": "peppol-bis-billing", "outcome": "passed"})
				document["result"] = "fail"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneObject(t, valid)
			test.mutate(candidate)
			if err := schema.Validate(candidate); err == nil {
				t.Fatal("non-normalized result unexpectedly satisfied the schema")
			}
		})
	}
}

func TestNormalizedResultValidatorEnforcesRuleIDOrdering(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "normalized-result.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.ValidateNormalizedResult(data); err != nil {
		t.Fatalf("valid normalized result rejected: %v", err)
	}
	sorted := readObject(t, filepath.Join("testdata", "normalized-result.example.json"))
	for _, field := range []string{"expectedStages", "actualStages"} {
		stages := sorted[field].([]any)
		stages[1].(map[string]any)["ruleIds"] = []any{"BR-A", "BR-Z"}
	}
	sortedData, err := json.Marshal(sorted)
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.ValidateNormalizedResult(sortedData); err != nil {
		t.Fatalf("sorted multi-rule normalized result rejected: %v", err)
	}

	for _, field := range []string{"expectedStages", "actualStages"} {
		t.Run(field, func(t *testing.T) {
			document := readObject(t, filepath.Join("testdata", "normalized-result.example.json"))
			stages := document[field].([]any)
			stages[1].(map[string]any)["ruleIds"] = []any{"BR-Z", "BR-A"}
			candidate, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			if err := regression.ValidateNormalizedResult(candidate); err == nil || !strings.Contains(err.Error(), "sorted lexicographically") {
				t.Fatalf("unsorted %s rule IDs error = %v", field, err)
			}
		})
	}
}

func TestNormalizedResultAllowsOperationalAndEarlyStandardsFailures(t *testing.T) {
	schema := compileSchema(t, "normalized-result.schema.json")
	valid := readObject(t, filepath.Join("testdata", "normalized-result.example.json"))

	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{
			name: "semantic validator operational failure",
			mutate: func(document map[string]any) {
				document["actualFinal"] = "operational-failure"
				document["result"] = "fail"
				stages := document["actualStages"].([]any)
				stage := stages[1].(map[string]any)
				stage["outcome"] = "operational-failure"
				delete(stage, "ruleIds")
			},
		},
		{
			name: "unexpected early XSD failure",
			mutate: func(document map[string]any) {
				document["actualStages"] = []any{
					map[string]any{"stage": "ubl21-xsd", "outcome": "standards-invalid"},
				}
				document["result"] = "fail"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneObject(t, valid)
			test.mutate(candidate)
			if err := schema.Validate(candidate); err != nil {
				t.Fatalf("representable regression failure does not satisfy the schema: %v", err)
			}
		})
	}
}

func compileSchema(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	path := filepath.Join("schemas", name)
	document := readJSON(t, path)
	if err := compiler.AddResource(name, document); err != nil {
		t.Fatalf("add schema resource %s: %v", path, err)
	}
	schema, err := compiler.Compile(name)
	if err != nil {
		t.Fatalf("compile schema %s: %v", path, err)
	}
	return schema
}

func readJSON(t *testing.T, path string) any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return value
}

func readObject(t *testing.T, path string) map[string]any {
	t.Helper()
	return readJSON(t, path).(map[string]any)
}

func cloneObject(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("clone value: %v", err)
	}
	var clone map[string]any
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatalf("decode cloned value: %v", err)
	}
	return clone
}

func firstFixture(document map[string]any) map[string]any {
	return document["fixtures"].([]any)[0].(map[string]any)
}
