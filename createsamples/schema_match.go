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
	if f.typ.ComplexContent == nil {
		return schemaFrame{}, false
	}
	var base string
	if f.typ.ComplexContent.Extension != nil {
		base = f.typ.ComplexContent.Extension.Base
	}
	if f.typ.ComplexContent.Restriction != nil {
		base = f.typ.ComplexContent.Restriction.Base
	}
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
	return schemaFrame{doc: doc, typ: typ, typeName: name, targets: f.targets}, true
}
func recordInheritance(index schemaIndex, f schemaFrame) {
	if f.typ.ComplexContent == nil {
		return
	}
	base := ""
	if f.typ.ComplexContent.Extension != nil {
		base = f.typ.ComplexContent.Extension.Base
	}
	if f.typ.ComplexContent.Restriction != nil {
		base = f.typ.ComplexContent.Restriction.Base
	}
	if base == "" {
		return
	}
	ns, name := qualifyRef(base, f.doc.TargetNamespace)
	f.targets[fmt.Sprintf("%s|%s|inheritance|%s->%s#%s", f.doc.TargetNamespace, f.doc.File, f.typeName, ns, name)]++
}
func particleElement(index schemaIndex, f schemaFrame, q xml.Name) (xsdElementDecl, string, bool) {
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
		for _, declaration := range append(append([]xsdAttribute{}, f.typ.Attributes...), extensionAttributes(f.typ)...) {
			if declaration.Name == attribute.Name.Local {
				f.targets[fmt.Sprintf("%s|%s|attribute|%s/@%s", f.doc.TargetNamespace, f.doc.File, f.typeName, declaration.Name)]++
			}
		}
	}
	if base, ok := baseFrame(index, f); ok {
		recordAttributes(index, base, start)
	}
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
				out[fmt.Sprintf("%s|%s|complexType|%s", td.TargetNamespace, td.File, n)]++
				stack = append(stack, schemaFrame{td, t, n, out})
				recordAttributes(index, stack[len(stack)-1], v)
				recordInheritance(index, stack[len(stack)-1])
				continue
			}
			parent := stack[len(stack)-1]
			decl, id, ok := particleElement(index, parent, v.Name)
			if !ok {
				if parent.typ.Sequence != nil && parent.typ.Sequence.Any != nil {
					out[fmt.Sprintf("%s|%s|any|%s/sequence", parent.doc.TargetNamespace, parent.doc.File, parent.typeName)]++
				}
				stack = append(stack, schemaFrame{})
				continue
			}
			out[id]++
			if strings.Contains(id, "/choice[") {
				out[fmt.Sprintf("%s|%s|choice|%s", parent.doc.TargetNamespace, parent.doc.File, parent.typeName)]++
			}
			td, t, n, ok := declaredType(index, parent.doc, decl)
			if ok {
				out[fmt.Sprintf("%s|%s|complexType|%s", td.TargetNamespace, td.File, n)]++
				stack = append(stack, schemaFrame{td, t, n, out})
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
