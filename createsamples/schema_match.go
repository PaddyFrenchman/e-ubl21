package main

import (
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
}

func complexTypeIDForElement(elementID, typeName string) string {
	if typeName == "inline" {
		return strings.Replace(elementID, "|element|", "|complexType|inline/", 1)
	}
	parts := strings.Split(elementID, "|")
	if len(parts) != 4 {
		return ""
	}
	return fmt.Sprintf("%s|%s|complexType|%s", parts[0], parts[1], typeName)
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
	return schemaFrame{doc: doc, typ: typ, typeName: name, targets: f.targets, siblings: map[string]int{}}, true
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
func particleElement(index schemaIndex, f schemaFrame, q xml.Name) (xsdElementDecl, string, bool) {
	if x, id, ok := findInSequence(index, f, f.typeName, f.typ.Sequence, q); ok {
		return x, id, true
	}
	if x, id, ok := findInChoice(index, f, f.typeName, f.typ.Choice, q); ok {
		return x, id, true
	}
	if x, id, ok := findInAll(index, f, f.typeName, f.typ.All, q); ok {
		return x, id, true
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
	if f.typ.Sequence != nil {
		if x, id, ok := check("sequence", f.typ.Sequence.Elements); ok {
			return x, id, true
		}
	}
	if f.typ.All != nil {
		if x, id, ok := check("all", f.typ.All.Elements); ok {
			return x, id, true
		}
	}
	if f.typ.Choice != nil {
		if x, id, ok := check("choice", f.typ.Choice.Elements); ok {
			return x, id, true
		}
	}
	if base, ok := baseFrame(index, f); ok {
		return particleElement(index, base, q)
	}
	return xsdElementDecl{}, "", false
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
func wildcardSite(index schemaIndex, f schemaFrame) string {
	if f.typ.Sequence != nil && f.typ.Sequence.Any != nil {
		return fmt.Sprintf("%s|%s|any|%s/sequence", f.doc.TargetNamespace, f.doc.File, f.typeName)
	}
	if f.typ.Choice != nil && f.typ.Choice.Any != nil {
		return fmt.Sprintf("%s|%s|any|%s/choice", f.doc.TargetNamespace, f.doc.File, f.typeName)
	}
	if f.typ.All != nil && f.typ.All.Any != nil {
		return fmt.Sprintf("%s|%s|any|%s/all", f.doc.TargetNamespace, f.doc.File, f.typeName)
	}
	if base, ok := baseFrame(index, f); ok {
		return wildcardSite(index, base)
	}
	return ""
}
func matchFixture(index schemaIndex, path string) (map[string]int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	out := map[string]int{}
	dec := xml.NewDecoder(file)
	var stack []schemaFrame
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			if len(stack) == 0 {
				root, ok := index.GlobalElements[qualifiedKey(v.Name.Space, v.Name.Local)]
				if !ok {
					return nil, fmt.Errorf("unknown root %s", v.Name)
				}
				d := index.ByTargetNS[v.Name.Space]
				td, t, n, ok := declaredType(index, d, root)
				if !ok {
					stack = append(stack, schemaFrame{})
					continue
				}
				id := fmt.Sprintf("%s|%s|element|global/%s", d.TargetNamespace, d.File, v.Name.Local)
				out[id]++
				out[complexTypeIDForElement(id, n)]++
				stack = append(stack, schemaFrame{doc: td, typ: t, typeName: n, targets: out, siblings: map[string]int{}})
				recordAttributes(index, stack[len(stack)-1], v)
				recordInheritance(index, stack[len(stack)-1])
				continue
			}
			parent := stack[len(stack)-1]
			decl, id, ok := particleElement(index, parent, v.Name)
			if !ok {
				if wildcard := wildcardSite(index, parent); wildcard != "" {
					out[wildcard]++
				}
				stack = append(stack, schemaFrame{})
				continue
			}
			parent.siblings[id]++
			if parent.siblings[id] > out[id] {
				out[id] = parent.siblings[id]
			}
			if choiceID := choiceSiteID(id); choiceID != "" {
				out[choiceID]++
			}
			td, t, n, ok := declaredType(index, parent.doc, decl)
			if ok {
				out[complexTypeIDForElement(id, n)]++
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
				stack = append(stack, schemaFrame{doc: td, typ: t, typeName: n, targets: out, siblings: map[string]int{}})
				recordAttributes(index, stack[len(stack)-1], v)
				recordInheritance(index, stack[len(stack)-1])
			} else {
				stack = append(stack, schemaFrame{})
			}
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return out, nil
}
