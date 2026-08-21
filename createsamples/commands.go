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
	count := flags.Int("count", 1, "number of sample files to generate per main document schema")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *count <= 0 {
		return errors.New("count must be greater than zero")
	}

	report, err := discoverInventory(*xsdDir)
	if err != nil {
		return err
	}
	if len(report.MainFiles) == 0 {
		return errors.New("no main document XSD files discovered")
	}
	if err := os.RemoveAll(*output); err != nil {
		return fmt.Errorf("clear existing output %s: %w", *output, err)
	}
	if err := os.MkdirAll(*output, 0o755); err != nil {
		return fmt.Errorf("create output dir %s: %w", *output, err)
	}

	index, err := buildSchemaIndex(*xsdDir)
	if err != nil {
		return err
	}

	generated := 0
	for _, schema := range report.MainFiles {
		root := schema.RootElement
		if root == "" {
			root = strings.TrimSuffix(filepath.Base(schema.File), filepath.Ext(schema.File))
		}
		folder := filepath.Join(*output, root)
		if err := os.MkdirAll(folder, 0o755); err != nil {
			return fmt.Errorf("create sample dir %s: %w", folder, err)
		}
		for i := 1; i <= *count; i++ {
			path := filepath.Join(folder, fmt.Sprintf("%s-sample-%02d.xml", root, i))
			content, err := generateCompleteSample(index, root, *xsdDir, schema.File)
			if err != nil {
				return fmt.Errorf("generate %s: %w", path, err)
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}
			fmt.Println(path)
			generated++
		}
	}
	fmt.Printf("Generated %d samples under %s\n", generated, *output)
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
