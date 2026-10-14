package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
)

type schemaFrame struct {
	doc      *schemaDoc
	typ      xsdComplexType
	typeName string
	targets  map[string]int
	siblings map[string]int
	seen     map[string]bool
}

// fixtureMatch retains the existing target maximums while preserving sibling
// counts for every individual parent instance. Cardinality coverage must not
// reduce these observations to one maximum per fixture: one document may
// intentionally contain parents with zero, one, and many children.
type fixtureMatch struct {
	targets         map[string]int
	cardinalities   map[string][]int
	parentInstances map[string]int
}

func complexTypeIDForElement(typeDoc *schemaDoc, elementID, typeName string) string {
	if typeName == "inline" {
		return strings.Replace(elementID, "|element|", "|complexType|inline/", 1)
	}
	if typeDoc == nil {
		return ""
	}
	return fmt.Sprintf("%s|%s|complexType|%s", typeDoc.TargetNamespace, typeDoc.File, typeName)
}

func elementDeclaration(index schemaIndex, doc *schemaDoc, decl xsdElementDecl) (*schemaDoc, xsdElementDecl, string, string) {
	if decl.Ref != "" {
		ns, n := qualifyRef(decl.Ref, doc.TargetNamespace)
		key := qualifiedKey(ns, n)
		if d := index.GlobalElementDocs[key]; d != nil {
			return d, index.GlobalElements[key], ns, n
		}
		if d := index.ByTargetNS[ns]; d != nil {
			if x, ok := d.Elements[n]; ok {
				return d, x, ns, n
			}
		}
		return doc, decl, ns, n
	}
	return doc, decl, doc.TargetNamespace, decl.Name
}

// recordGlobalElementUse credits a referenced global declaration in addition
// to the containing complex type's particle. Coverage inventories both kinds
// of targets: the local particle describes where the element appears, while
// the global target describes the shared declaration used by xsd:element ref.
func recordGlobalElementUse(index schemaIndex, targets map[string]int, parentDoc *schemaDoc, decl xsdElementDecl, actual xml.Name, substitutionID string) {
	if decl.Ref != "" {
		doc, _, _, name := elementDeclaration(index, parentDoc, decl)
		if doc != nil {
			targets[coverageID(doc, "element", "global/"+name+"/"+name)]++
		}
	}
	if substitutionID != "" {
		key := qualifiedKey(actual.Space, actual.Local)
		if doc := index.GlobalElementDocs[key]; doc != nil {
			targets[coverageID(doc, "element", "global/"+actual.Local+"/"+actual.Local)]++
		}
	}
}

func substitutionMember(index schemaIndex, doc *schemaDoc, head xsdElementDecl, q xml.Name) (xsdElementDecl, string, bool) {
	headDoc, resolvedHead, headNS, headName := elementDeclaration(index, doc, head)
	_ = headDoc
	for _, candidateDoc := range index.ByTargetNS {
		for name, candidate := range candidateDoc.Elements {
			groupNS, groupName := qualifyRef(candidate.SubstitutionGroup, candidateDoc.TargetNamespace)
			if candidate.SubstitutionGroup == "" || groupNS != headNS || groupName != headName || candidateDoc.TargetNamespace != q.Space || name != q.Local {
				continue
			}
			return candidate, coverageID(candidateDoc, "substitution", name+"->"+headNS+"#"+resolvedHead.Name), true
		}
	}
	return xsdElementDecl{}, "", false
}
func findSubstitutionInSequence(index schemaIndex, f schemaFrame, parent string, seq *xsdSequence, q xml.Name) (xsdElementDecl, string, string, bool) {
	if seq == nil {
		return xsdElementDecl{}, "", "", false
	}
	for i, x := range seq.Elements {
		if member, sub, ok := substitutionMember(index, f.doc, x, q); ok {
			return member, fmt.Sprintf("%s|%s|element|%s/sequence[%d]/%s", f.doc.TargetNamespace, f.doc.File, parent, i, localPart(x.Ref)), sub, true
		}
	}
	for i := range seq.Sequences {
		if x, id, sub, ok := findSubstitutionInSequence(index, f, parent+fmt.Sprintf("/sequence[%d]", i), &seq.Sequences[i], q); ok {
			return x, id, sub, true
		}
	}
	for i := range seq.Choices {
		if x, id, sub, ok := findSubstitutionInChoice(index, f, parent+fmt.Sprintf("/choice[%d]", i), &seq.Choices[i], q); ok {
			return x, id, sub, true
		}
	}
	for i := range seq.Alls {
		if x, id, sub, ok := findSubstitutionInAll(index, f, parent+fmt.Sprintf("/all[%d]", i), &seq.Alls[i], q); ok {
			return x, id, sub, true
		}
	}
	return xsdElementDecl{}, "", "", false
}
func findSubstitutionInChoice(index schemaIndex, f schemaFrame, parent string, choice *xsdChoice, q xml.Name) (xsdElementDecl, string, string, bool) {
	if choice == nil {
		return xsdElementDecl{}, "", "", false
	}
	for i, x := range choice.Elements {
		if member, sub, ok := substitutionMember(index, f.doc, x, q); ok {
			return member, fmt.Sprintf("%s|%s|element|%s/choice[%d]/%s", f.doc.TargetNamespace, f.doc.File, parent, i, localPart(x.Ref)), sub, true
		}
	}
	for i := range choice.Sequences {
		if x, id, sub, ok := findSubstitutionInSequence(index, f, parent+fmt.Sprintf("/sequence[%d]", i), &choice.Sequences[i], q); ok {
			return x, id, sub, true
		}
	}
	for i := range choice.Choices {
		if x, id, sub, ok := findSubstitutionInChoice(index, f, parent+fmt.Sprintf("/choice[%d]", i), &choice.Choices[i], q); ok {
			return x, id, sub, true
		}
	}
	for i := range choice.Alls {
		if x, id, sub, ok := findSubstitutionInAll(index, f, parent+fmt.Sprintf("/all[%d]", i), &choice.Alls[i], q); ok {
			return x, id, sub, true
		}
	}
	return xsdElementDecl{}, "", "", false
}
func findSubstitutionInAll(index schemaIndex, f schemaFrame, parent string, all *xsdAll, q xml.Name) (xsdElementDecl, string, string, bool) {
	if all == nil {
		return xsdElementDecl{}, "", "", false
	}
	for i, x := range all.Elements {
		if member, sub, ok := substitutionMember(index, f.doc, x, q); ok {
			return member, fmt.Sprintf("%s|%s|element|%s/all[%d]/%s", f.doc.TargetNamespace, f.doc.File, parent, i, localPart(x.Ref)), sub, true
		}
	}
	for i := range all.Sequences {
		if x, id, sub, ok := findSubstitutionInSequence(index, f, parent+fmt.Sprintf("/sequence[%d]", i), &all.Sequences[i], q); ok {
			return x, id, sub, true
		}
	}
	for i := range all.Choices {
		if x, id, sub, ok := findSubstitutionInChoice(index, f, parent+fmt.Sprintf("/choice[%d]", i), &all.Choices[i], q); ok {
			return x, id, sub, true
		}
	}
	for i := range all.Alls {
		if x, id, sub, ok := findSubstitutionInAll(index, f, parent+fmt.Sprintf("/all[%d]", i), &all.Alls[i], q); ok {
			return x, id, sub, true
		}
	}
	return xsdElementDecl{}, "", "", false
}
func declaredType(index schemaIndex, doc *schemaDoc, decl xsdElementDecl) (*schemaDoc, xsdComplexType, string, bool) {
	d, x, _, _ := elementDeclaration(index, doc, decl)
	if x.ComplexType != nil {
		return d, *x.ComplexType, "inline", true
	}
	if x.Type == "" {
		return nil, xsdComplexType{}, "", false
	}
	ns, n := qualifyRef(x.Type, d.TargetNamespace)
	td, t, ok := schemaTypeDocument(index, ns, n)
	return td, t, n, ok
}

// schemaTypeDocument resolves types declared in xsd:include files as well as
// in the namespace's primary document.  Includes share a target namespace;
// ByTargetNS deliberately has one representative document, so using it alone
// loses the included ExtensionContentType and makes otherwise observed
// wildcard content invisible to coverage matching.
func schemaTypeDocument(index schemaIndex, namespace, name string) (*schemaDoc, xsdComplexType, bool) {
	if doc := index.GlobalTypeDocs[qualifiedKey(namespace, name)]; doc != nil {
		return doc, index.GlobalTypes[qualifiedKey(namespace, name)], true
	}
	if doc := index.ByTargetNS[namespace]; doc != nil {
		if typ, ok := doc.Types[name]; ok {
			return doc, typ, true
		}
	}
	for _, doc := range index.ByFile {
		if doc == nil || doc.TargetNamespace != namespace {
			continue
		}
		if typ, ok := doc.Types[name]; ok {
			return doc, typ, true
		}
	}
	return nil, xsdComplexType{}, false
}
func baseFrame(index schemaIndex, f schemaFrame) (schemaFrame, bool) {
	base := typeBase(f.typ)
	if base == "" {
		return schemaFrame{}, false
	}
	ns, name := qualifyRef(base, f.doc.TargetNamespace)
	doc, typ, ok := schemaTypeDocument(index, ns, name)
	if !ok {
		return schemaFrame{}, false
	}
	return schemaFrame{doc: doc, typ: typ, typeName: name, targets: f.targets, siblings: map[string]int{}, seen: f.seen}, true
}
func findInSequence(index schemaIndex, f schemaFrame, parent string, seq *xsdSequence, q xml.Name) (xsdElementDecl, string, bool) {
	if seq == nil {
		return xsdElementDecl{}, "", false
	}
	for i, x := range seq.Elements {
		_, _, ns, n := elementDeclaration(index, f.doc, x)
		if ns == q.Space && n == q.Local {
			return x, fmt.Sprintf("%s|%s|element|%s/sequence[%d]/%s", f.doc.TargetNamespace, f.doc.File, parent, i, n), true
		}
	}
	for i := range seq.Sequences {
		if x, id, ok := findInSequence(index, f, parent+fmt.Sprintf("/sequence[%d]", i), &seq.Sequences[i], q); ok {
			return x, id, true
		}
	}
	for i := range seq.Choices {
		if x, id, ok := findInChoice(index, f, parent+fmt.Sprintf("/choice[%d]", i), &seq.Choices[i], q); ok {
			return x, id, true
		}
	}
	for i := range seq.Alls {
		if x, id, ok := findInAll(index, f, parent+fmt.Sprintf("/all[%d]", i), &seq.Alls[i], q); ok {
			return x, id, true
		}
	}
	return xsdElementDecl{}, "", false
}
func findInChoice(index schemaIndex, f schemaFrame, parent string, choice *xsdChoice, q xml.Name) (xsdElementDecl, string, bool) {
	if choice == nil {
		return xsdElementDecl{}, "", false
	}
	for i, x := range choice.Elements {
		_, _, ns, n := elementDeclaration(index, f.doc, x)
		if ns == q.Space && n == q.Local {
			return x, fmt.Sprintf("%s|%s|element|%s/choice[%d]/%s", f.doc.TargetNamespace, f.doc.File, parent, i, n), true
		}
	}
	for i := range choice.Sequences {
		if !requiredBranchElementsObserved(index, f, choice.Sequences[i].Elements, q) {
			continue
		}
		if x, id, ok := findInSequence(index, f, parent+fmt.Sprintf("/sequence[%d]", i), &choice.Sequences[i], q); ok {
			return x, id, true
		}
	}
	for i := range choice.Choices {
		if x, id, ok := findInChoice(index, f, parent+fmt.Sprintf("/choice[%d]", i), &choice.Choices[i], q); ok {
			return x, id, true
		}
	}
	for i := range choice.Alls {
		if !requiredBranchElementsObserved(index, f, choice.Alls[i].Elements, q) {
			continue
		}
		if x, id, ok := findInAll(index, f, parent+fmt.Sprintf("/all[%d]", i), &choice.Alls[i], q); ok {
			return x, id, true
		}
	}
	return xsdElementDecl{}, "", false
}

func requiredBranchElementsObserved(index schemaIndex, f schemaFrame, elements []xsdElementDecl, child xml.Name) bool {
	for _, element := range elements {
		if element.MinOccurs == "0" {
			continue
		}
		_, _, namespace, name := elementDeclaration(index, f.doc, element)
		if namespace == child.Space && name == child.Local {
			continue
		}
		if !f.seen[qualifiedKey(namespace, name)] {
			return false
		}
	}
	return true
}

func findInAll(index schemaIndex, f schemaFrame, parent string, all *xsdAll, q xml.Name) (xsdElementDecl, string, bool) {
	if all == nil {
		return xsdElementDecl{}, "", false
	}
	for i, x := range all.Elements {
		_, _, ns, n := elementDeclaration(index, f.doc, x)
		if ns == q.Space && n == q.Local {
			return x, fmt.Sprintf("%s|%s|element|%s/all[%d]/%s", f.doc.TargetNamespace, f.doc.File, parent, i, n), true
		}
	}
	for i := range all.Sequences {
		if x, id, ok := findInSequence(index, f, parent+fmt.Sprintf("/sequence[%d]", i), &all.Sequences[i], q); ok {
			return x, id, true
		}
	}
	for i := range all.Choices {
		if x, id, ok := findInChoice(index, f, parent+fmt.Sprintf("/choice[%d]", i), &all.Choices[i], q); ok {
			return x, id, true
		}
	}
	for i := range all.Alls {
		if x, id, ok := findInAll(index, f, parent+fmt.Sprintf("/all[%d]", i), &all.Alls[i], q); ok {
			return x, id, true
		}
	}
	return xsdElementDecl{}, "", false
}
func recordInheritance(index schemaIndex, f schemaFrame) {
	seen := map[string]struct{}{}
	for f.doc != nil && f.typeName != "" {
		key := qualifiedKey(f.doc.TargetNamespace, f.typeName)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		f.targets[complexTypeIDForElement(f.doc, "", f.typeName)]++
		if simple, ok := f.doc.SimpleTypes[f.typeName]; ok && simple.Restriction != nil {
			baseNS, baseName := qualifyRef(simple.Restriction.Base, f.doc.TargetNamespace)
			if baseName != "" {
				f.targets[coverageID(f.doc, "inheritance", f.typeName+"->"+baseNS+"#"+baseName)]++
			}
			return
		}
		base := typeBase(f.typ)
		if base == "" {
			return
		}
		ns, name := qualifyRef(base, f.doc.TargetNamespace)
		f.targets[fmt.Sprintf("%s|%s|inheritance|%s->%s#%s", f.doc.TargetNamespace, f.doc.File, f.typeName, ns, name)]++
		next, ok := baseFrame(index, f)
		if !ok {
			return
		}
		f = next
	}
}
func particleElement(index schemaIndex, f schemaFrame, q xml.Name) (xsdElementDecl, string, string, bool) {
	for _, p := range typeParticleSets(f.typ) {
		if x, id, ok := findInSequence(index, f, f.typeName, p.sequence, q); ok {
			return x, id, "", true
		}
		if x, id, ok := findInChoice(index, f, f.typeName, p.choice, q); ok {
			return x, id, "", true
		}
		if x, id, ok := findInAll(index, f, f.typeName, p.all, q); ok {
			return x, id, "", true
		}
	}
	check := func(kind string, xs []xsdElementDecl) (xsdElementDecl, string, bool) {
		for i, x := range xs {
			_, resolved, ns, n := elementDeclaration(index, f.doc, x)
			_ = resolved
			if ns == q.Space && n == q.Local {
				return x, fmt.Sprintf("%s|%s|element|%s/%s[%d]/%s", f.doc.TargetNamespace, f.doc.File, f.typeName, kind, i, n), true
			}
		}
		return xsdElementDecl{}, "", false
	}
	for _, p := range typeParticleSets(f.typ) {
		if p.sequence != nil {
			if x, id, ok := check("sequence", p.sequence.Elements); ok {
				return x, id, "", true
			}
		}
		if p.all != nil {
			if x, id, ok := check("all", p.all.Elements); ok {
				return x, id, "", true
			}
		}
		if p.choice != nil {
			if x, id, ok := check("choice", p.choice.Elements); ok {
				return x, id, "", true
			}
		}
	}
	for _, p := range typeParticleSets(f.typ) {
		if x, id, sub, ok := findSubstitutionInSequence(index, f, f.typeName, p.sequence, q); ok {
			return x, id, sub, true
		}
		if x, id, sub, ok := findSubstitutionInChoice(index, f, f.typeName, p.choice, q); ok {
			return x, id, sub, true
		}
		if x, id, sub, ok := findSubstitutionInAll(index, f, f.typeName, p.all, q); ok {
			return x, id, sub, true
		}
	}
	if base, ok := baseFrame(index, f); ok {
		return particleElement(index, base, q)
	}
	return xsdElementDecl{}, "", "", false
}
func recordAttributes(index schemaIndex, f schemaFrame, start xml.StartElement) {
	for _, attribute := range start.Attr {
		for _, declaration := range typeAttributes(f.typ) {
			if declaration.Name == attribute.Name.Local {
				f.targets[fmt.Sprintf("%s|%s|attribute|%s/@%s", f.doc.TargetNamespace, f.doc.File, f.typeName, declaration.Name)]++
				if declaration.Type != "" {
					ns, name := qualifyRef(declaration.Type, f.doc.TargetNamespace)
					if typeDoc, _, ok := schemaTypeDocument(index, ns, name); ok {
						f.targets[coverageID(typeDoc, "complexType", name)]++
					}
				}
			}
		}
	}
	if base, ok := baseFrame(index, f); ok {
		recordAttributes(index, base, start)
	}
}
func choiceSiteID(elementID string) string {
	parts := strings.Split(elementID, "|")
	if len(parts) != 4 {
		return ""
	}
	at := strings.LastIndex(parts[3], "/choice[")
	if at < 0 {
		return ""
	}
	return parts[0] + "|" + parts[1] + "|choice|" + parts[3][:at]
}
func choiceGroupAlternativeID(elementID string) string {
	parts := strings.Split(elementID, "|")
	if len(parts) != 4 {
		return ""
	}
	path := parts[3]
	at := strings.LastIndex(path, "/choice[")
	if at < 0 {
		return ""
	}
	end := strings.Index(path[at:], "]")
	if end < 0 {
		return ""
	}
	end += at + 1
	tail := path[end:]
	for _, kind := range []string{"/sequence[", "/all[", "/choice["} {
		if strings.HasPrefix(tail, kind) {
			close := strings.Index(tail, "]")
			if close >= 0 {
				return parts[0] + "|" + parts[1] + "|choiceAlternative|" + path[:end+close+1]
			}
		}
	}
	return ""
}
func choiceSiteForGroupAlternative(alternativeID string) string {
	parts := strings.Split(alternativeID, "|")
	if len(parts) != 4 {
		return ""
	}
	path := parts[3]
	cut := strings.LastIndex(path, "/sequence[")
	if cut < 0 {
		cut = strings.LastIndex(path, "/all[")
	}
	if cut < 0 {
		cut = strings.LastIndex(path, "/choice[")
	}
	if cut < 0 {
		return ""
	}
	return parts[0] + "|" + parts[1] + "|choice|" + path[:cut]
}

func choiceObservationForElement(index schemaIndex, f schemaFrame, child xml.Name) (sites, alternatives []string) {
	if f.doc == nil {
		return nil, nil
	}
	if index.ChoiceTypes != nil {
		if _, hasChoice := index.ChoiceTypes[qualifiedKey(f.doc.TargetNamespace, f.typeName)]; !hasChoice {
			if base, ok := baseFrame(index, f); ok {
				return choiceObservationForElement(index, base, child)
			}
			return nil, nil
		}
	}
	matched := false
	contains := func(elements []xsdElementDecl) bool {
		for _, element := range elements {
			_, _, ns, name := elementDeclaration(index, f.doc, element)
			if ns == child.Space && name == child.Local {
				return true
			}
		}
		return false
	}
	requiredElementsObserved := func(elements []xsdElementDecl) bool {
		for _, element := range elements {
			if element.MinOccurs == "0" {
				continue
			}
			_, _, ns, name := elementDeclaration(index, f.doc, element)
			if ns == child.Space && name == child.Local {
				continue
			}
			if !f.seen[qualifiedKey(ns, name)] {
				return false
			}
		}
		return true
	}
	var walkSequence func(string, *xsdSequence, []string, []string) bool
	var walkChoice func(string, *xsdChoice, []string, []string) bool
	var walkAll func(string, *xsdAll, []string, []string) bool
	walkSequence = func(parent string, sequence *xsdSequence, activeSites, activeAlternatives []string) bool {
		if sequence == nil {
			return false
		}
		if contains(sequence.Elements) {
			sites, alternatives, matched = activeSites, activeAlternatives, true
			return true
		}
		for i := range sequence.Sequences {
			if walkSequence(fmt.Sprintf("%s/sequence[%d]", parent, i), &sequence.Sequences[i], activeSites, activeAlternatives) {
				return true
			}
		}
		for i := range sequence.Choices {
			if walkChoice(fmt.Sprintf("%s/choice[%d]", parent, i), &sequence.Choices[i], activeSites, activeAlternatives) {
				return true
			}
		}
		for i := range sequence.Alls {
			if walkAll(fmt.Sprintf("%s/all[%d]", parent, i), &sequence.Alls[i], activeSites, activeAlternatives) {
				return true
			}
		}
		return false
	}
	walkChoice = func(parent string, choice *xsdChoice, activeSites, activeAlternatives []string) bool {
		if choice == nil {
			return false
		}
		choiceID := coverageID(f.doc, "choice", parent)
		nextSites := append(append([]string(nil), activeSites...), choiceID)
		if contains(choice.Elements) {
			sites, alternatives, matched = nextSites, activeAlternatives, true
			return true
		}
		for i := range choice.Sequences {
			if !requiredElementsObserved(choice.Sequences[i].Elements) {
				continue
			}
			path := fmt.Sprintf("%s/sequence[%d]", parent, i)
			nextAlternatives := append(append([]string(nil), activeAlternatives...), coverageID(f.doc, "choiceAlternative", path))
			if walkSequence(path, &choice.Sequences[i], nextSites, nextAlternatives) {
				return true
			}
		}
		for i := range choice.Choices {
			path := fmt.Sprintf("%s/choice[%d]", parent, i)
			nextAlternatives := append(append([]string(nil), activeAlternatives...), coverageID(f.doc, "choiceAlternative", path))
			if walkChoice(path, &choice.Choices[i], nextSites, nextAlternatives) {
				return true
			}
		}
		for i := range choice.Alls {
			if !requiredElementsObserved(choice.Alls[i].Elements) {
				continue
			}
			path := fmt.Sprintf("%s/all[%d]", parent, i)
			nextAlternatives := append(append([]string(nil), activeAlternatives...), coverageID(f.doc, "choiceAlternative", path))
			if walkAll(path, &choice.Alls[i], nextSites, nextAlternatives) {
				return true
			}
		}
		return false
	}
	walkAll = func(parent string, all *xsdAll, activeSites, activeAlternatives []string) bool {
		if all == nil {
			return false
		}
		if contains(all.Elements) {
			sites, alternatives, matched = activeSites, activeAlternatives, true
			return true
		}
		for i := range all.Sequences {
			if walkSequence(fmt.Sprintf("%s/sequence[%d]", parent, i), &all.Sequences[i], activeSites, activeAlternatives) {
				return true
			}
		}
		for i := range all.Choices {
			if walkChoice(fmt.Sprintf("%s/choice[%d]", parent, i), &all.Choices[i], activeSites, activeAlternatives) {
				return true
			}
		}
		for i := range all.Alls {
			if walkAll(fmt.Sprintf("%s/all[%d]", parent, i), &all.Alls[i], activeSites, activeAlternatives) {
				return true
			}
		}
		return false
	}
	for _, particles := range typeParticleSets(f.typ) {
		if walkSequence(f.typeName, particles.sequence, nil, nil) || walkChoice(f.typeName, particles.choice, nil, nil) || walkAll(f.typeName, particles.all, nil, nil) {
			return sites, alternatives
		}
	}
	if !matched {
		if base, ok := baseFrame(index, f); ok {
			return choiceObservationForElement(index, base, child)
		}
	}
	return sites, alternatives
}

func choiceSiteForWildcard(wildcardID string) string {
	parts := strings.Split(wildcardID, "|")
	if len(parts) != 4 || !strings.HasSuffix(parts[3], "/choice") {
		return ""
	}
	return parts[0] + "|" + parts[1] + "|choice|" + strings.TrimSuffix(parts[3], "/choice")
}

func wildcardAllows(any *xsdAny, ownerNS, childNS string) bool {
	if any == nil {
		return false
	}
	switch any.Namespace {
	case "", "##any":
		return true
	case "##other":
		return childNS != ownerNS
	case "##targetNamespace":
		return childNS == ownerNS
	}
	for _, ns := range strings.Fields(any.Namespace) {
		if ns == childNS {
			return true
		}
	}
	return false
}
func sequencePrefixPresent(index schemaIndex, f schemaFrame, sequence *xsdSequence) bool {
	if sequence == nil {
		return false
	}
	for _, element := range sequence.Elements {
		if element.MinOccurs == "0" {
			continue
		}
		_, _, ns, name := elementDeclaration(index, f.doc, element)
		if !f.seen[qualifiedKey(ns, name)] {
			return false
		}
	}
	return true
}
func wildcardSite(index schemaIndex, f schemaFrame, child xml.Name) string {
	candidates := []string{}
	var sequence func(string, *xsdSequence) string
	var choice func(string, *xsdChoice) string
	var all func(string, *xsdAll) string
	sequence = func(parent string, p *xsdSequence) string {
		if p == nil {
			return ""
		}
		if wildcardAllows(p.Any, f.doc.TargetNamespace, child.Space) && sequencePrefixPresent(index, f, p) {
			candidates = append(candidates, coverageID(f.doc, "any", parent+"/sequence"))
		}
		for i := range p.Sequences {
			sequence(fmt.Sprintf("%s/sequence[%d]", parent, i), &p.Sequences[i])
		}
		for i := range p.Choices {
			choice(fmt.Sprintf("%s/choice[%d]", parent, i), &p.Choices[i])
		}
		for i := range p.Alls {
			all(fmt.Sprintf("%s/all[%d]", parent, i), &p.Alls[i])
		}
		return ""
	}
	choice = func(parent string, p *xsdChoice) string {
		if p == nil {
			return ""
		}
		if wildcardAllows(p.Any, f.doc.TargetNamespace, child.Space) {
			candidates = append(candidates, coverageID(f.doc, "any", parent+"/choice"))
		}
		for i := range p.Sequences {
			sequence(fmt.Sprintf("%s/sequence[%d]", parent, i), &p.Sequences[i])
		}
		for i := range p.Choices {
			choice(fmt.Sprintf("%s/choice[%d]", parent, i), &p.Choices[i])
		}
		for i := range p.Alls {
			all(fmt.Sprintf("%s/all[%d]", parent, i), &p.Alls[i])
		}
		return ""
	}
	all = func(parent string, p *xsdAll) string {
		if p == nil {
			return ""
		}
		if wildcardAllows(p.Any, f.doc.TargetNamespace, child.Space) {
			candidates = append(candidates, coverageID(f.doc, "any", parent+"/all"))
		}
		for i := range p.Sequences {
			sequence(fmt.Sprintf("%s/sequence[%d]", parent, i), &p.Sequences[i])
		}
		for i := range p.Choices {
			choice(fmt.Sprintf("%s/choice[%d]", parent, i), &p.Choices[i])
		}
		for i := range p.Alls {
			all(fmt.Sprintf("%s/all[%d]", parent, i), &p.Alls[i])
		}
		return ""
	}
	for _, p := range typeParticleSets(f.typ) {
		sequence(f.typeName, p.sequence)
		choice(f.typeName, p.choice)
		all(f.typeName, p.all)
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	if base, ok := baseFrame(index, f); ok {
		return wildcardSite(index, base, child)
	}
	return ""
}
func matchFixture(index schemaIndex, path string) (map[string]int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	match, err := matchFixtureDetailedReader(index, file)
	return match.targets, err
}

// matchFixtureBytes lets corpus generation record the targets of a fixture
// before it is written. The same matcher powers the independent coverage
// analyzer, keeping fixture metadata and observed coverage in one namespace.
func matchFixtureBytes(index schemaIndex, data []byte) (map[string]int, error) {
	match, err := matchFixtureDetailedReader(index, bytes.NewReader(data))
	return match.targets, err
}

func matchFixtureReader(index schemaIndex, reader io.Reader) (map[string]int, error) {
	match, err := matchFixtureDetailedReader(index, reader)
	return match.targets, err
}

func matchFixtureDetailed(index schemaIndex, path string) (fixtureMatch, error) {
	file, err := os.Open(path)
	if err != nil {
		return fixtureMatch{}, err
	}
	defer file.Close()
	return matchFixtureDetailedReader(index, file)
}

func matchFixtureDetailedReader(index schemaIndex, reader io.Reader) (fixtureMatch, error) {
	match := fixtureMatch{targets: map[string]int{}, cardinalities: map[string][]int{}, parentInstances: map[string]int{}}
	out := match.targets
	// Recursive coverage targets are schema edges that participate in a type
	// cycle. Observing the direct edge in a valid fixture exercises that target;
	// the edge need not close a cycle in this particular XML path.
	recursiveEdges := index.RecursiveEdges
	dec := xml.NewDecoder(reader)
	var stack []schemaFrame
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fixtureMatch{}, err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			if len(stack) == 0 {
				root, ok := index.GlobalElements[qualifiedKey(v.Name.Space, v.Name.Local)]
				if !ok {
					typeKey := qualifiedKey(v.Name.Space, v.Name.Local)
					if typ, exists := index.GlobalTypes[typeKey]; exists {
						doc := index.GlobalTypeDocs[typeKey]
						if doc == nil {
							return fixtureMatch{}, fmt.Errorf("schema document for type root %s is missing", v.Name)
						}
						frame := schemaFrame{doc: doc, typ: typ, typeName: v.Name.Local, targets: out, siblings: map[string]int{}, seen: map[string]bool{}}
						out[complexTypeIDForElement(doc, "", v.Name.Local)]++
						stack = append(stack, frame)
						recordAttributes(index, frame, v)
						recordInheritance(index, frame)
						continue
					}
					return fixtureMatch{}, fmt.Errorf("unknown root %s", v.Name)
				}
				d := index.GlobalElementDocs[qualifiedKey(v.Name.Space, v.Name.Local)]
				if d == nil {
					d = index.ByTargetNS[v.Name.Space]
				}
				if d == nil {
					return fixtureMatch{}, fmt.Errorf("schema document for root %s is missing", v.Name)
				}
				rootID := fmt.Sprintf("%s|%s|element|global/%s", d.TargetNamespace, d.File, v.Name.Local)
				id := fmt.Sprintf("%s|%s|element|global/%s/%s", d.TargetNamespace, d.File, v.Name.Local, v.Name.Local)
				out[rootID]++
				out[id]++
				td, t, n, ok := declaredType(index, d, root)
				if !ok {
					stack = append(stack, schemaFrame{})
					continue
				}
				out[complexTypeIDForElement(td, id, n)]++
				stack = append(stack, schemaFrame{doc: td, typ: t, typeName: n, targets: out, siblings: map[string]int{}, seen: map[string]bool{}})
				recordAttributes(index, stack[len(stack)-1], v)
				recordInheritance(index, stack[len(stack)-1])
				continue
			}
			parent := stack[len(stack)-1]
			decl, id, substitutionID, ok := particleElement(index, parent, v.Name)
			if !ok {
				if wildcard := wildcardSite(index, parent, v.Name); wildcard != "" {
					out[wildcard]++
					if choiceID := choiceSiteForWildcard(wildcard); choiceID != "" {
						out[choiceID]++
					}
					// A lax wildcard may contain a declaration from an imported
					// schema. Continue matching through it when we know that
					// declaration; otherwise it remains an intentionally opaque
					// private extension payload.
					if global, exists := index.GlobalElements[qualifiedKey(v.Name.Space, v.Name.Local)]; exists {
						doc := index.GlobalElementDocs[qualifiedKey(v.Name.Space, v.Name.Local)]
						if doc == nil {
							doc = index.ByTargetNS[v.Name.Space]
						}
						if doc != nil {
							td, typ, typeName, known := declaredType(index, doc, global)
							if known {
								globalID := fmt.Sprintf("%s|%s|element|global/%s/%s", doc.TargetNamespace, doc.File, v.Name.Local, v.Name.Local)
								out[globalID]++
								out[complexTypeIDForElement(td, globalID, typeName)]++
								frame := schemaFrame{doc: td, typ: typ, typeName: typeName, targets: out, siblings: map[string]int{}, seen: map[string]bool{}}
								recordAttributes(index, frame, v)
								recordInheritance(index, frame)
								stack = append(stack, frame)
								continue
							}
						}
					}
				}
				stack = append(stack, schemaFrame{})
				continue
			}
			parent.siblings[id]++
			parent.seen[qualifiedKey(v.Name.Space, v.Name.Local)] = true
			choiceSites, choiceAlternatives := choiceObservationForElement(index, parent, v.Name)
			for _, choiceID := range choiceSites {
				out[choiceID]++
			}
			for _, alternativeID := range choiceAlternatives {
				out[alternativeID]++
			}
			if parent.siblings[id] > out[id] {
				out[id] = parent.siblings[id]
			}
			if choiceID := choiceSiteID(id); choiceID != "" {
				out[choiceID]++
			}
			if alternativeID := choiceGroupAlternativeID(id); alternativeID != "" {
				out[alternativeID]++
				if siteID := choiceSiteForGroupAlternative(alternativeID); siteID != "" {
					out[siteID]++
				}
			}
			if substitutionID != "" {
				out[substitutionID]++
			}
			recordGlobalElementUse(index, out, parent.doc, decl, v.Name, substitutionID)
			declarationDoc := parent.doc
			if substitutionID != "" && index.ByTargetNS[v.Name.Space] != nil {
				declarationDoc = index.ByTargetNS[v.Name.Space]
			}
			td, t, n, ok := declaredType(index, declarationDoc, decl)
			if ok {
				if parent.doc != nil && parent.typeName != "" {
					recursiveID := fmt.Sprintf("%s|%s|recursive|%s->%s", parent.doc.TargetNamespace, parent.doc.File, parent.typeName, qualifiedKey(td.TargetNamespace, n))
					if _, isRecursiveEdge := recursiveEdges[recursiveID]; isRecursiveEdge {
						out[recursiveID]++
					}
				}
				out[complexTypeIDForElement(td, id, n)]++
				depth := 1
				for _, ancestor := range stack {
					if ancestor.doc != nil && ancestor.doc.TargetNamespace == td.TargetNamespace && ancestor.typeName == n {
						depth++
					}
				}
				if depth > 1 {
					recursiveID := fmt.Sprintf("%s|%s|recursive|%s->%s", parent.doc.TargetNamespace, parent.doc.File, parent.typeName, qualifiedKey(td.TargetNamespace, n))
					if depth > out[recursiveID] {
						out[recursiveID] = depth
					}
				}
				stack = append(stack, schemaFrame{doc: td, typ: t, typeName: n, targets: out, siblings: map[string]int{}, seen: map[string]bool{}})
				recordAttributes(index, stack[len(stack)-1], v)
				recordInheritance(index, stack[len(stack)-1])
			} else {
				stack = append(stack, schemaFrame{})
			}
		case xml.EndElement:
			if len(stack) > 0 {
				frame := stack[len(stack)-1]
				recordParentInstance(index, &match, frame)
				for id, count := range frame.siblings {
					match.cardinalities[id] = append(match.cardinalities[id], count)
				}
				stack = stack[:len(stack)-1]
			}
		}
	}
	return match, nil
}

func recordParentInstance(index schemaIndex, match *fixtureMatch, frame schemaFrame) {
	for {
		if frame.doc == nil || frame.typeName == "" {
			return
		}
		match.parentInstances[complexTypeIDForElement(frame.doc, "", frame.typeName)]++
		base, ok := baseFrame(index, frame)
		if !ok {
			return
		}
		frame = base
	}
}
