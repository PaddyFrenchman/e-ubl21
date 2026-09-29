package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestSchemaCoverageIsDeterministicAndIncludesNestedParticles(t *testing.T) {
	xsd := filepath.Join("..", "XSD")
	samples := "samples"
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
	if first.Categories["optionalElements"].Total == 0 || first.Categories["repeatableElements"].Total == 0 {
		t.Fatal("nested cardinality particles were not indexed")
	}
	if first.Categories["choiceAlternatives"].Total == 0 {
		t.Fatal("choice alternatives were not indexed")
	}
}
