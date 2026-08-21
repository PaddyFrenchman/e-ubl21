// This file implements the UBL 2.1 sample document generator: given a root
// element declaration, it recursively expands the schema's content model
// into an in-memory xmlNode tree, which render.go then serializes to XML.
//
// Generation policy: every declared element - required or optional - is
// covered somewhere in the generated document. Because the UBL common
// aggregate components form a densely cross-referenced graph rather than a
// tree (the same complex type, e.g. PartyType or LocationType, is reachable
// through dozens of different optional paths), naively expanding every
// optional element every time it is reached would revisit shared types
// exponentially often. To keep generation fast and the output bounded, each
// distinct complex type is given its full (all-elements) expansion only the
// first time it is reached in a document; every later reuse of that same
// type falls back to a required-elements-only expansion (see genState and
// its full/visitedFull fields). This still guarantees every element and type
// in the schema appears fully expanded at least once, while bounding total
// work to roughly the number of distinct types in the schema.
package main

import (
	"fmt"
	"strconv"
)

type xmlNode struct {
	Prefix     string
	Name       string
	Value      string
	Attributes map[string]string
	Children   []xmlNode
}

// genState carries the traversal state threaded through expandElement and
// the flatten* helpers.
//
//   - full reports whether the current subtree should still expand optional
//     elements (true) or has been demoted to required-elements-only (false).
//   - ancestors is the set of complex types on the current path, used to cut
//     off genuine required structural cycles while in required-only mode; it
//     is cloned (via withAncestor) whenever it changes, so sibling branches
//     never see each other's ancestry.
//   - visitedFull is shared (never cloned) across the entire document being
//     generated: it records every complex type that has already received a
//     full expansion, so any later occurrence of that same type - anywhere
//     in the document - is demoted to required-only instead of being
//     expanded again.
type genState struct {
	full        bool
	ancestors   map[string]bool
	visitedFull map[string]bool
}

func newGenState() genState {
	return genState{full: true, ancestors: map[string]bool{}, visitedFull: map[string]bool{}}
}

func (s genState) withAncestor(key string) genState {
	s.ancestors = withAncestor(s.ancestors, key)
	return s
}

// demoted returns a copy of s with full-element expansion switched off for
// the remainder of the current subtree; ancestors starts tracking fresh from
// this point since required-only cycle protection was not needed while in
// full mode.
func (s genState) demoted() genState {
	s.full = false
	s.ancestors = map[string]bool{}
	return s
}

func generateCompleteSample(index schemaIndex, rootName, xsdDir, schemaFile string) (string, error) {
	rootNS, err := rootNamespaceForSchema(xsdDir, schemaFile)
	if err != nil {
		return "", err
	}
	rootDecl, ok := index.GlobalElements[qualifiedKey(rootNS, rootName)]
	if !ok {
		return "", fmt.Errorf("root element %q not found in schema index", rootName)
	}
	rootNode, supported := expandElement(index, rootDecl, rootName, "", rootNS, newGenState(), 0)
	if !supported {
		return "", fmt.Errorf("root element %q requires an unsupported XSD construct (e.g. xsd:any) that cannot be generated", rootName)
	}
	if rootNode.Attributes == nil {
		rootNode.Attributes = map[string]string{}
	}
	rootNode.Attributes["xmlns"] = rootNS
	rootNode.Attributes["xmlns:cbc"] = "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
	rootNode.Attributes["xmlns:cac"] = "urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
	rootNode.Attributes["xmlns:ext"] = "urn:oasis:names:specification:ubl:schema:xsd:CommonExtensionComponents-2"
	return "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" + renderXMLNode(rootNode, 0) + "\n", nil
}

// maxDepth guards against runaway or genuinely cyclic schema graphs. Generation
// terminates naturally well before this in every known UBL 2.1 schema (see
// the cycle-cutoff note below); the cap exists purely as a safety net.
const maxDepth = 60

// maxOccursSample caps how many times a single repeatable element is emitted
// so a schema with a large explicit minOccurs cannot blow up the output. No
// element in the UBL 2.1 distribution declares minOccurs greater than 1, so
// in practice every element is emitted exactly once.
const maxOccursSample = 5

// expandElement renders a single XML element (and, recursively, its content
// model) for the given element declaration. elementName/prefix are the
// qualified name to render; they can differ from element's own Name when the
// element is reached through an xsd:element ref. ns is the namespace that
// unprefixed type/ref/base attributes on element (and on the type ultimately
// resolved for it) are interpreted relative to - the target namespace of the
// schema document that declared element.
//
// The returned bool reports whether generation was possible at all: it is
// false only when the element's content model requires a construct this
// generator cannot produce (currently just a required xsd:any wildcard).
// Callers skip an unsupported element if it was itself optional, or
// propagate the failure upward if it was required.
func expandElement(index schemaIndex, element xsdElementDecl, elementName, prefix, ns string, state genState, depth int) (xmlNode, bool) {
	node := xmlNode{Prefix: prefix, Name: elementName}
	if depth > maxDepth {
		node.Value = sampleValueForType(elementName, element.Type, 0)
		return node, true
	}

	// A bare ref (no local type/content override) delegates entirely to the
	// referenced global element's own declaration, but keeps rendering under
	// elementName/prefix as resolved by the caller.
	if element.Ref != "" && element.Type == "" && element.ComplexType == nil &&
		element.SimpleType == nil && element.Sequence == nil && element.Choice == nil && element.All == nil {
		refNS, refName := qualifyRef(element.Ref, ns)
		if target, ok := index.GlobalElements[qualifiedKey(refNS, refName)]; ok {
			return expandElement(index, target, elementName, prefix, refNS, state, depth)
		}
	}

	typeName := element.Type
	if typeName != "" && isPrimitiveType(typeName) {
		node.Value = sampleValueForType(elementName, typeName, 0)
		return node, true
	}

	var ct *xsdComplexType
	var typeNS string
	switch {
	case element.ComplexType != nil:
		ct = element.ComplexType
		typeNS = ns
	case typeName != "":
		tNS, tName := qualifyRef(typeName, ns)
		if t, ok := index.GlobalTypes[qualifiedKey(tNS, tName)]; ok {
			typeCopy := t
			ct = &typeCopy
			typeNS = tNS
		} else if _, ok := index.GlobalSimpleTypes[qualifiedKey(tNS, tName)]; ok {
			node.Value = sampleScalarForType(index, elementName, typeName, tNS, 0)
			return node, true
		}
	}

	if ct != nil {
		if ct.SimpleContent != nil {
			node.Attributes = requiredAttributesForComplexType(index, *ct, typeNS, map[string]bool{}, 0)
			node.Value = sampleScalarForType(index, elementName, typeName, typeNS, 0)
			return node, true
		}
		if ct.Name != "" {
			cycleKey := qualifiedKey(typeNS, ct.Name)
			if state.full {
				if state.visitedFull[cycleKey] {
					// This exact type already received a full (all-elements)
					// expansion earlier in the document; reusing it again in
					// full would revisit the same densely cross-referenced
					// subgraph repeatedly, so fall back to a fast,
					// required-only expansion instead.
					state = state.demoted().withAncestor(cycleKey)
				} else {
					state.visitedFull[cycleKey] = true
				}
			} else if state.ancestors[cycleKey] {
				// Required structural cycle in required-only mode: every
				// self-referential UBL 2.1 type observed in this position
				// has an all-optional top-level content model (a schema
				// could not otherwise be satisfiable by any finite
				// document), so cutting off here and leaving the element
				// childless is always legal - never fabricate text on an
				// element-only type.
				return node, true
			} else {
				state = state.withAncestor(cycleKey)
			}
		}
		node.Attributes = requiredAttributesForComplexType(index, *ct, typeNS, map[string]bool{}, 0)
		// Any complex type reaching this point has no simple content: it is
		// either element-only or attribute-only. Neither permits text, so no
		// Value fallback is applied here.
		children, supported := flattenContentModel(ct.Sequence, ct.Choice, ct.All, index, typeNS, state, depth+1)
		if !supported {
			return node, false
		}
		node.Children = children
		return node, true
	}

	// Inline group declared directly on the element (no named/anonymous type).
	if element.Sequence != nil || element.Choice != nil || element.All != nil {
		children, supported := flattenContentModel(element.Sequence, element.Choice, element.All, index, ns, state, depth+1)
		if !supported {
			return node, false
		}
		node.Children = children
		return node, true
	}
	node.Value = sampleValueForType(elementName, typeName, 0)
	return node, true
}

// flattenContentModel dispatches to the sequence/choice/all flattener
// matching whichever content model a complex type or inline group declares.
// A type with none of the three (attribute-only or entirely empty) is
// trivially supported with no children.
func flattenContentModel(seq *xsdSequence, choice *xsdChoice, all *xsdAll, index schemaIndex, ns string, state genState, depth int) ([]xmlNode, bool) {
	switch {
	case seq != nil:
		if sequenceRequiresAny(seq) {
			// A required xsd:any wildcard has no concrete model to render;
			// this content model cannot be legally satisfied by this
			// generator (see .github/create-samples.md section 9: unsupported
			// constructs must be reported, not silently ignored).
			return nil, false
		}
		return flattenSequence(*seq, index, ns, state, depth)
	case choice != nil:
		return flattenChoice(*choice, index, ns, state, depth)
	case all != nil:
		return flattenAll(*all, index, ns, state, depth)
	}
	return nil, true
}

// sequenceRequiresAny reports whether seq's own top-level xsd:any wildcard
// (if any) is mandatory (minOccurs != "0"). UBL 2.1's only reachable use of
// xsd:any is the required wildcard inside ExtensionContentType (the payload
// of ext:UBLExtensions), which is always optional at its point of use, so in
// practice this only ever causes that one optional element to be omitted.
func sequenceRequiresAny(seq *xsdSequence) bool {
	if seq == nil || seq.Any == nil {
		return false
	}
	return seq.Any.MinOccurs != "0"
}

// flattenSequence emits every element in document order. While state.full is
// true, both required and optional elements are emitted (repeating each the
// number of times its schema demands, capped at maxOccursSample) so the
// generated sample exercises the schema's full content model; once demoted
// (state.full false), optional elements are skipped entirely to keep reused
// shared types cheap, matching the required-only generation policy. An
// element whose own content is unsupported is omitted if it was optional; if
// it was required, the whole sequence becomes unsupported and is propagated
// to the caller.
func flattenSequence(seq xsdSequence, index schemaIndex, ns string, state genState, depth int) ([]xmlNode, bool) {
	var out []xmlNode
	for _, child := range seq.Elements {
		name, prefix := elementNameAndPrefix(child)
		if name == "" {
			continue
		}
		if !state.full && isOptionalElement(child) {
			continue
		}
		node, supported := expandElement(index, child, name, prefix, ns, state, depth)
		if !supported {
			if isOptionalElement(child) {
				continue
			}
			return nil, false
		}
		for i := 0; i < occurrenceCount(child); i++ {
			out = append(out, node)
		}
	}
	return out, true
}

// flattenChoice deterministically selects the first alternative whose
// content is actually supported, which satisfies the choice's own
// cardinality while keeping generation stable. If every alternative is
// unsupported, the choice itself is unsupported.
func flattenChoice(choice xsdChoice, index schemaIndex, ns string, state genState, depth int) ([]xmlNode, bool) {
	for _, alt := range choice.Elements {
		name, prefix := elementNameAndPrefix(alt)
		if name == "" {
			continue
		}
		node, supported := expandElement(index, alt, name, prefix, ns, state, depth)
		if !supported {
			continue
		}
		occurs := occurrenceCount(alt)
		if occurs == 0 {
			// The choice itself must select a branch even if that branch's
			// own minOccurs is 0.
			occurs = 1
		}
		out := make([]xmlNode, 0, occurs)
		for i := 0; i < occurs; i++ {
			out = append(out, node)
		}
		return out, true
	}
	return nil, false
}

// flattenAll emits every member of an xsd:all group while state.full is true
// (required and optional alike), or only required members once demoted;
// xsd:all only permits minOccurs of 0 or 1 so at most one instance of each is
// produced.
func flattenAll(all xsdAll, index schemaIndex, ns string, state genState, depth int) ([]xmlNode, bool) {
	var out []xmlNode
	for _, child := range all.Elements {
		name, prefix := elementNameAndPrefix(child)
		if name == "" {
			continue
		}
		if !state.full && isOptionalElement(child) {
			continue
		}
		node, supported := expandElement(index, child, name, prefix, ns, state, depth)
		if !supported {
			if isOptionalElement(child) {
				continue
			}
			return nil, false
		}
		out = append(out, node)
	}
	return out, true
}

func elementNameAndPrefix(element xsdElementDecl) (string, string) {
	if element.Ref != "" {
		return localPart(element.Ref), prefixPart(element.Ref)
	}
	return element.Name, ""
}

func isOptionalElement(element xsdElementDecl) bool {
	return element.MinOccurs == "0"
}

// occurrenceCount returns how many times an element should be emitted: its
// declared minOccurs (defaulting to, and never falling below, 1 so that
// optional elements are still represented at least once), capped at
// maxOccursSample.
func occurrenceCount(element xsdElementDecl) int {
	n := parseOccursInt(element.MinOccurs, 1)
	if n < 1 {
		n = 1
	}
	if n > maxOccursSample {
		n = maxOccursSample
	}
	return n
}

func parseOccursInt(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	return n
}

func cloneAncestors(ancestors map[string]bool) map[string]bool {
	out := make(map[string]bool, len(ancestors))
	for k, v := range ancestors {
		out[k] = v
	}
	return out
}

func withAncestor(ancestors map[string]bool, name string) map[string]bool {
	out := cloneAncestors(ancestors)
	out[name] = true
	return out
}

// requiredAttributesForComplexType collects every use="required" attribute
// declared on ct, following xsd:extension/xsd:restriction base chains (by
// local type name) so attributes declared on a wrapped base type - e.g.
// currencyID on AmountType, unitCode on MeasureType - are still produced for
// the many UBL types that merely extend those base types with no attributes
// of their own.
func requiredAttributesForComplexType(index schemaIndex, ct xsdComplexType, ns string, visited map[string]bool, depth int) map[string]string {
	if depth > 10 {
		return nil
	}
	attrs := map[string]string{}
	collect := func(list []xsdAttribute) {
		for _, attr := range list {
			if attr.Name == "" || attr.Use != "required" {
				continue
			}
			attrs[attr.Name] = sampleValueForType(attr.Name, attr.Type, 0)
		}
	}
	collect(ct.Attributes)
	if ct.SimpleContent != nil {
		if r := ct.SimpleContent.Restriction; r != nil {
			collect(r.Attributes)
			mergeBaseTypeAttributes(index, r.Base, ns, visited, depth, attrs)
		}
		if e := ct.SimpleContent.Extension; e != nil {
			collect(e.Attributes)
			mergeBaseTypeAttributes(index, e.Base, ns, visited, depth, attrs)
		}
	}
	if len(attrs) == 0 {
		return nil
	}
	return attrs
}

func mergeBaseTypeAttributes(index schemaIndex, base, ns string, visited map[string]bool, depth int, attrs map[string]string) {
	baseNS, name := qualifyRef(base, ns)
	key := qualifiedKey(baseNS, name)
	if name == "" || visited[key] {
		return
	}
	baseType, ok := index.GlobalTypes[key]
	if !ok {
		return
	}
	visited = withAncestor(visited, key)
	for k, v := range requiredAttributesForComplexType(index, baseType, baseNS, visited, depth+1) {
		if _, exists := attrs[k]; !exists {
			attrs[k] = v
		}
	}
}
