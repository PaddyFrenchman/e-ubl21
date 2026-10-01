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
		if d := index.ByTargetNS[ns]; d != nil {
			if x, ok := d.Elements[n]; ok {
				return d, x, ns, n
			}
		}
		return doc, decl, ns, n
	}
	return doc, decl, doc.TargetNamespace, decl.Name
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
	td := index.ByTargetNS[ns]
	if td == nil {
		return nil, xsdComplexType{}, "", false
	}
	t, ok := td.Types[n]
	return td, t, n, ok
}
func baseFrame(index schemaIndex, f schemaFrame) (schemaFrame, bool) {
	base := typeBase(f.typ)
	if base == "" {
		return schemaFrame{}, false
	}
	ns, name := qualifyRef(base, f.doc.TargetNamespace)
	doc := index.ByTargetNS[ns]
	if doc == nil {
		return schemaFrame{}, false
	}
	typ, ok := doc.Types[name]
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
		if x, id, ok := findInAll(index, f, parent+fmt.Sprintf("/all[%d]", i), &choice.Alls[i], q); ok {
			return x, id, true
		}
	}
	return xsdElementDecl{}, "", false
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
	base := typeBase(f.typ)
	if base == "" {
		return
	}
	ns, name := qualifyRef(base, f.doc.TargetNamespace)
	f.targets[fmt.Sprintf("%s|%s|inheritance|%s->%s#%s", f.doc.TargetNamespace, f.doc.File, f.typeName, ns, name)]++
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
					return fixtureMatch{}, fmt.Errorf("unknown root %s", v.Name)
				}
				d := index.ByTargetNS[v.Name.Space]
				td, t, n, ok := declaredType(index, d, root)
				if !ok {
					stack = append(stack, schemaFrame{})
					continue
				}
				id := fmt.Sprintf("%s|%s|element|global/%s", d.TargetNamespace, d.File, v.Name.Local)
				out[id]++
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
				}
				stack = append(stack, schemaFrame{})
				continue
			}
			parent.siblings[id]++
			parent.seen[qualifiedKey(v.Name.Space, v.Name.Local)] = true
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
			declarationDoc := parent.doc
			if substitutionID != "" && index.ByTargetNS[v.Name.Space] != nil {
				declarationDoc = index.ByTargetNS[v.Name.Space]
			}
			td, t, n, ok := declaredType(index, declarationDoc, decl)
			if ok {
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
