package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMarshalCoverageIncludesCompactFixtureTargetMappings(t *testing.T) {
	report := schemaCoverage{
		FixtureObservations: map[string]fixtureObservation{"sample.xml": {}},
		FixtureTargets:      map[string][]string{"sample.xml": {"z-target", "a-target"}},
	}
	data, err := marshalCoverage(report)
	if err != nil {
		t.Fatal(err)
	}
	var encoded struct {
		FixtureObservations  map[string]fixtureObservation `json:"fixtureObservations"`
		TargetCatalog        []string                      `json:"targetCatalog"`
		FixtureTargetIndexes map[string][]int              `json:"fixtureTargetIndexes"`
	}
	if err := json.Unmarshal(data, &encoded); err != nil {
		t.Fatal(err)
	}
	if len(encoded.FixtureObservations) != 1 {
		t.Fatalf("fixture observations = %#v, want sample.xml", encoded.FixtureObservations)
	}
	if got := encoded.TargetCatalog; len(got) != 2 || got[0] != "a-target" || got[1] != "z-target" {
		t.Fatalf("target catalog = %#v, want sorted targets", got)
	}
	if got := encoded.FixtureTargetIndexes["sample.xml"]; len(got) != 2 || got[0] != 1 || got[1] != 0 {
		t.Fatalf("fixture target indexes = %#v, want [1 0]", got)
	}
}

func TestFixtureReportPathIsRelativeToCorpusRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "corpus")
	path := filepath.Join(root, "Invoice", "Invoice-minimal.xml")
	got, err := fixtureReportPath(root, path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Invoice/Invoice-minimal.xml" {
		t.Fatalf("fixture report path = %q", got)
	}
	if _, err := fixtureReportPath(root, filepath.Join(filepath.Dir(root), "outside.xml")); err == nil {
		t.Fatal("expected outside-corpus fixture path rejection")
	}
}

func TestSchemaCoverageIsDeterministicAndIncludesNestedParticles(t *testing.T) {
	xsd := filepath.Join("..", "XSD")
	samples := "samples"
	index, err := buildSchemaIndex(xsd)
	if err != nil {
		t.Fatal(err)
	}
	first, err := buildSchemaCoverage(xsd, samples)
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildSchemaCoverage(xsd, samples)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := marshalCoverage(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := marshalCoverage(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatal("coverage report is not deterministic")
	}
	if first.Fixtures != 71 {
		t.Fatalf("fixtures = %d, want 71", first.Fixtures)
	}
	if got := first.Categories["roots"]; got.Total != 65 || got.Exercised != 65 {
		t.Fatalf("roots = %+v, want 65/65", got)
	}
	if first.Categories["optionalElements"].Total == 0 || first.Categories["repeatableElements"].Total == 0 {
		t.Fatal("nested cardinality particles were not indexed")
	}
	if first.Categories["choiceAlternatives"].Total == 0 {
		t.Fatal("choice alternatives were not indexed")
	}
	if len(first.FixtureTargets) == 0 {
		t.Fatal("fixture-to-target mappings were not recorded")
	}
	if len(first.MaximumCardinality) == 0 {
		t.Fatal("maximum cardinalities were not recorded")
	}
	for id := range schemaCoverageTargets(index).repeatable {
		if _, ok := first.MaximumCardinality[id]; !ok {
			t.Fatalf("repeatable target %s has no recorded maximum cardinality", id)
		}
	}
	for _, category := range []string{"roots", "requiredElements", "optionalElements", "sequenceMembers", "allMembers", "inheritance", "substitutionGroups", "recursiveEdges", "wildcards"} {
		if _, ok := first.Categories[category]; !ok {
			t.Fatalf("missing category %s", category)
		}
	}
	markdown := filepath.Join(t.TempDir(), "coverage.md")
	if err := writeCoverageMarkdown(markdown, first); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(markdown)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("Repeatable particles with zero exercised occurrences")) {
		t.Fatal("coverage Markdown omits zero-cardinality repeatable particles")
	}
	if len(first.CardinalityGaps) == 0 {
		t.Fatal("coverage did not identify repeatable particles missing required cardinalities")
	}
	if !bytes.Contains(data, []byte("Repeatable particles missing required cardinalities")) {
		t.Fatal("coverage Markdown omits cardinality acceptance gaps")
	}
}

func TestCardinalityRequirementsUseSchemaBounds(t *testing.T) {
	if got := requiredCardinalities(occurrenceBounds{min: 0, max: 5}); !containsAllInts(got, []int{0, 1, 2, 5}) {
		t.Fatalf("finite bounds requirements = %v, want 0, 1, 2, and 5", got)
	}
	if containsAllInts([]int{0, 1, 2}, requiredCardinalities(occurrenceBounds{min: 0, max: 5})) {
		t.Fatal("two occurrences satisfied a particle whose schema permits five")
	}
	if containsAllInts([]int{0, 1, 5}, requiredCardinalities(occurrenceBounds{min: 0, max: 5})) {
		t.Fatal("maximum cardinality did not require a distinct multiple case")
	}
	if got := requiredCardinalities(occurrenceBounds{min: 0, max: 10}); !containsAllInts(got, []int{0, 1, 2, 10}) {
		t.Fatalf("finite maximum above the fixture cap was truncated: got %v, want 0, 1, 2, and 10", got)
	}
	if containsAllInts([]int{0, 1, 2, 5}, requiredCardinalities(occurrenceBounds{min: 0, max: 10})) {
		t.Fatal("five occurrences satisfied a particle whose schema permits ten")
	}
	if !containsAllInts([]int{0, 1, 2, 5}, requiredCardinalities(occurrenceBounds{min: 0, max: -1})) {
		t.Fatal("unbounded particle did not require the capped multiple case")
	}
}

func TestCardinalityResidualsAreTargetLevelAndNonWaiving(t *testing.T) {
	report := schemaCoverage{
		CardinalityGaps:        []string{"target-b", "target-a"},
		ExercisedCardinalities: map[string][]int{"target-a": {0, 1}, "target-b": {1, 2}},
		RequiredCardinalities:  map[string][]int{"target-a": {0, 1, 2}, "target-b": {0, 1, 2}},
	}
	got := cardinalityResiduals(report)
	if len(got) != 2 || got[0].Target != "target-a" || got[1].Target != "target-b" {
		t.Fatalf("residuals = %#v, want sorted per-target records", got)
	}
	if got[0].Disposition != "requires-xsd-valid-fixture" || got[0].Approved || !containsAllInts(got[0].Missing, []int{2}) {
		t.Fatalf("target-a residual = %#v, want non-waiving missing-fixture disposition", got[0])
	}
	data, err := marshalCoverage(report)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("\"cardinalityResiduals\"")) {
		t.Fatal("coverage JSON omits target-level cardinality residuals")
	}
}
