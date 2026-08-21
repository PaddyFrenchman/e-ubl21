// This file serializes the generated xmlNode tree (see generate.go) to
// indented, namespace-prefixed XML text.
package main

import (
	"fmt"
	"sort"
	"strings"
)

func renderXMLNode(node xmlNode, indent int) string {
	prefix := ""
	if node.Prefix != "" {
		prefix = node.Prefix + ":"
	}
	attrText := ""
	if len(node.Attributes) > 0 {
		keys := make([]string, 0, len(node.Attributes))
		for k := range node.Attributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			attrText += fmt.Sprintf(" %s=\"%s\"", k, escapeText(node.Attributes[k]))
		}
	}
	if len(node.Children) == 0 {
		return indentString(indent) + "<" + prefix + node.Name + attrText + ">" + escapeText(node.Value) + "</" + prefix + node.Name + ">\n"
	}
	buf := indentString(indent) + "<" + prefix + node.Name + attrText + ">\n"
	for _, child := range node.Children {
		buf += renderXMLNode(child, indent+2)
	}
	buf += indentString(indent) + "</" + prefix + node.Name + ">\n"
	return buf
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
