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
//   - qname.go      - small qualified-name string helpers shared by the
//     other files.
//
// XML tree serialization lives in the sibling xmlnode package
// (createsamples/xmlnode), not in this package, so that the EN 16931
// semantic generator under createsamples/en16931 can reuse the same node
// type and renderer (see ideas/EN16931-samples-generator-plan.md §4).
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return fmt.Errorf("a command is required")
	}

	switch args[0] {
	case "inventory":
		if err := runInventory(args[1:]); err != nil {
			return fmt.Errorf("inventory error: %w", err)
		}
	case "generate":
		if err := runGenerate(args[1:]); err != nil {
			return fmt.Errorf("generate error: %w", err)
		}
	case "validate":
		if err := runValidate(args[1:]); err != nil {
			return fmt.Errorf("validate error: %w", err)
		}
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
	return nil
}

func printUsage() {
	fmt.Println("Usage: createsamples <inventory|generate|validate> [flags]")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  go run ./createsamples inventory --xsd XSD")
	fmt.Println("  go run ./createsamples generate --profile ubl21 --set positive --xsd XSD --output createsamples/samples")
	fmt.Println("  go run ./createsamples generate --profile en16931 --set all --xsd XSD --output createsamples/samples-en16931")
	fmt.Println("  go run ./createsamples validate --xsd XSD --samples createsamples/samples")
}
