// This file contains small string helpers for splitting/normalizing
// prefixed QNames and UBL schema file names, shared by the schema index,
// generator and inventory code.
package main

import "strings"

func localPart(q string) string {
	if q == "" {
		return ""
	}
	parts := strings.SplitN(q, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return q
}

func prefixPart(q string) string {
	parts := strings.SplitN(q, ":", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}

func trimUBLDocumentType(name string) string {
	name = strings.TrimPrefix(name, "UBL-")
	name = strings.TrimSuffix(name, "-2.1")
	return strings.TrimSuffix(name, ".xsd")
}
