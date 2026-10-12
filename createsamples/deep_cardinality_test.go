package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeepCardinalityDoesNotMultiplyNestedWitnesses(t *testing.T) {
	for _, tc := range []struct {
		name    string
		model   string
		parents int
		leaves  int
	}{
		{"element", `<xsd:sequence><xsd:element name="Parent" type="ParentType" maxOccurs="5"/></xsd:sequence>`, 5, 6},
		{"sequence", `<xsd:sequence maxOccurs="5"><xsd:element name="Parent" type="ParentType" maxOccurs="2"/></xsd:sequence>`, 6, 7},
		{"optional-sequence", `<xsd:sequence maxOccurs="5"><xsd:element name="Parent" type="ParentType" minOccurs="0" maxOccurs="2"/></xsd:sequence>`, 6, 7},
		{"choice", `<xsd:choice maxOccurs="5"><xsd:element name="Parent" type="ParentType" maxOccurs="2"/><xsd:element name="Other" type="xsd:string"/></xsd:choice>`, 6, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "maindoc"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(dir, "common"), 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "maindoc", "root.xsd")
			schema := fmt.Sprintf(`<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns="urn:test" targetNamespace="urn:test" elementFormDefault="qualified">
  <xsd:element name="Root" type="RootType"/>
  <xsd:complexType name="RootType">%s</xsd:complexType>
  <xsd:complexType name="ParentType"><xsd:sequence><xsd:element name="Leaf" type="LeafType" maxOccurs="2"/></xsd:sequence></xsd:complexType>
  <xsd:complexType name="LeafType"><xsd:sequence><xsd:element name="Required" type="xsd:string"/><xsd:element name="Optional" type="xsd:string" minOccurs="0"/></xsd:sequence><xsd:attribute name="Id" type="xsd:ID" use="required"/></xsd:complexType>
</xsd:schema>`, tc.model)
			if err := os.WriteFile(path, []byte(schema), 0o600); err != nil {
				t.Fatal(err)
			}
			index, err := buildSchemaIndex(dir)
			if err != nil {
				t.Fatal(err)
			}
			content, err := generateDeepCardinalityStructuralSample(index, "Root", dir, "root.xsd", nil, 5)
			if err != nil {
				t.Fatal(err)
			}
			for tag, want := range map[string]int{"<Parent>": tc.parents, "<Leaf ": tc.leaves, "<Required>": tc.leaves, "<Optional>": 1} {
				if got := strings.Count(content, tag); got != want {
					t.Errorf("%s count = %d, want %d", tag, got, want)
				}
			}
			// This also verifies finite maxima and document-wide xsd:ID uniqueness.
			if err := validateStructuralCandidate(content, path); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDeepCardinalityMainDocumentsAreBoundedAndValid(t *testing.T) {
	xsdDir := filepath.Join("..", "XSD")
	index, err := buildSchemaIndex(xsdDir)
	if err != nil {
		t.Fatal(err)
	}
	report, err := discoverInventory(xsdDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, schema := range report.MainFiles {
		for _, count := range []int{2, 5} {
			t.Run(fmt.Sprintf("%s/%d", schema.RootElement, count), func(t *testing.T) {
				content, err := generateDeepCardinalityStructuralSample(index, schema.RootElement, xsdDir, schema.File, nil, count)
				if err != nil {
					t.Fatal(err)
				}
				// AwardedNotification's five-occurrence witness previously exhausted
				// memory while cloning nested subtrees, before it could be rendered.
				if len(content) > 2*1024*1024 {
					t.Fatalf("deep witness grew to %d bytes, want at most 2 MiB", len(content))
				}
				if err := validateStructuralCandidate(content, filepath.Join(xsdDir, "maindoc", schema.File)); err != nil {
					t.Fatal(err)
				}
				if schema.RootElement == "AwardedNotification" {
					t.Logf("deep-%d witness: %d bytes", count, len(content))
					again, err := generateDeepCardinalityStructuralSample(index, schema.RootElement, xsdDir, schema.File, nil, count)
					if err != nil || content != again {
						t.Fatalf("generation was not deterministic: %v", err)
					}
				}
			})
		}
	}
}
