package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
)

func TestMatchFixtureUsesNestedChoiceParticleAndSiblingCardinality(t *testing.T) {
	const ns = "urn:test"
	doc := &schemaDoc{File: "test.xsd", TargetNamespace: ns, Elements: map[string]xsdElementDecl{}, Types: map[string]xsdComplexType{}, SimpleTypes: map[string]xsdSimpleType{}}
	doc.Elements["Root"] = xsdElementDecl{Name: "Root", Type: "RootType"}
	doc.Types["RootType"] = xsdComplexType{Name: "RootType", Sequence: &xsdSequence{Choices: []xsdChoice{{Elements: []xsdElementDecl{{Name: "Item", Type: "ItemType", MaxOccurs: "unbounded"}}}}}}
	doc.Types["ItemType"] = xsdComplexType{Name: "ItemType"}
	index := schemaIndex{ByTargetNS: map[string]*schemaDoc{ns: doc}, GlobalElements: map[string]xsdElementDecl{qualifiedKey(ns, "Root"): doc.Elements["Root"]}, GlobalTypes: map[string]xsdComplexType{qualifiedKey(ns, "RootType"): doc.Types["RootType"], qualifiedKey(ns, "ItemType"): doc.Types["ItemType"]}}
	path := filepath.Join(t.TempDir(), "fixture.xml")
	if err := os.WriteFile(path, []byte(`<Root xmlns="urn:test"><Item/><Item/></Root>`), 0o600); err != nil {
		t.Fatal(err)
	}
	matched, err := matchFixture(index, path)
	if err != nil {
		t.Fatal(err)
	}
	itemID := ns + "|test.xsd|element|RootType/choice[0]/choice[0]/Item"
	if matched[itemID] != 2 {
		t.Fatalf("nested choice item cardinality = %d, want 2", matched[itemID])
	}
	choiceID := ns + "|test.xsd|choice|RootType/choice[0]"
	if matched[choiceID] != 2 {
		t.Fatalf("choice site = %d, want 2", matched[choiceID])
	}
}

func TestInheritedAttributeAndWildcardSitesUseDeclaringType(t *testing.T) {
	const ns = "urn:test"
	doc := &schemaDoc{File: "test.xsd", TargetNamespace: ns, Types: map[string]xsdComplexType{}}
	doc.Types["Base"] = xsdComplexType{Name: "Base", Sequence: &xsdSequence{Any: &xsdAny{}}, Attributes: []xsdAttribute{{Name: "baseAttr"}}}
	doc.Types["Derived"] = xsdComplexType{Name: "Derived", ComplexContent: &xsdComplexContent{Extension: &xsdExtension{Base: "Base"}}}
	index := schemaIndex{ByTargetNS: map[string]*schemaDoc{ns: doc}, GlobalTypes: map[string]xsdComplexType{qualifiedKey(ns, "Base"): doc.Types["Base"]}}
	out := map[string]int{}
	derived := schemaFrame{doc: doc, typ: doc.Types["Derived"], typeName: "Derived", targets: out}
	recordAttributes(index, derived, xml.StartElement{Attr: []xml.Attr{{Name: xml.Name{Local: "baseAttr"}}}})
	attributeID := ns + "|test.xsd|attribute|Base/@baseAttr"
	if out[attributeID] != 1 {
		t.Fatalf("inherited attribute = %d, want 1", out[attributeID])
	}
	wildcardID := ns + "|test.xsd|any|Base/sequence"
	if got := wildcardSite(index, derived); got != wildcardID {
		t.Fatalf("wildcard site = %q, want %q", got, wildcardID)
	}
}

func TestSimpleContentInheritanceAttributesAndDepthAreRecorded(t *testing.T) {
	const ns = "urn:test"
	doc := &schemaDoc{File: "test.xsd", TargetNamespace: ns, Elements: map[string]xsdElementDecl{}, Types: map[string]xsdComplexType{}}
	doc.Elements["Root"] = xsdElementDecl{Name: "Root", Type: "A"}
	doc.Types["Base"] = xsdComplexType{Name: "Base", SimpleContent: &xsdSimpleContent{Extension: &xsdExtension{Base: "string", Attributes: []xsdAttribute{{Name: "code"}}}}}
	doc.Types["A"] = xsdComplexType{Name: "A", Sequence: &xsdSequence{Elements: []xsdElementDecl{{Name: "B", Type: "B"}}}}
	doc.Types["B"] = xsdComplexType{Name: "B", Sequence: &xsdSequence{Elements: []xsdElementDecl{{Name: "A", Type: "A"}}}}
	index := schemaIndex{ByTargetNS: map[string]*schemaDoc{ns: doc}, GlobalElements: map[string]xsdElementDecl{qualifiedKey(ns, "Root"): doc.Elements["Root"]}, GlobalTypes: map[string]xsdComplexType{qualifiedKey(ns, "A"): doc.Types["A"], qualifiedKey(ns, "B"): doc.Types["B"]}}
	derived := schemaFrame{doc: doc, typ: xsdComplexType{Name: "Derived", SimpleContent: &xsdSimpleContent{Extension: &xsdExtension{Base: "Base"}}}, typeName: "Derived", targets: map[string]int{}}
	recordAttributes(index, derived, xml.StartElement{Attr: []xml.Attr{{Name: xml.Name{Local: "code"}}}})
	if got := derived.targets[ns+"|test.xsd|attribute|Base/@code"]; got != 1 {
		t.Fatalf("simple-content inherited attribute = %d, want 1", got)
	}
	path := filepath.Join(t.TempDir(), "fixture.xml")
	if err := os.WriteFile(path, []byte(`<Root xmlns="urn:test"><B><A/></B></Root>`), 0o600); err != nil {
		t.Fatal(err)
	}
	matched, err := matchFixture(index, path)
	if err != nil {
		t.Fatal(err)
	}
	if got := matched[ns+"|test.xsd|recursive|B->"+qualifiedKey(ns, "A")]; got != 2 {
		t.Fatalf("recursive depth = %d, want 2", got)
	}
}

func TestSchemaCoverageInventoriesCyclicAndSubstitutionTargets(t *testing.T) {
	const ns = "urn:test"
	doc := &schemaDoc{File: "test.xsd", TargetNamespace: ns, Elements: map[string]xsdElementDecl{"Member": {Name: "Member", SubstitutionGroup: "Head"}}, Types: map[string]xsdComplexType{}}
	doc.Types["A"] = xsdComplexType{Name: "A", Sequence: &xsdSequence{Elements: []xsdElementDecl{{Name: "B", Type: "B"}}}}
	doc.Types["B"] = xsdComplexType{Name: "B", Sequence: &xsdSequence{Elements: []xsdElementDecl{{Name: "A", Type: "A"}}}}
	index := schemaIndex{ByFile: map[string]*schemaDoc{"/tmp/maindoc/test.xsd": doc}, ByTargetNS: map[string]*schemaDoc{ns: doc}}
	targets := schemaCoverageTargets(index)
	if len(targets.recursiveEdges) != 2 {
		t.Fatalf("recursive edges = %d, want 2", len(targets.recursiveEdges))
	}
	if len(targets.substitutions) != 1 {
		t.Fatalf("substitution targets = %d, want 1", len(targets.substitutions))
	}
}
