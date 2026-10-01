package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

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
	if !containsAllInts([]int{0, 1, 2, 5}, requiredCardinalities(occurrenceBounds{min: 0, max: -1})) {
		t.Fatal("unbounded particle did not require the capped multiple case")
	}
}
