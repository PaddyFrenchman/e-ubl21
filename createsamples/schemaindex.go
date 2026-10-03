// This file builds and queries the namespace-qualified schema index: a
// global lookup of every element/complexType/simpleType declared across the
// UBL 2.1 common and main-document schemas, keyed by (targetNamespace, name)
// so that same-named types declared in different namespaces (e.g. LocationType
// in both cac and cbc) never collide.
package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type schemaIndex struct {
	ByFile            map[string]*schemaDoc
	ByTargetNS        map[string]*schemaDoc
	GlobalElements    map[string]xsdElementDecl
	GlobalElementDocs map[string]*schemaDoc
	GlobalTypes       map[string]xsdComplexType
	GlobalTypeDocs    map[string]*schemaDoc
	GlobalSimpleTypes map[string]xsdSimpleType
}

// ublNamespaceByPrefix records the fixed namespace URI for every namespace
// prefix used across the OASIS UBL 2.1 schema distribution. These prefix
// conventions are consistent in every one of the 79 shipped XSD files, so a
// static table is sufficient (and much simpler than reparsing each file's
// xmlns declarations) to resolve a prefixed type/element/base reference to
// the schema it actually belongs to.
var ublNamespaceByPrefix = map[string]string{
	"cac":      "urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2",
	"cbc":      "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2",
	"ext":      "urn:oasis:names:specification:ubl:schema:xsd:CommonExtensionComponents-2",
	"udt":      "urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2",
	"qdt":      "urn:oasis:names:specification:ubl:schema:xsd:QualifiedDataTypes-2",
	"ccts-cct": "urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2",
	"sac":      "urn:oasis:names:specification:ubl:schema:xsd:SignatureAggregateComponents-2",
	"sbc":      "urn:oasis:names:specification:ubl:schema:xsd:SignatureBasicComponents-2",
	"ds":       "http://www.w3.org/2000/09/xmldsig#",
	"xades":    "http://uri.etsi.org/01903/v1.3.2#",
}

// qualifyRef resolves a possibly-prefixed XSD reference (an element ref, a
// type attribute or a restriction/extension base) to the namespace it lives
// in and its bare local name. An unprefixed reference belongs to the same
// namespace as the schema document that contains it (contextNS); this always
// holds for the UBL 2.1 distribution because every schema declares its own
// target namespace as the default (unprefixed) namespace.
func qualifyRef(raw, contextNS string) (namespace, name string) {
	if raw == "" {
		return "", ""
	}
	prefix, local := prefixPart(raw), localPart(raw)
	if prefix == "" {
		return contextNS, local
	}
	if ns, ok := ublNamespaceByPrefix[prefix]; ok {
		return ns, local
	}
	return "", local
}

func qualifiedKey(namespace, name string) string {
	return namespace + "#" + name
}

type schemaDoc struct {
	File            string
	TargetNamespace string
	Elements        map[string]xsdElementDecl
	Types           map[string]xsdComplexType
	SimpleTypes     map[string]xsdSimpleType
}

func buildSchemaIndex(xsdDir string) (schemaIndex, error) {
	index := schemaIndex{
		ByFile:            map[string]*schemaDoc{},
		ByTargetNS:        map[string]*schemaDoc{},
		GlobalElements:    map[string]xsdElementDecl{},
		GlobalElementDocs: map[string]*schemaDoc{},
		GlobalTypes:       map[string]xsdComplexType{},
		GlobalTypeDocs:    map[string]*schemaDoc{},
		GlobalSimpleTypes: map[string]xsdSimpleType{},
	}

	commonFiles, err := listXSDs(filepath.Join(xsdDir, "common"))
	if err != nil {
		return schemaIndex{}, err
	}
	mainFiles, err := listXSDs(filepath.Join(xsdDir, "maindoc"))
	if err != nil {
		return schemaIndex{}, err
	}
	for _, file := range append(commonFiles, mainFiles...) {
		doc, err := loadSchemaDoc(file)
		if err != nil {
			return schemaIndex{}, fmt.Errorf("load schema %s: %w", file, err)
		}
		index.ByFile[file] = doc
		if doc.TargetNamespace != "" {
			index.ByTargetNS[doc.TargetNamespace] = doc
		}
		for name, decl := range doc.Elements {
			key := qualifiedKey(doc.TargetNamespace, name)
			index.GlobalElements[key] = decl
			index.GlobalElementDocs[key] = doc
		}
		for name, typ := range doc.Types {
			key := qualifiedKey(doc.TargetNamespace, name)
			index.GlobalTypes[key] = typ
			index.GlobalTypeDocs[key] = doc
		}
		for name, typ := range doc.SimpleTypes {
			index.GlobalSimpleTypes[qualifiedKey(doc.TargetNamespace, name)] = typ
		}
	}
	return index, nil
}

func loadSchemaDoc(path string) (*schemaDoc, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc xsdSchema
	if err := xml.Unmarshal(content, &doc); err != nil {
		return nil, err
	}
	result := &schemaDoc{File: filepath.Base(path), TargetNamespace: doc.TargetNamespace, Elements: map[string]xsdElementDecl{}, Types: map[string]xsdComplexType{}, SimpleTypes: map[string]xsdSimpleType{}}
	for _, element := range doc.Elements {
		if element.Name != "" {
			result.Elements[element.Name] = element
		}
		if element.Ref != "" {
			result.Elements[localPart(element.Ref)] = element
		}
	}
	for _, typ := range doc.ComplexTypes {
		if typ.Name != "" {
			result.Types[typ.Name] = typ
		}
	}
	for _, typ := range doc.SimpleTypes {
		if typ.Name != "" {
			result.SimpleTypes[typ.Name] = typ
			result.Types[typ.Name] = xsdComplexType{Name: typ.Name}
		}
	}
	return result, nil
}

func rootNamespaceForSchema(xsdDir, schemaFile string) (string, error) {
	path := filepath.Join(xsdDir, "maindoc", schemaFile)
	if filepath.Dir(schemaFile) != "." {
		path = filepath.Join(xsdDir, schemaFile)
	} else if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		path = filepath.Join(xsdDir, "common", schemaFile)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var doc xsdSchema
	if err := xml.Unmarshal(content, &doc); err != nil {
		return "", err
	}
	if doc.TargetNamespace != "" {
		return doc.TargetNamespace, nil
	}
	return fmt.Sprintf("urn:oasis:names:specification:ubl:schema:xsd:%s-2", trimUBLDocumentType(schemaFile)), nil
}
