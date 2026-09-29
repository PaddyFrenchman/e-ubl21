// This file defines Go structs mirroring the subset of the W3C XML Schema
// language used across the OASIS UBL 2.1 XSD distribution: schema documents,
// element/attribute declarations, complex/simple types, content models
// (sequence/choice/all) and simpleContent restriction/extension. Values are
// populated directly via encoding/xml unmarshalling.
package main

import "encoding/xml"

type xsdSchema struct {
	XMLName         xml.Name         `xml:"schema"`
	TargetNamespace string           `xml:"targetNamespace,attr"`
	Elements        []xsdElementDecl `xml:"element"`
	ComplexTypes    []xsdComplexType `xml:"complexType"`
	SimpleTypes     []xsdSimpleType  `xml:"simpleType"`
	Imports         []xsdImport      `xml:"import"`
}

type xsdImport struct {
	Namespace      string `xml:"namespace,attr"`
	SchemaLocation string `xml:"schemaLocation,attr"`
}

type xsdElementDecl struct {
	Name              string          `xml:"name,attr"`
	Ref               string          `xml:"ref,attr"`
	Type              string          `xml:"type,attr"`
	MinOccurs         string          `xml:"minOccurs,attr"`
	MaxOccurs         string          `xml:"maxOccurs,attr"`
	SubstitutionGroup string          `xml:"substitutionGroup,attr"`
	ComplexType       *xsdComplexType `xml:"complexType"`
	SimpleType        *xsdSimpleType  `xml:"simpleType"`
	Sequence          *xsdSequence    `xml:"sequence"`
	Choice            *xsdChoice      `xml:"choice"`
	All               *xsdAll         `xml:"all"`
}

type xsdComplexType struct {
	ComplexContent *xsdComplexContent `xml:"complexContent"`
	Name           string             `xml:"name,attr"`
	Sequence       *xsdSequence       `xml:"sequence"`
	Choice         *xsdChoice         `xml:"choice"`
	All            *xsdAll            `xml:"all"`
	SimpleContent  *xsdSimpleContent  `xml:"simpleContent"`
	Attributes     []xsdAttribute     `xml:"attribute"`
}

type xsdComplexContent struct {
	Extension   *xsdExtension   `xml:"extension"`
	Restriction *xsdRestriction `xml:"restriction"`
}

type xsdSimpleContent struct {
	Restriction *xsdRestriction `xml:"restriction"`
	Extension   *xsdExtension   `xml:"extension"`
}

type xsdExtension struct {
	Base       string         `xml:"base,attr"`
	Attributes []xsdAttribute `xml:"attribute"`
	Sequence   *xsdSequence   `xml:"sequence"`
	Choice     *xsdChoice     `xml:"choice"`
	All        *xsdAll        `xml:"all"`
}

type xsdSequence struct {
	Sequences []xsdSequence    `xml:"sequence"`
	Choices   []xsdChoice      `xml:"choice"`
	Alls      []xsdAll         `xml:"all"`
	Elements  []xsdElementDecl `xml:"element"`
	Any       *xsdAny          `xml:"any"`
}

type xsdAny struct {
	MinOccurs string `xml:"minOccurs,attr"`
	MaxOccurs string `xml:"maxOccurs,attr"`
}

type xsdChoice struct {
	Sequences []xsdSequence    `xml:"sequence"`
	Choices   []xsdChoice      `xml:"choice"`
	Alls      []xsdAll         `xml:"all"`
	Elements  []xsdElementDecl `xml:"element"`
	Any       *xsdAny          `xml:"any"`
}

type xsdAll struct {
	Sequences []xsdSequence    `xml:"sequence"`
	Choices   []xsdChoice      `xml:"choice"`
	Alls      []xsdAll         `xml:"all"`
	Elements  []xsdElementDecl `xml:"element"`
	Any       *xsdAny          `xml:"any"`
}

type xsdSimpleType struct {
	Name        string          `xml:"name,attr"`
	Restriction *xsdRestriction `xml:"restriction"`
}

type xsdRestriction struct {
	Base         string           `xml:"base,attr"`
	Attributes   []xsdAttribute   `xml:"attribute"`
	Enumerations []xsdEnumeration `xml:"enumeration"`
}

type xsdEnumeration struct {
	Value string `xml:"value,attr"`
}

type xsdAttribute struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
	Use  string `xml:"use,attr"`
}
