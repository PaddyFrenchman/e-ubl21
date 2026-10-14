package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSchemaTypeDocumentFindsIncludedSchemaType(t *testing.T) {
	const ns = "urn:test"
	primary := &schemaDoc{TargetNamespace: ns, Types: map[string]xsdComplexType{}}
	included := &schemaDoc{TargetNamespace: ns, Types: map[string]xsdComplexType{"Included": {Name: "Included"}}}
	index := schemaIndex{
		ByTargetNS: map[string]*schemaDoc{ns: primary},
		ByFile:     map[string]*schemaDoc{"primary.xsd": primary, "included.xsd": included},
	}
	doc, typ, ok := schemaTypeDocument(index, ns, "Included")
	if !ok || doc != included || typ.Name != "Included" {
		t.Fatalf("included type resolution = (%#v, %#v, %v)", doc, typ, ok)
	}
}

func TestElementDeclarationFindsIncludedSchemaElement(t *testing.T) {
	const ns = "urn:test"
	primary := &schemaDoc{TargetNamespace: ns, Elements: map[string]xsdElementDecl{}}
	included := &schemaDoc{TargetNamespace: ns, Elements: map[string]xsdElementDecl{"Included": {Name: "Included", Type: "IncludedType"}}}
	index := schemaIndex{
		ByTargetNS:        map[string]*schemaDoc{ns: primary},
		GlobalElements:    map[string]xsdElementDecl{qualifiedKey(ns, "Included"): included.Elements["Included"]},
		GlobalElementDocs: map[string]*schemaDoc{qualifiedKey(ns, "Included"): included},
	}
	doc, declaration, gotNS, name := elementDeclaration(index, primary, xsdElementDecl{Ref: "Included"})
	if doc != included || declaration.Name != "Included" || gotNS != ns || name != "Included" {
		t.Fatalf("included element resolution = (%#v, %#v, %q, %q)", doc, declaration, gotNS, name)
	}
}

func TestMatchFixtureRecordsExtensionContentWildcardFromIncludedSchema(t *testing.T) {
	index, err := buildSchemaIndex(filepath.Join("..", "XSD"))
	if err != nil {
		t.Fatal(err)
	}
	matched, err := matchFixture(index, filepath.Join("samples-structural", "Invoice", "Invoice-optional-rich.xml"))
	if err != nil {
		t.Fatal(err)
	}
	wildcard := "urn:oasis:names:specification:ubl:schema:xsd:CommonExtensionComponents-2|UBL-ExtensionContentDataType-2.1.xsd|any|ExtensionContentType/sequence"
	if matched[wildcard] != 1 {
		t.Fatalf("extension wildcard matches = %d, want 1", matched[wildcard])
	}
}

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

func TestMatchFixtureCreditsGlobalElementReferencedByParticle(t *testing.T) {
	const ns = "urn:test"
	doc := &schemaDoc{File: "test.xsd", TargetNamespace: ns, Elements: map[string]xsdElementDecl{}, Types: map[string]xsdComplexType{}, SimpleTypes: map[string]xsdSimpleType{}}
	doc.Elements["Root"] = xsdElementDecl{Name: "Root", Type: "RootType"}
	doc.Elements["Child"] = xsdElementDecl{Name: "Child", Type: "ChildType"}
	doc.Types["RootType"] = xsdComplexType{Name: "RootType", Sequence: &xsdSequence{Elements: []xsdElementDecl{{Ref: "Child"}}}}
	doc.Types["ChildType"] = xsdComplexType{Name: "ChildType"}
	index := schemaIndex{
		ByTargetNS:        map[string]*schemaDoc{ns: doc},
		GlobalElements:    map[string]xsdElementDecl{qualifiedKey(ns, "Root"): doc.Elements["Root"], qualifiedKey(ns, "Child"): doc.Elements["Child"]},
		GlobalElementDocs: map[string]*schemaDoc{qualifiedKey(ns, "Root"): doc, qualifiedKey(ns, "Child"): doc},
		GlobalTypes:       map[string]xsdComplexType{qualifiedKey(ns, "RootType"): doc.Types["RootType"], qualifiedKey(ns, "ChildType"): doc.Types["ChildType"]},
		GlobalTypeDocs:    map[string]*schemaDoc{qualifiedKey(ns, "RootType"): doc, qualifiedKey(ns, "ChildType"): doc},
	}
	path := filepath.Join(t.TempDir(), "fixture.xml")
	if err := os.WriteFile(path, []byte(`<Root xmlns="urn:test"><Child/></Root>`), 0o600); err != nil {
		t.Fatal(err)
	}
	matched, err := matchFixture(index, path)
	if err != nil {
		t.Fatal(err)
	}
	particleID := ns + "|test.xsd|element|RootType/sequence[0]/Child"
	rootID := ns + "|test.xsd|element|global/Root"
	rootElementID := ns + "|test.xsd|element|global/Root/Root"
	globalID := ns + "|test.xsd|element|global/Child/Child"
	if matched[rootID] != 1 || matched[rootElementID] != 1 {
		t.Fatalf("root targets = (%d, %d), want (1, 1)", matched[rootID], matched[rootElementID])
	}
	if matched[particleID] != 1 {
		t.Fatalf("particle target = %d, want 1", matched[particleID])
	}
	if matched[globalID] != 1 {
		t.Fatalf("referenced global target = %d, want 1", matched[globalID])
	}
}

func TestMatchStructuralFixtureCreditsReferencedUBLGlobalElement(t *testing.T) {
	index, err := buildSchemaIndex(filepath.Join("..", "XSD"))
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join("samples-structural", "CatalogueRequest", "CatalogueRequest-optional-rich.xml")
	matched, err := matchFixture(index, fixture)
	if err != nil {
		t.Fatal(err)
	}
	globalID := "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2|UBL-CommonBasicComponents-2.1.xsd|element|global/BirthDate/BirthDate"
	if matched[globalID] == 0 {
		t.Fatalf("referenced global target %s was not credited", globalID)
	}
	amountTypeID := "urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|AmountType"
	if matched[amountTypeID] == 0 {
		t.Fatalf("instantiated complex type %s was not credited", amountTypeID)
	}
	cctsTypeID := "urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|complexType|AmountType"
	if matched[cctsTypeID] == 0 {
		t.Fatalf("inherited complex type %s was not credited", cctsTypeID)
	}
	udtInheritanceID := "urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|AmountType->urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2#AmountType"
	if matched[udtInheritanceID] == 0 {
		t.Fatalf("inherited edge %s was not credited", udtInheritanceID)
	}
	cctsInheritanceID := "urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|inheritance|AmountType->#decimal"
	if matched[cctsInheritanceID] == 0 {
		t.Fatalf("base edge %s was not credited", cctsInheritanceID)
	}
}

func TestCardinalityObservationsRetainEveryParentInstance(t *testing.T) {
	const ns = "urn:test"
	doc := &schemaDoc{File: "test.xsd", TargetNamespace: ns, Elements: map[string]xsdElementDecl{"Root": {Name: "Root", Type: "RootType"}}, Types: map[string]xsdComplexType{
		"RootType":   {Name: "RootType", Sequence: &xsdSequence{Elements: []xsdElementDecl{{Name: "Parent", Type: "ParentType", MaxOccurs: "unbounded"}}}},
		"ParentType": {Name: "ParentType", Sequence: &xsdSequence{Elements: []xsdElementDecl{{Name: "Item", MaxOccurs: "5"}}}},
	}}
	index := schemaIndex{ByTargetNS: map[string]*schemaDoc{ns: doc}, GlobalElements: map[string]xsdElementDecl{qualifiedKey(ns, "Root"): doc.Elements["Root"]}, GlobalTypes: map[string]xsdComplexType{qualifiedKey(ns, "RootType"): doc.Types["RootType"], qualifiedKey(ns, "ParentType"): doc.Types["ParentType"]}}
	path := filepath.Join(t.TempDir(), "fixture.xml")
	if err := os.WriteFile(path, []byte(`<Root xmlns="urn:test"><Parent/><Parent><Item/></Parent><Parent><Item/><Item/><Item/><Item/><Item/></Parent></Root>`), 0o600); err != nil {
		t.Fatal(err)
	}
	match, err := matchFixtureDetailed(index, path)
	if err != nil {
		t.Fatal(err)
	}
	id := ns + "|test.xsd|element|ParentType/sequence[0]/Item"
	if got := match.cardinalities[id]; fmt.Sprint(got) != "[1 5]" {
		t.Fatalf("nonzero parent counts = %v, want [1 5]", got)
	}
	targets := newCoverageTargets()
	targets.repeatable[id] = struct{}{}
	result := schemaCoverage{ExercisedCardinalities: map[string][]int{id: nil}}
	recordCardinalityObservations(&result, targets, map[string]fixtureMatch{"fixture.xml": match})
	if got := result.ExercisedCardinalities[id]; fmt.Sprint(got) != "[0 1 5]" {
		t.Fatalf("parent-instance cardinalities = %v, want [0 1 5]", got)
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
	if got := wildcardSite(index, derived, xml.Name{Space: "urn:other", Local: "Unknown"}); got != wildcardID {
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

func TestMatchFixtureResolvesSubstitutionMembersAndDeclaringTypeIDs(t *testing.T) {
	const rootNS = "urn:test-root"
	const headNS = "urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
	const memberNS = "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
	root := &schemaDoc{File: "root.xsd", TargetNamespace: rootNS, Elements: map[string]xsdElementDecl{"Root": {Name: "Root", Type: "RootType"}}, Types: map[string]xsdComplexType{"RootType": {Name: "RootType", Sequence: &xsdSequence{Elements: []xsdElementDecl{{Ref: "cac:Head"}}}}}}
	head := &schemaDoc{File: "head.xsd", TargetNamespace: headNS, Elements: map[string]xsdElementDecl{"Head": {Name: "Head"}}, Types: map[string]xsdComplexType{}}
	member := &schemaDoc{File: "member.xsd", TargetNamespace: memberNS, Elements: map[string]xsdElementDecl{"Member": {Name: "Member", Type: "MemberType", SubstitutionGroup: "cac:Head"}}, Types: map[string]xsdComplexType{"MemberType": {Name: "MemberType"}}}
	index := schemaIndex{ByTargetNS: map[string]*schemaDoc{rootNS: root, headNS: head, memberNS: member}, GlobalElements: map[string]xsdElementDecl{qualifiedKey(rootNS, "Root"): root.Elements["Root"]}, GlobalTypes: map[string]xsdComplexType{qualifiedKey(rootNS, "RootType"): root.Types["RootType"], qualifiedKey(memberNS, "MemberType"): member.Types["MemberType"]}}
	path := filepath.Join(t.TempDir(), "fixture.xml")
	if err := os.WriteFile(path, []byte(`<Root xmlns="urn:test-root" xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"><cbc:Member/></Root>`), 0o600); err != nil {
		t.Fatal(err)
	}
	matched, err := matchFixture(index, path)
	if err != nil {
		t.Fatal(err)
	}
	if got := matched[memberNS+"|member.xsd|substitution|Member->"+headNS+"#Head"]; got != 1 {
		t.Fatalf("substitution match = %d, want 1", got)
	}
	if got := matched[memberNS+"|member.xsd|complexType|MemberType"]; got != 1 {
		t.Fatalf("cross-schema type match = %d, want 1", got)
	}
}

func TestCompositorCardinalityIsEffectiveForContainedElements(t *testing.T) {
	const ns = "urn:test"
	doc := &schemaDoc{File: "test.xsd", TargetNamespace: ns, Types: map[string]xsdComplexType{
		"Optional": {Name: "Optional", Choice: &xsdChoice{MinOccurs: "0", Elements: []xsdElementDecl{{Name: "A"}}}},
		"Repeated": {Name: "Repeated", Sequence: &xsdSequence{MaxOccurs: "unbounded", Elements: []xsdElementDecl{{Name: "Line"}}}},
	}}
	index := schemaIndex{ByFile: map[string]*schemaDoc{"test.xsd": doc}, ByTargetNS: map[string]*schemaDoc{ns: doc}}
	targets := schemaCoverageTargets(index)
	optionalID := ns + "|test.xsd|element|Optional/choice[0]/A"
	if _, ok := targets.required[optionalID]; ok {
		t.Fatal("element in optional choice was marked required")
	}
	if _, ok := targets.optional[optionalID]; !ok {
		t.Fatal("element in optional choice was not marked optional")
	}
	repeatedID := ns + "|test.xsd|element|Repeated/sequence[0]/Line"
	if _, ok := targets.repeatable[repeatedID]; !ok {
		t.Fatal("element in repeated sequence was not marked repeatable")
	}
}

func TestChoiceAlternativesAreNotIndividuallyRequired(t *testing.T) {
	const ns = "urn:test"
	doc := &schemaDoc{File: "test.xsd", TargetNamespace: ns, Types: map[string]xsdComplexType{
		"T": {Name: "T", Choice: &xsdChoice{Elements: []xsdElementDecl{{Name: "A"}, {Name: "B"}}}},
	}}
	targets := schemaCoverageTargets(schemaIndex{ByFile: map[string]*schemaDoc{"test.xsd": doc}, ByTargetNS: map[string]*schemaDoc{ns: doc}})
	for i, name := range []string{"A", "B"} {
		id := fmt.Sprintf("%s|test.xsd|element|T/choice[%d]/%s", ns, i, name)
		if _, ok := targets.required[id]; ok {
			t.Fatalf("choice alternative %s was marked required", name)
		}
		if _, ok := targets.optional[id]; !ok {
			t.Fatalf("choice alternative %s was not marked optional", name)
		}
	}
}

func TestChoiceSiteIsRecordedForSequenceAlternative(t *testing.T) {
	const ns = "urn:test"
	doc := &schemaDoc{File: "test.xsd", TargetNamespace: ns, Elements: map[string]xsdElementDecl{"Root": {Name: "Root", Type: "RootType"}}, Types: map[string]xsdComplexType{"RootType": {Name: "RootType", Sequence: &xsdSequence{Choices: []xsdChoice{{Sequences: []xsdSequence{{Elements: []xsdElementDecl{{Name: "Item"}}}}}}}}}}
	index := schemaIndex{ByTargetNS: map[string]*schemaDoc{ns: doc}, GlobalElements: map[string]xsdElementDecl{qualifiedKey(ns, "Root"): doc.Elements["Root"]}, GlobalTypes: map[string]xsdComplexType{qualifiedKey(ns, "RootType"): doc.Types["RootType"]}}
	path := filepath.Join(t.TempDir(), "fixture.xml")
	if err := os.WriteFile(path, []byte(`<Root xmlns="urn:test"><Item/></Root>`), 0o600); err != nil {
		t.Fatal(err)
	}
	matched, err := matchFixture(index, path)
	if err != nil {
		t.Fatal(err)
	}
	if got := matched[ns+"|test.xsd|choice|RootType/choice[0]"]; got != 1 {
		t.Fatalf("choice site = %d, want 1", got)
	}
}

func TestWildcardUsesConsumedSequencePrefixToSelectChoiceBranch(t *testing.T) {
	const ns = "urn:test"
	doc := &schemaDoc{File: "test.xsd", TargetNamespace: ns, Elements: map[string]xsdElementDecl{"Root": {Name: "Root", Type: "T"}}, Types: map[string]xsdComplexType{"T": {Name: "T", Choice: &xsdChoice{Sequences: []xsdSequence{{Elements: []xsdElementDecl{{Name: "KeyID"}}, Any: &xsdAny{Namespace: "##other"}}, {Elements: []xsdElementDecl{{Name: "KeyPacket"}}, Any: &xsdAny{Namespace: "##other"}}}}}}}
	index := schemaIndex{ByTargetNS: map[string]*schemaDoc{ns: doc}, GlobalElements: map[string]xsdElementDecl{qualifiedKey(ns, "Root"): doc.Elements["Root"]}, GlobalTypes: map[string]xsdComplexType{qualifiedKey(ns, "T"): doc.Types["T"]}}
	path := filepath.Join(t.TempDir(), "fixture.xml")
	if err := os.WriteFile(path, []byte(`<Root xmlns="urn:test"><KeyID/><x xmlns="urn:other"/></Root>`), 0o600); err != nil {
		t.Fatal(err)
	}
	matched, err := matchFixture(index, path)
	if err != nil {
		t.Fatal(err)
	}
	if got := matched[ns+"|test.xsd|any|T/sequence[0]/sequence"]; got != 1 {
		t.Fatalf("first wildcard = %d, want 1", got)
	}
	if got := matched[ns+"|test.xsd|any|T/sequence[1]/sequence"]; got != 0 {
		t.Fatalf("second wildcard = %d, want 0", got)
	}
}
