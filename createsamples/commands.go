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
	output := flags.String("output", filepath.Join("createsamples", "samples"), "output directory for generated samples")
	profile := flags.String("profile", "ubl21", "validation profile to generate (currently ubl21)")
	set := flags.String("set", "positive", "fixture set to generate (currently positive)")
	generatorVersion := flags.String("generator-version", "", "immutable generator version as git:<40-or-64-lowercase-hex-commit> (default: current HEAD)")
	count := flags.Int("count", 1, "number of sample files to generate per main document schema")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *count <= 0 {
		return errors.New("count must be greater than zero")
	}
	if *profile != "ubl21" {
		return fmt.Errorf("unsupported generation profile %q; issue #33 supports ubl21", *profile)
	}
	if *set != "positive" {
		return fmt.Errorf("unsupported generation set %q; issue #33 supports positive", *set)
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
	index, err := buildSchemaIndex(*xsdDir)
	if err != nil {
		return err
	}
	version, err := resolveGeneratorVersion(*generatorVersion)
	if err != nil {
		return err
	}
	first, err := generateUBLPositiveCorpus(report, index, *xsdDir, *count, version)
	if err != nil {
		return err
	}
	second, err := generateUBLPositiveCorpus(report, index, *xsdDir, *count, version)
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
