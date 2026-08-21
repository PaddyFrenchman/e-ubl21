// Command createsamples generates and validates sample UBL 2.1 XML documents
// for every main document schema shipped under XSD/maindoc. See
// .github/create-samples.md for the full design and roadmap.
//
// The implementation is split across several files, all in this same
// package, grouped by responsibility:
//
//   - main.go       - CLI entry point and usage text.
//   - commands.go   - the inventory/generate/validate subcommands.
//   - inventory.go  - schema inventory discovery (file listing, root
//     element/import extraction).
//   - xsdmodel.go   - Go structs mirroring the subset of the XSD schema
//     language this tool understands.
//   - schemaindex.go - namespace-qualified schema index used to resolve
//     element/type references across the whole XSD tree.
//   - generate.go   - the minimal-mode sample document generator.
//   - values.go     - deterministic sample value/enumeration generation for
//     XSD simple types.
//   - render.go     - XML serialization of the generated document tree.
//   - qname.go      - small qualified-name string helpers shared by the
//     other files.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "inventory":
		if err := runInventory(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "inventory error:", err)
			os.Exit(1)
		}
	case "generate":
		if err := runGenerate(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "generate error:", err)
			os.Exit(1)
		}
	case "validate":
		if err := runValidate(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "validate error:", err)
			os.Exit(1)
		}
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: createsamples <inventory|generate|validate> [flags]")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  go run ./createsamples inventory --xsd XSD")
	fmt.Println("  go run ./createsamples generate --xsd XSD --output createsamples/samples --count 10")
	fmt.Println("  go run ./createsamples validate --xsd XSD --samples createsamples/samples")
}
