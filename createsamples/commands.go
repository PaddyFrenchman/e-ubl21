// This file implements the three CLI subcommands (inventory, generate,
// validate) described in the package doc comment in main.go.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runInventory(args []string) error {
	flags := flag.NewFlagSet("inventory", flag.ContinueOnError)
	xsdDir := flags.String("xsd", filepath.Join("XSD"), "path to the UBL XSD tree")
	output := flags.String("output", "", "optional path for JSON report output")
	if err := flags.Parse(args); err != nil {
		return err
	}

	report, err := discoverInventory(*xsdDir)
	if err != nil {
		return err
	}

	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal inventory: %w", err)
	}

	fmt.Println(string(encoded))
	if *output != "" {
		if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
			return fmt.Errorf("create report dir: %w", err)
		}
		if err := os.WriteFile(*output, encoded, 0o644); err != nil {
			return fmt.Errorf("write report: %w", err)
		}
	}
	return nil
}

func runGenerate(args []string) error {
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	xsdDir := flags.String("xsd", filepath.Join("XSD"), "path to the UBL XSD tree")
	output := flags.String("output", "", "output directory for generated samples (default depends on profile)")
	profile := flags.String("profile", "ubl21", "validation profile to generate (ubl21, ubl21-structural, en16931, or peppol-bis-billing)")
	set := flags.String("set", "positive", "fixture set to generate (positive, negative, or all)")
	generatorVersion := flags.String("generator-version", "", "immutable generator version as git:<40-or-64-lowercase-hex-commit> (default: latest generator implementation commit)")
	count := flags.Int("count", 1, "number of positive sample files to generate per main document schema")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *count <= 0 {
		return errors.New("count must be greater than zero")
	}
	if *profile != "ubl21" && *profile != "ubl21-structural" && *profile != "en16931" && *profile != "peppol-bis-billing" {
		return fmt.Errorf("unsupported generation profile %q; supported profiles are ubl21, ubl21-structural, en16931, and peppol-bis-billing", *profile)
	}
	if *output == "" {
		switch *profile {
		case "ubl21":
			*output = filepath.Join("createsamples", "samples")
		case "ubl21-structural":
			*output = filepath.Join("createsamples", "samples-structural")
		case "en16931":
			*output = filepath.Join("createsamples", "samples-en16931")
		case "peppol-bis-billing":
			*output = filepath.Join("createsamples", "samples-peppol")
		}
	}
	if *set != "positive" && *set != "negative" && *set != "all" {
		return fmt.Errorf("unsupported generation set %q; supported sets are positive, negative, and all", *set)
	}
	if err := validateCorpusOutputDirectory(*output); err != nil {
		return err
	}

	report, err := discoverInventory(*xsdDir)
	if err != nil {
		return err
	}
	if len(report.MainFiles) == 0 {
		return errors.New("no main document XSD files discovered")
	}
	version, err := resolveGeneratorVersion(*generatorVersion)
	if err != nil {
		return err
	}
	generate := func() (generatedCorpus, error) {
		if *profile == "peppol-bis-billing" {
			if *count != 1 {
				return generatedCorpus{}, errors.New("count is only supported for profile ubl21")
			}
			switch *set {
			case "positive":
				return generatePeppolPositiveCorpus(*xsdDir, version)
			case "negative":
				return generatePeppolNegativeCorpus(*xsdDir, version)
			default:
				positive, err := generatePeppolPositiveCorpus(*xsdDir, version)
				if err != nil {
					return generatedCorpus{}, err
				}
				negative, err := generatePeppolNegativeCorpus(*xsdDir, version)
				if err != nil {
					return generatedCorpus{}, err
				}
				return combineGeneratedCorpora(positive, negative)
			}
		}
		if *profile == "en16931" {
			if *count != 1 {
				return generatedCorpus{}, errors.New("count is only supported for profile ubl21")
			}
			switch *set {
			case "positive":
				return generateEN16931PositiveCorpus(*xsdDir, version)
			case "negative":
				return generateEN16931NegativeCorpus(*xsdDir, version)
			default:
				positive, err := generateEN16931PositiveCorpus(*xsdDir, version)
				if err != nil {
					return generatedCorpus{}, err
				}
				negative, err := generateEN16931NegativeCorpus(*xsdDir, version)
				if err != nil {
					return generatedCorpus{}, err
				}
				return combineGeneratedCorpora(positive, negative)
			}
		}
		index, err := buildSchemaIndex(*xsdDir)
		if err != nil {
			return generatedCorpus{}, err
		}
		if *profile == "ubl21-structural" {
			if *count != 1 || *set != "positive" {
				return generatedCorpus{}, errors.New("ubl21-structural supports only --set positive and --count 1")
			}
			return generateUBLStructuralCorpus(report, index, *xsdDir, version)
		}
		switch *set {
		case "positive":
			return generateUBLPositiveCorpus(report, index, *xsdDir, *count, version)
		case "negative":
			return generateUBLNegativeCorpus(*xsdDir, version)
		default:
			positive, err := generateUBLPositiveCorpus(report, index, *xsdDir, *count, version)
			if err != nil {
				return generatedCorpus{}, err
			}
			negative, err := generateUBLNegativeCorpus(*xsdDir, version)
			if err != nil {
				return generatedCorpus{}, err
			}
			return combineGeneratedCorpora(positive, negative)
		}
	}
	first, err := generate()
	if err != nil {
		return err
	}
	second, err := generate()
	if err != nil {
		return err
	}
	if err := verifyCorpusDeterminism(first, second); err != nil {
		return err
	}
	xmllintPath, err := exec.LookPath("xmllint")
	if err != nil {
		return errors.New("xmllint is required for independent generated-corpus validation; install libxml2 and retry")
	}
	if err := validateGeneratedCorpus(xmllintPath, first); err != nil {
		return err
	}
	if err := writeGeneratedCorpus(*output, first); err != nil {
		return err
	}
	fmt.Printf("Generated and independently validated %d deterministic samples under %s\n", len(first.fixtures), *output)
	return nil
}

func runValidate(args []string) error {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	xsdDir := flags.String("xsd", filepath.Join("XSD"), "path to the UBL XSD tree")
	samplesDir := flags.String("samples", filepath.Join("createsamples", "samples"), "directory with generated XML samples")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if _, err := exec.LookPath("xmllint"); err != nil {
		return errors.New("xmllint is required for validation; install libxml2 and retry")
	}

	report, err := discoverInventory(*xsdDir)
	if err != nil {
		return err
	}

	mainByName := map[string]string{}
	for _, f := range report.MainFiles {
		root := f.RootElement
		if root == "" {
			root = strings.TrimSuffix(filepath.Base(f.File), filepath.Ext(f.File))
		}
		mainByName[root] = filepath.Join(*xsdDir, "maindoc", filepath.Base(f.File))
	}

	var generated []string
	if err := filepath.WalkDir(*samplesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".xml" {
			generated = append(generated, path)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("walk samples: %w", err)
	}
	if len(generated) == 0 {
		return fmt.Errorf("no XML samples found in %s; run generate first", *samplesDir)
	}

	for _, samplePath := range generated {
		rootName := filepath.Base(filepath.Dir(samplePath))
		schemaPath, ok := mainByName[rootName]
		if !ok {
			fmt.Printf("SKIP %s (root %s not found in schema inventory)\n", samplePath, rootName)
			continue
		}
		cmd := exec.Command("xmllint", "--noout", "--schema", schemaPath, samplePath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("FAIL %s\n%s\n", samplePath, strings.TrimSpace(string(output)))
			continue
		}
		fmt.Printf("PASS %s\n", samplePath)
		_ = output
	}
	fmt.Printf("Validated %d XML samples against %d main schemas\n", len(generated), len(report.MainFiles))
	return nil
}
