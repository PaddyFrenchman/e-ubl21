// Package xsdschemas embeds the authoritative OASIS UBL 2.1 XSD schema tree
// (common/ and maindoc/) into the compiled binary, so consumers (such as the
// HTTP server's upload validator) can perform real XSD validation without
// depending on this directory being present on disk at the deployment
// location.
package xsdschemas

import "embed"

// FS holds every UBL 2.1 XSD file under common/ and maindoc/, rooted at this
// package's directory (i.e. FS.Open("maindoc/UBL-Invoice-2.1.xsd") works).
//
//go:embed common/*.xsd maindoc/*.xsd
var FS embed.FS
