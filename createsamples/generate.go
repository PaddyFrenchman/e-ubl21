// This file implements the UBL 2.1 sample document generator: given a root
// element declaration, it recursively expands the schema's content model
// into an in-memory xmlNode tree, which the xmlnode package then serializes
// to XML.
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

	"github.com/PaddyFrenchman/e-ubl21/createsamples/xmlnode"
)

// xmlNode is a type alias (not a distinct type) for xmlnode.Node: the tree
// type was extracted into the importable xmlnode package so that
// createsamples/en16931's semantic generator can build and render the same
// tree shape too (package main cannot be imported from elsewhere - see
// ideas/EN16931-samples-generator-plan.md §4/§10). The alias keeps every
// existing xmlNode{...} literal and []xmlNode usage below unchanged.
type xmlNode = xmlnode.Node

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
	// choiceAlternative selects a deterministic branch at every choice site.
	// Zero preserves the historic first-supported-branch behaviour; structural
	// corpus variants use the later values to exercise the other branches.
	choiceAlternative    int
	choiceSelections     map[int]int
	nextChoice           *int
	choiceAlternatives   *map[int]int
	includeOptionalAttrs bool
	structural           bool
	recursiveDepthLimit  int
	facetBoundary        int
	// repeatCount exercises every repeatable particle at this cardinality. A
	// finite schema maximum is always respected. repeatOverrides is used for
	// deliberately large, line-oriented documents without inflating unrelated
	// collections in the same document.
	repeatCount     int
	repeatOverrides map[string]int
	typeDepth       map[string]int
}

func newGenState() genState {
	return genState{full: true, ancestors: map[string]bool{}, visitedFull: map[string]bool{}, recursiveDepthLimit: 1, typeDepth: map[string]int{}}
}

// newStructuralGenState keeps the ordinary shared-type expansion bound. UBL's
// aggregate types form a dense graph, so expanding every optional member on
// every reuse grows exponentially. Structural families cover reused types by
// varying their entry paths, while this per-fixture bound keeps each fixture
// small and independently useful. recursiveDepthLimit still permits the
// deliberately selected recursive variants to descend safely.
func newStructuralGenState(choiceAlternative, recursiveDepthLimit int, includeOptionalAttrs bool) genState {
	if recursiveDepthLimit < 1 {
		recursiveDepthLimit = 1
	}
	nextChoice := 0
	observed := map[int]int{}
	return genState{full: true, ancestors: map[string]bool{}, visitedFull: map[string]bool{}, choiceAlternative: choiceAlternative, choiceSelections: map[int]int{}, nextChoice: &nextChoice, choiceAlternatives: &observed, includeOptionalAttrs: includeOptionalAttrs, structural: true, recursiveDepthLimit: recursiveDepthLimit, typeDepth: map[string]int{}}
}

// newTargetedStructuralGenState selects alternatives independently by traversal-site ordinal.
func newTargetedStructuralGenState(choiceSelections map[int]int, recursiveDepthLimit int) genState {
	s := newStructuralGenState(0, recursiveDepthLimit, true)
	s.choiceSelections = choiceSelections
	return s
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

func (s genState) withTypeDepth(key string) genState {
	s.typeDepth = cloneTypeDepth(s.typeDepth)
	s.typeDepth[key]++
	return s
}

func cloneTypeDepth(depths map[string]int) map[string]int {
	out := make(map[string]int, len(depths))
	for key, depth := range depths {
		out[key] = depth
	}
	return out
}

func generateCompleteSample(index schemaIndex, rootName, xsdDir, schemaFile string) (string, error) {
	return generateSampleWithState(index, rootName, xsdDir, schemaFile, newGenState())
}

func generateStructuralSample(index schemaIndex, rootName, xsdDir, schemaFile string, choiceAlternative, recursiveDepth int) (string, error) {
	return generateSampleWithState(index, rootName, xsdDir, schemaFile, newStructuralGenState(choiceAlternative, recursiveDepth, true))
}

func generateTargetedStructuralSample(index schemaIndex, rootName, xsdDir, schemaFile string, choices map[int]int, recursiveDepth int) (string, error) {
	return generateSampleWithState(index, rootName, xsdDir, schemaFile, newTargetedStructuralGenState(choices, recursiveDepth))
}

func generateFacetBoundaryStructuralSample(index schemaIndex, rootName, xsdDir, schemaFile string, boundary int) (string, error) {
	state := newTargetedStructuralGenState(nil, 1)
	state.facetBoundary = boundary
	return generateSampleWithState(index, rootName, xsdDir, schemaFile, state)
}

func generateCardinalityStructuralSample(index schemaIndex, rootName, xsdDir, schemaFile string, count int) (string, error) {
	state := newTargetedStructuralGenState(nil, 1)
	state.repeatCount = count
	return generateSampleWithState(index, rootName, xsdDir, schemaFile, state)
}

func generateLargeCollectionStructuralSample(index schemaIndex, rootName, xsdDir, schemaFile, element string, count int) (string, error) {
	return generateLargeCollectionsStructuralSample(index, rootName, xsdDir, schemaFile, map[string]int{element: count})
}

func generateLargeCollectionsStructuralSample(index schemaIndex, rootName, xsdDir, schemaFile string, elements map[string]int) (string, error) {
	state := newTargetedStructuralGenState(nil, 1)
	state.repeatOverrides = elements
	return generateSampleWithState(index, rootName, xsdDir, schemaFile, state)
}

// structuralChoiceSiteCount counts choice sites reached while expanding this
// root. Choice-site ordinals are traversal-local: counting every choice in
// the global schema graph would create variants for unrelated document types.
func structuralChoiceSiteCount(index schemaIndex, rootName, xsdDir, schemaFile string) (int, error) {
	state := newTargetedStructuralGenState(nil, 1)
	if _, err := generateSampleWithState(index, rootName, xsdDir, schemaFile, state); err != nil {
		return 0, err
	}
	return *state.nextChoice, nil
}

// structuralChoicePlans exhaustively explores supported choice paths. Each
// generated path reports the choice sites it actually reaches; alternatives
// discovered beneath a non-default parent alternative are then queued too.
func structuralChoicePlans(index schemaIndex, rootName, xsdDir, schemaFile string) ([]map[int]int, error) {
	queue := []map[int]int{{}}
	seen := map[string]bool{}
	plans := []map[int]int{}
	for len(queue) > 0 {
		selection := queue[0]
		queue = queue[1:]
		key := fmt.Sprint(selection)
		if seen[key] {
			continue
		}
		seen[key] = true
		state := newTargetedStructuralGenState(selection, 1)
		if _, err := generateSampleWithState(index, rootName, xsdDir, schemaFile, state); err != nil {
			return nil, err
		}
		plans = append(plans, selection)
		for site, alternatives := range *state.choiceAlternatives {
			for alternative := 1; alternative < alternatives; alternative++ {
				next := make(map[int]int, len(selection)+1)
				for k, v := range selection {
					next[k] = v
				}
				next[site] = alternative
				queue = append(queue, next)
			}
		}
	}
	return plans, nil
}

func generateMinimalSample(index schemaIndex, rootName, xsdDir, schemaFile string) (string, error) {
	state := newGenState()
	state.full = false
	return generateSampleWithState(index, rootName, xsdDir, schemaFile, state)
}

func generateSampleWithState(index schemaIndex, rootName, xsdDir, schemaFile string, state genState) (string, error) {
	rootNS, err := rootNamespaceForSchema(xsdDir, schemaFile)
	if err != nil {
		return "", err
	}
	rootDecl, ok := index.GlobalElements[qualifiedKey(rootNS, rootName)]
	if !ok {
		return "", fmt.Errorf("root element %q not found in schema index", rootName)
	}
	rootNode, supported := expandElement(index, rootDecl, rootName, "", rootNS, state, 0)
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
	if state.structural {
		rootNode.Attributes["xmlns:struct"] = "urn:e-ubl21:structural-fixture"
	}
	return "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" + xmlnode.Render(rootNode, 0) + "\n", nil
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
			// A reference within a common-component schema is often
			// unprefixed (for example UBLExtension inside the extension
			// schema). It still belongs to that common-component namespace,
			// not the main-document default namespace used by the rendered
			// root, so give it the conventional UBL prefix.
			if state.structural && prefix == "" {
				prefix = ublNamespacePrefix(refNS)
			}
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
			node.Value = sampleScalarForType(index, elementName, typeName, tNS, state.facetBoundary)
			return node, true
		}
	}

	if ct != nil {
		if ct.SimpleContent != nil {
			node.Attributes = attributesForComplexType(index, *ct, typeNS, map[string]bool{}, 0, state.includeOptionalAttrs)
			node.Value = sampleScalarForType(index, elementName, typeName, typeNS, state.facetBoundary)
			return node, true
		}
		if ct.Name != "" {
			cycleKey := qualifiedKey(typeNS, ct.Name)
			if state.structural {
				if state.typeDepth[cycleKey] >= state.recursiveDepthLimit {
					// Do not leave an element-only type empty merely because a
					// recursive budget was reached: it may still have mandatory
					// children. Required-only expansion below preserves schema
					// validity while stopping optional graph growth.
					state = state.demoted()
				} else {
					state = state.withTypeDepth(cycleKey)
				}
			}
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
		node.Attributes = attributesForComplexType(index, *ct, typeNS, map[string]bool{}, 0, state.includeOptionalAttrs)
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
		if sequenceRequiresAny(seq) && !state.structural {
			// A required xsd:any wildcard has no concrete model to render;
			// this content model cannot be legally satisfied by this
			// generator (see .github/create-samples.md section 9: unsupported
			// constructs must be reported, not silently ignored).
			return nil, false
		}
		return flattenSequence(*seq, index, ns, state, depth)
	case choice != nil:
		if !state.full && choice.MinOccurs == "0" {
			return nil, true
		}
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
		for i := 0; i < occurrenceCount(child, state, depth); i++ {
			out = append(out, node)
		}
	}
	for i := range seq.Sequences {
		children, supported := flattenSequence(seq.Sequences[i], index, ns, state, depth)
		if !supported {
			return nil, false
		}
		out = append(out, children...)
	}
	for i := range seq.Choices {
		if !state.full && seq.Choices[i].MinOccurs == "0" {
			continue
		}
		children, supported := flattenChoice(seq.Choices[i], index, ns, state, depth)
		if !supported {
			return nil, false
		}
		out = append(out, children...)
	}
	for i := range seq.Alls {
		children, supported := flattenAll(seq.Alls[i], index, ns, state, depth)
		if !supported {
			return nil, false
		}
		out = append(out, children...)
	}
	if seq.Any != nil && state.structural {
		if node, ok := structuralWildcardNode(*seq.Any); ok {
			out = append(out, node)
		}
	}
	return out, true
}

// structuralWildcardNode is deliberately foreign to every UBL namespace.
// UBL extension points declare xsd:any with lax processing, so this proves
// namespace-qualified pass-through without presenting private data as UBL
// business content.
func structuralWildcardNode(any xsdAny) (xmlNode, bool) {
	// Strict processing needs a declared permitted element; do not invent one.
	if any.ProcessContents == "strict" {
		return xmlNode{}, false
	}
	switch any.Namespace {
	case "##local":
		return xmlNode{Name: "Wildcard", Attributes: map[string]string{"xmlns": "", "purpose": "schema-coverage"}, Value: "sample"}, true
	case "##targetNamespace":
		return xmlNode{}, false
	default: // ##any, ##other, and explicit foreign namespace lists
		return xmlNode{Prefix: "struct", Name: "Extension", Attributes: map[string]string{"purpose": "schema-coverage", "namespacePolicy": any.Namespace, "processContents": any.ProcessContents}, Children: []xmlNode{{Prefix: "struct", Name: "Value", Value: "sample"}}}, true
	}
}

// flattenChoice deterministically selects the first alternative whose
// content is actually supported, which satisfies the choice's own
// cardinality while keeping generation stable. If every alternative is
// unsupported, the choice itself is unsupported.
func flattenChoice(choice xsdChoice, index schemaIndex, ns string, state genState, depth int) ([]xmlNode, bool) {
	type alternative func() ([]xmlNode, bool)
	alternatives := make([]alternative, 0, len(choice.Elements)+len(choice.Sequences)+len(choice.Choices)+len(choice.Alls))
	for _, element := range choice.Elements {
		alt := element
		alternatives = append(alternatives, func() ([]xmlNode, bool) {
			name, prefix := elementNameAndPrefix(alt)
			if name == "" {
				return nil, false
			}
			node, supported := expandElement(index, alt, name, prefix, ns, state, depth)
			if !supported {
				return nil, false
			}
			occurs := occurrenceCount(alt, state, depth)
			if occurs == 0 {
				occurs = 1
			}
			out := make([]xmlNode, 0, occurs)
			for i := 0; i < occurs; i++ {
				out = append(out, node)
			}
			return out, true
		})
	}
	for i := range choice.Sequences {
		alt := choice.Sequences[i]
		alternatives = append(alternatives, func() ([]xmlNode, bool) { return flattenSequence(alt, index, ns, state, depth) })
	}
	for i := range choice.Choices {
		alt := choice.Choices[i]
		alternatives = append(alternatives, func() ([]xmlNode, bool) { return flattenChoice(alt, index, ns, state, depth) })
	}
	for i := range choice.Alls {
		alt := choice.Alls[i]
		alternatives = append(alternatives, func() ([]xmlNode, bool) { return flattenAll(alt, index, ns, state, depth) })
	}
	selection := state.choiceAlternative
	site := -1
	if state.nextChoice != nil {
		site = *state.nextChoice
		(*state.nextChoice)++
		if selected, ok := state.choiceSelections[site]; ok {
			selection = selected
		}
	}
	if state.choiceAlternatives != nil && site >= 0 {
		(*state.choiceAlternatives)[site] = len(alternatives)
	}
	for offset := range alternatives {
		out, supported := alternatives[(selection+offset)%len(alternatives)]()
		if supported {
			return out, true
		}
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
	for i := range all.Sequences {
		children, supported := flattenSequence(all.Sequences[i], index, ns, state, depth)
		if !supported {
			return nil, false
		}
		out = append(out, children...)
	}
	for i := range all.Choices {
		if !state.full && all.Choices[i].MinOccurs == "0" {
			continue
		}
		children, supported := flattenChoice(all.Choices[i], index, ns, state, depth)
		if !supported {
			return nil, false
		}
		out = append(out, children...)
	}
	for i := range all.Alls {
		children, supported := flattenAll(all.Alls[i], index, ns, state, depth)
		if !supported {
			return nil, false
		}
		out = append(out, children...)
	}
	if all.Any != nil && state.structural {
		if node, ok := structuralWildcardNode(*all.Any); ok {
			out = append(out, node)
		}
	}
	return out, true
}

func elementNameAndPrefix(element xsdElementDecl) (string, string) {
	if element.Ref != "" {
		return localPart(element.Ref), prefixPart(element.Ref)
	}
	return element.Name, ""
}

func ublNamespacePrefix(namespace string) string {
	switch namespace {
	case "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2":
		return "cbc"
	case "urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2":
		return "cac"
	case "urn:oasis:names:specification:ubl:schema:xsd:CommonExtensionComponents-2":
		return "ext"
	}
	return ""
}

func isOptionalElement(element xsdElementDecl) bool {
	return element.MinOccurs == "0"
}

// occurrenceCount returns how many times an element should be emitted: its
// declared minOccurs (defaulting to, and never falling below, 1 so that
// optional elements are still represented at least once), capped at
// maxOccursSample.
func occurrenceCount(element xsdElementDecl, state genState, depth int) int {
	n := parseOccursInt(element.MinOccurs, 1)
	if n < 1 {
		n = 1
	}
	if desired, ok := state.repeatOverrides[elementNameForOccurrence(element)]; ok && depth == 1 && isRepeatableElement(element) {
		n = desired
		// Targeted and broad cardinality families are deliberately limited to a
		// document's direct collections. Repeating a same-named nested aggregate
		// as well would distort the requested count and can create an exponential
		// document, neither of which is realistic or useful.
	} else if state.repeatCount > 0 && depth == 1 && isRepeatableElement(element) {
		n = state.repeatCount
	}
	max := parseMaxOccurs(element.MaxOccurs)
	if max > 0 && n > max {
		n = max
	}
	if state.repeatCount == 0 && len(state.repeatOverrides) == 0 && n > maxOccursSample {
		n = maxOccursSample
	}
	return n
}

func elementNameForOccurrence(element xsdElementDecl) string {
	if element.Ref != "" {
		return localPart(element.Ref)
	}
	return element.Name
}

func isRepeatableElement(element xsdElementDecl) bool {
	return element.MaxOccurs == "unbounded" || parseOccursInt(element.MaxOccurs, 1) > 1
}

func parseMaxOccurs(s string) int {
	if s == "" || s == "unbounded" {
		return 0
	}
	return parseOccursInt(s, 1)
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
	return attributesForComplexType(index, ct, ns, visited, depth, false)
}

// attributesForComplexType follows the same inheritance chain as the
// baseline generator but can include optional attributes for structural
// fixtures. Attribute-only and simple-content types use this path too.
func attributesForComplexType(index schemaIndex, ct xsdComplexType, ns string, visited map[string]bool, depth int, includeOptional bool) map[string]string {
	if depth > 10 {
		return nil
	}
	attrs := map[string]string{}
	collect := func(list []xsdAttribute) {
		for _, attr := range list {
			if attr.Name == "" || attr.Use == "prohibited" || (attr.Use != "required" && !includeOptional) {
				continue
			}
			attrs[attr.Name] = sampleValueForType(attr.Name, attr.Type, 0)
		}
	}
	collect(ct.Attributes)
	if ct.SimpleContent != nil {
		if r := ct.SimpleContent.Restriction; r != nil {
			collect(r.Attributes)
			mergeBaseTypeAttributes(index, r.Base, ns, visited, depth, attrs, includeOptional)
		}
		if e := ct.SimpleContent.Extension; e != nil {
			collect(e.Attributes)
			mergeBaseTypeAttributes(index, e.Base, ns, visited, depth, attrs, includeOptional)
		}
	}
	if len(attrs) == 0 {
		return nil
	}
	return attrs
}

func mergeBaseTypeAttributes(index schemaIndex, base, ns string, visited map[string]bool, depth int, attrs map[string]string, includeOptional bool) {
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
	for k, v := range attributesForComplexType(index, baseType, baseNS, visited, depth+1, includeOptional) {
		if _, exists := attrs[k]; !exists {
			attrs[k] = v
		}
	}
}
