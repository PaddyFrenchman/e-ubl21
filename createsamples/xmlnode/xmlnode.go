// Package xmlnode provides a minimal in-memory XML element tree and
// indented-XML renderer, shared by the structural UBL sample generator
// (createsamples, package main) and the EN 16931 semantic generator
// (createsamples/en16931). It was extracted from the structural
// generator's generate.go/render.go so both generators can build a tree of
// the same shape and serialize it identically - Go does not allow
// importing a "package main" directory from elsewhere, so this shared
// logic must live in its own importable package. See
// ideas/EN16931-samples-generator-plan.md §4/§10 (Phase EN-2, step 10).
package xmlnode

import (
	"sort"
	"strings"
)

// Node is one element in the tree: an optional namespace Prefix, an
// element Name, either a leaf Value or nested Children (not both), and any
// XML attributes.
type Node struct {
	Prefix     string
	Name       string
	Value      string
	Attributes map[string]string
	Children   []Node
}

// Render serializes node to indented XML text, starting at the given
// indent level (in 2-space units). Attribute keys are emitted in sorted
// order for deterministic output.
func Render(node Node, indent int) string {
	var output strings.Builder
	renderTo(&output, node, indent)
	return output.String()
}

func renderTo(output *strings.Builder, node Node, indent int) {
	prefix := ""
	if node.Prefix != "" {
		prefix = node.Prefix + ":"
	}
	output.WriteString(indentString(indent))
	output.WriteByte('<')
	output.WriteString(prefix)
	output.WriteString(node.Name)
	if len(node.Attributes) > 0 {
		keys := make([]string, 0, len(node.Attributes))
		for k := range node.Attributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			output.WriteByte(' ')
			output.WriteString(k)
			output.WriteString("=\"")
			output.WriteString(escapeText(node.Attributes[k]))
			output.WriteByte('"')
		}
	}
	if len(node.Children) == 0 {
		output.WriteByte('>')
		output.WriteString(escapeText(node.Value))
		output.WriteString("</")
		output.WriteString(prefix)
		output.WriteString(node.Name)
		output.WriteString(">\n")
		return
	}
	output.WriteString(">\n")
	for _, child := range node.Children {
		renderTo(output, child, indent+2)
	}
	output.WriteString(indentString(indent))
	output.WriteString("</")
	output.WriteString(prefix)
	output.WriteString(node.Name)
	output.WriteString(">\n")
}

func indentString(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("  ", n)
}

func escapeText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
