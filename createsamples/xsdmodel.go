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
	Abstract       string             `xml:"abstract,attr"`
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
	MinOccurs string           `xml:"minOccurs,attr"`
	MaxOccurs string           `xml:"maxOccurs,attr"`
	Sequences []xsdSequence    `xml:"sequence"`
	Choices   []xsdChoice      `xml:"choice"`
	Alls      []xsdAll         `xml:"all"`
	Elements  []xsdElementDecl `xml:"element"`
	Any       *xsdAny          `xml:"any"`
	Particles []xsdParticle    `xml:"-"`
}

// xsdParticle retains the declaration order which encoding/xml otherwise
// loses by collecting each child element into a separate slice. Sequence
// ordering is significant: an xsd:choice may appear between two elements.
type xsdParticle struct {
	kind  string
	index int
}

func (s *xsdSequence) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	*s = xsdSequence{}
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "minOccurs":
			s.MinOccurs = attr.Value
		case "maxOccurs":
			s.MaxOccurs = attr.Value
		}
	}
	for {
		token, err := d.Token()
		if err != nil {
			return err
		}
		switch token := token.(type) {
		case xml.EndElement:
			return nil
		case xml.StartElement:
			switch token.Name.Local {
			case "element":
				var value xsdElementDecl
				if err := d.DecodeElement(&value, &token); err != nil {
					return err
				}
				s.Elements = append(s.Elements, value)
				s.Particles = append(s.Particles, xsdParticle{"element", len(s.Elements) - 1})
			case "sequence":
				var value xsdSequence
				if err := d.DecodeElement(&value, &token); err != nil {
					return err
				}
				s.Sequences = append(s.Sequences, value)
				s.Particles = append(s.Particles, xsdParticle{"sequence", len(s.Sequences) - 1})
			case "choice":
				var value xsdChoice
				if err := d.DecodeElement(&value, &token); err != nil {
					return err
				}
				s.Choices = append(s.Choices, value)
				s.Particles = append(s.Particles, xsdParticle{"choice", len(s.Choices) - 1})
			case "all":
				var value xsdAll
				if err := d.DecodeElement(&value, &token); err != nil {
					return err
				}
				s.Alls = append(s.Alls, value)
				s.Particles = append(s.Particles, xsdParticle{"all", len(s.Alls) - 1})
			case "any":
				var value xsdAny
				if err := d.DecodeElement(&value, &token); err != nil {
					return err
				}
				s.Any = &value
				s.Particles = append(s.Particles, xsdParticle{"any", 0})
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		}
	}
}

type xsdAny struct {
	MinOccurs       string `xml:"minOccurs,attr"`
	MaxOccurs       string `xml:"maxOccurs,attr"`
	Namespace       string `xml:"namespace,attr"`
	ProcessContents string `xml:"processContents,attr"`
}

type xsdChoice struct {
	MinOccurs string           `xml:"minOccurs,attr"`
	MaxOccurs string           `xml:"maxOccurs,attr"`
	Sequences []xsdSequence    `xml:"sequence"`
	Choices   []xsdChoice      `xml:"choice"`
	Alls      []xsdAll         `xml:"all"`
	Elements  []xsdElementDecl `xml:"element"`
	Any       *xsdAny          `xml:"any"`
}

type xsdAll struct {
	MinOccurs string           `xml:"minOccurs,attr"`
	MaxOccurs string           `xml:"maxOccurs,attr"`
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
	Base           string           `xml:"base,attr"`
	Attributes     []xsdAttribute   `xml:"attribute"`
	Enumerations   []xsdEnumeration `xml:"enumeration"`
	MinInclusive   *xsdFacet        `xml:"minInclusive"`
	MaxInclusive   *xsdFacet        `xml:"maxInclusive"`
	MinExclusive   *xsdFacet        `xml:"minExclusive"`
	MaxExclusive   *xsdFacet        `xml:"maxExclusive"`
	Length         *xsdFacet        `xml:"length"`
	MinLength      *xsdFacet        `xml:"minLength"`
	MaxLength      *xsdFacet        `xml:"maxLength"`
	TotalDigits    *xsdFacet        `xml:"totalDigits"`
	FractionDigits *xsdFacet        `xml:"fractionDigits"`
	Patterns       []xsdPattern     `xml:"pattern"`
	Sequence       *xsdSequence     `xml:"sequence"`
	Choice         *xsdChoice       `xml:"choice"`
	All            *xsdAll          `xml:"all"`
}

type xsdFacet struct {
	Value string `xml:"value,attr"`
}

type xsdPattern struct {
	Value string `xml:"value,attr"`
}

type xsdEnumeration struct {
	Value string `xml:"value,attr"`
}

type xsdAttribute struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
	Use  string `xml:"use,attr"`
}
