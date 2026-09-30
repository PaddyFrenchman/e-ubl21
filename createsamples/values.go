// This file generates deterministic sample scalar values for XSD simple
// types, as described in section 11 (Value Generation) of
// .github/create-samples.md: legal enumerated values take priority, then a
// UBL naming-convention heuristic keyed on the element/type name's last
// CamelCase word (its CCTS "representation term", e.g. ...Indicator,
// ...Amount, ...Percent) picks a plausible literal.
//
// Matching whole trailing words - rather than raw substrings - matters: UBL
// business names routinely contain unrelated category words earlier in the
// compound (e.g. RefrigeratedIndicator contains "rate", CandidateReduction-
// ConstraintIndicator and ItemUpdateRequestIndicator both contain "date",
// UnknownPriceIndicator contains "price", TierRatePercent contains "rate").
// Only the true trailing representation term should govern the generated
// value, so names are tokenized into CamelCase words first and only the
// last word (or, for two-word terms like BinaryObject/DateTime/CurrencyCode,
// the last two) is looked up.
package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// camelWordPattern splits a PascalCase or camelCase identifier (an XML
// attribute name may start with a lowercase word, e.g. "currencyID") into
// its component words, keeping a run of uppercase letters that forms an
// acronym (ID, URI, ...) together as a single trailing word.
var camelWordPattern = regexp.MustCompile(`[A-Z]+[a-z0-9]*|[a-z0-9]+`)

func camelWords(s string) []string {
	return camelWordPattern.FindAllString(s, -1)
}

// twoWordValueCategories maps two-word CamelCase suffixes to a value
// category, checked before the single trailing word so that these
// multi-word representation terms aren't shadowed by their own last word
// (e.g. "BinaryObject" by plain "Object", "CurrencyCode"/"CurrencyID" by
// plain "Code"/"ID").
var twoWordValueCategories = map[string]string{
	"binaryobject": "binary",
	"datetime":     "datetime",
	"currencycode": "currency",
	"currencyid":   "currency",
}

// oneWordValueCategories maps a single trailing CamelCase word to a value
// category. The keys are drawn from the CCTS/UBL "representation term"
// vocabulary (Amount, Percent, Rate, Quantity, Measure, Numeric, Indicator,
// Date, Time, Code, Identifier/ID, ...); anything that doesn't match is left
// as free-form text, which is always legal since unmatched types are
// invariably unrestricted xsd:string (UBL TextType) in this schema set.
var oneWordValueCategories = map[string]string{
	"amount":     "amount",
	"percent":    "percent",
	"rate":       "rate",
	"quantity":   "quantity",
	"measure":    "measure",
	"numeric":    "numeric",
	"indicator":  "boolean",
	"boolean":    "boolean",
	"date":       "date",
	"time":       "time",
	"duration":   "duration",
	"code":       "code",
	"id":         "identifier",
	"identifier": "identifier",
	"currency":   "currency",
	"status":     "status",
	"graphic":    "binary",
	"picture":    "binary",
	"sound":      "binary",
	"video":      "binary",
}

// valueCategoryForName returns the value category implied by name's
// trailing CamelCase word(s), or "" if none of the known representation
// terms match.
func valueCategoryForName(name string) string {
	words := camelWords(name)
	if len(words) == 0 {
		return ""
	}
	if len(words) >= 2 {
		two := strings.ToLower(words[len(words)-2] + words[len(words)-1])
		if cat, ok := twoWordValueCategories[two]; ok {
			return cat
		}
	}
	one := strings.ToLower(words[len(words)-1])
	return oneWordValueCategories[one]
}

func sampleValueForType(name, typeName string, index int) string {
	category := valueCategoryForName(name)
	if category == "" {
		// Fall back to the type name (e.g. for an attribute whose own name
		// doesn't reveal its category), stripping the conventional "Type"
		// suffix so e.g. "PartecipationPercentType" resolves to "Percent".
		category = valueCategoryForName(strings.TrimSuffix(typeName, "Type"))
	}

	switch category {
	case "date":
		return "2026-01-01"
	case "time":
		return "12:00:00"
	case "datetime":
		return "2026-01-01T12:00:00Z"
	case "duration":
		return "P0D"
	case "rate":
		if index > 0 {
			return fmt.Sprintf("%.2f", float64(index)/10.0+0.95)
		}
		return "1.25"
	case "amount":
		return "100.00"
	case "percent":
		return "10.00"
	case "numeric", "quantity", "measure":
		return "1"
	case "boolean":
		return "true"
	case "currency":
		return "USD"
	case "identifier":
		return "ID-001"
	case "code":
		return "CODE-001"
	case "binary":
		return "U2FtcGxlRGF0YQ=="
	case "status":
		return "Active"
	default:
		base := "Value of " + strings.TrimSpace(name)
		if index > 0 {
			return fmt.Sprintf("%s %d", base, index)
		}
		return base
	}
}

func enumerationValuesForType(index schemaIndex, typeName, ns string) []string {
	if typeName == "" {
		return nil
	}
	tNS, tName := qualifyRef(typeName, ns)
	key := qualifiedKey(tNS, tName)
	if typ, ok := index.GlobalSimpleTypes[key]; ok && typ.Restriction != nil {
		values := make([]string, 0, len(typ.Restriction.Enumerations))
		for _, enum := range typ.Restriction.Enumerations {
			if enum.Value != "" {
				values = append(values, enum.Value)
			}
		}
		if len(values) > 0 {
			return values
		}
	}
	if typ, ok := index.GlobalTypes[key]; ok && typ.SimpleContent != nil && typ.SimpleContent.Restriction != nil {
		values := make([]string, 0, len(typ.SimpleContent.Restriction.Enumerations))
		for _, enum := range typ.SimpleContent.Restriction.Enumerations {
			if enum.Value != "" {
				values = append(values, enum.Value)
			}
		}
		if len(values) > 0 {
			return values
		}
	}
	return nil
}

func sampleScalarForType(index schemaIndex, name, typeName, ns string, idx int) string {
	if values := enumerationValuesForType(index, typeName, ns); len(values) > 0 {
		return values[0]
	}
	if boundaries := facetBoundaryValuesForType(index, typeName, ns); len(boundaries) > 0 && idx > 0 {
		return boundaries[(idx-1)%len(boundaries)]
	}
	// Boundary mode must leave unrestricted values unchanged: passing idx
	// through would alter ordinary text/rate samples without exercising a
	// schema facet.
	return sampleValueForType(name, typeName, 0)
}

// facetBoundaryValuesForType returns legal representative endpoints for the
// common numeric and length facets used by schema-derived simple types.
func facetBoundaryValuesForType(index schemaIndex, typeName, ns string) []string {
	tNS, tName := qualifyRef(typeName, ns)
	typ, ok := index.GlobalSimpleTypes[qualifiedKey(tNS, tName)]
	if !ok || typ.Restriction == nil {
		return nil
	}
	r := typ.Restriction
	var out []string
	for _, f := range []*xsdFacet{r.MinInclusive, r.MaxInclusive} {
		if f != nil && f.Value != "" {
			out = append(out, f.Value)
		}
	}
	for _, f := range []*xsdFacet{r.MinExclusive, r.MaxExclusive} {
		if f == nil || f.Value == "" {
			continue
		}
		if n, err := strconv.ParseInt(f.Value, 10, 64); err == nil {
			if f == r.MinExclusive {
				out = append(out, strconv.FormatInt(n+1, 10))
			} else {
				out = append(out, strconv.FormatInt(n-1, 10))
			}
		}
	}
	if r.Length != nil {
		out = append(out, strings.Repeat("X", mustPositiveInt(r.Length.Value, 1)))
	}
	if r.MinLength != nil {
		out = append(out, strings.Repeat("X", mustPositiveInt(r.MinLength.Value, 1)))
	}
	if r.MaxLength != nil {
		out = append(out, strings.Repeat("X", mustPositiveInt(r.MaxLength.Value, 1)))
	}
	// totalDigits/fractionDigits constrain the lexical decimal representation.
	// Use all nines so the value is at the permitted precision boundary without
	// assuming a numeric range that may be imposed by another facet.
	if r.TotalDigits != nil {
		digits := mustPositiveInt(r.TotalDigits.Value, 1)
		fraction := 0
		if r.FractionDigits != nil {
			fraction = mustPositiveInt(r.FractionDigits.Value, 1)
			if fraction > digits {
				fraction = digits
			}
		}
		whole := digits - fraction
		if whole < 1 {
			whole = 1
			fraction = digits - 1
		}
		value := strings.Repeat("9", whole)
		if fraction > 0 {
			value += "." + strings.Repeat("9", fraction)
		}
		out = append(out, value)
	}
	return out
}

func mustPositiveInt(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

func isPrimitiveType(t string) bool {
	set := map[string]bool{
		"xsd:string": true, "xsd:boolean": true, "xsd:int": true, "xsd:integer": true, "xsd:decimal": true, "xsd:double": true,
		"xsd:date": true, "xsd:dateTime": true, "xsd:time": true, "xsd:token": true, "xsd:normalizedString": true,
		"xsd:anyURI": true, "xsd:language": true, "xsd:base64Binary": true, "xsd:duration": true,
	}
	return set[t]
}
