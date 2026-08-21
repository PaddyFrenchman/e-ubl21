// This file discovers the UBL 2.1 schema inventory: which XSD files exist
// under the maindoc/ and common/ directories, and for each main-document
// schema, its root element and top-level imports. This is the data reported
// by the `inventory` subcommand and consumed by `generate`/`validate` to
// enumerate generation targets.
package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type schemaInventory struct {
	UBLVersion     string       `json:"ubl_version"`
	MainSchemas    int          `json:"main_schemas"`
	CommonSchemas  int          `json:"common_schemas"`
	MainFiles      []schemaInfo `json:"main_files"`
	CommonFiles    []string     `json:"common_files"`
	GeneratedFiles []string     `json:"generated_files,omitempty"`
	ValidSamples   int          `json:"valid_samples,omitempty"`
	InvalidSamples int          `json:"invalid_samples,omitempty"`
}

type schemaInfo struct {
	File        string   `json:"file"`
	RootElement string   `json:"root_element,omitempty"`
	Imports     []string `json:"imports,omitempty"`
}

func discoverInventory(xsdDir string) (schemaInventory, error) {
	mainDir := filepath.Join(xsdDir, "maindoc")
	commonDir := filepath.Join(xsdDir, "common")

	mainFiles, err := listXSDs(mainDir)
	if err != nil {
		return schemaInventory{}, fmt.Errorf("list main-doc schemas: %w", err)
	}
	commonFiles, err := listXSDs(commonDir)
	if err != nil {
		return schemaInventory{}, fmt.Errorf("list common schemas: %w", err)
	}

	report := schemaInventory{UBLVersion: "2.1", MainFiles: []schemaInfo{}, CommonFiles: []string{}}
	for _, file := range mainFiles {
		info, err := parseSchema(file)
		if err != nil {
			return schemaInventory{}, fmt.Errorf("parse schema %s: %w", file, err)
		}
		report.MainFiles = append(report.MainFiles, info)
	}
	for _, file := range commonFiles {
		report.CommonFiles = append(report.CommonFiles, filepath.Base(file))
	}
	report.MainSchemas = len(report.MainFiles)
	report.CommonSchemas = len(report.CommonFiles)
	return report, nil
}

func listXSDs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".xsd") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

func parseSchema(path string) (schemaInfo, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return schemaInfo{}, err
	}
	var doc xsdSchema
	if err := xml.Unmarshal(content, &doc); err != nil {
		return schemaInfo{}, err
	}
	info := schemaInfo{File: filepath.Base(path)}
	for _, imp := range doc.Imports {
		if imp.SchemaLocation != "" {
			info.Imports = append(info.Imports, imp.SchemaLocation)
		}
	}
	for _, element := range doc.Elements {
		if element.Name != "" {
			info.RootElement = element.Name
			break
		}
	}
	if info.RootElement == "" {
		return info, fmt.Errorf("no root element discovered in %s", path)
	}
	return info, nil
}
