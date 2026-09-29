package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// schemaCoverage is a deterministic inventory of the schema constructs that
// the tracked corpus can currently exercise. IDs include the declaring
// namespace and schema path, so same-named UBL declarations cannot collide.
type schemaCoverage struct {
	SchemaVersion int                         `json:"schemaVersion"`
	Fixtures      int                         `json:"fixtures"`
	Categories    map[string]coverageCategory `json:"categories"`
	Uncovered     map[string][]string         `json:"uncovered"`
}

type coverageCategory struct {
	Total     int     `json:"total"`
	Exercised int     `json:"exercised"`
	Percent   float64 `json:"percent"`
}

type coverageTargets struct {
	elements, optional, repeatable, attributes, optionalAttributes, choices, choiceAlternatives, wildcards, complexTypes map[string]struct{}
}

func newCoverageTargets() coverageTargets {
	return coverageTargets{map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}}
}

func coverageID(doc *schemaDoc, kind, name string) string {
	return doc.TargetNamespace + "|" + doc.File + "|" + kind + "|" + name
}
func schemaCoverageTargets(index schemaIndex) coverageTargets {
	t := newCoverageTargets()
	for _, doc := range index.ByFile {
		for name, element := range doc.Elements {
			t.addElement(doc, "global/"+name, element, false)
		}
		for name, typ := range doc.Types {
			t.complexTypes[coverageID(doc, "complexType", name)] = struct{}{}
			for _, attribute := range append(append([]xsdAttribute{}, typ.Attributes...), extensionAttributes(typ)...) {
				if attribute.Name == "" {
					continue
				}
				id := coverageID(doc, "attribute", name+"/@"+attribute.Name)
				t.attributes[id] = struct{}{}
				if attribute.Use != "required" {
					t.optionalAttributes[id] = struct{}{}
				}
			}
			t.addSequence(doc, name, typ.Sequence)
			t.addAll(doc, name, typ.All)
			t.addChoice(doc, name, typ.Choice)
			if typ.ComplexContent != nil && typ.ComplexContent.Extension != nil {
				t.addSequence(doc, name, typ.ComplexContent.Extension.Sequence)
				t.addAll(doc, name, typ.ComplexContent.Extension.All)
				t.addChoice(doc, name, typ.ComplexContent.Extension.Choice)
			}
		}
	}
	return t
}

func (t coverageTargets) addElement(doc *schemaDoc, parent string, element xsdElementDecl, choice bool) {
	name := element.Name
	if element.Ref != "" {
		name = localPart(element.Ref)
	}
	if name == "" {
		return
	}
	id := coverageID(doc, "element", parent+"/"+name)
	t.elements[id] = struct{}{}
	if element.MinOccurs == "0" {
		t.optional[id] = struct{}{}
	}
	if element.MaxOccurs == "unbounded" || (element.MaxOccurs != "" && element.MaxOccurs != "1") {
		t.repeatable[id] = struct{}{}
	}
	if choice {
		t.choiceAlternatives[id] = struct{}{}
	}
}
func (t coverageTargets) addSequence(doc *schemaDoc, parent string, sequence *xsdSequence) {
	if sequence == nil {
		return
	}
	for i, element := range sequence.Elements {
		t.addElement(doc, fmt.Sprintf("%s/sequence[%d]", parent, i), element, false)
	}
	if sequence.Any != nil {
		t.wildcards[coverageID(doc, "any", parent+"/sequence")] = struct{}{}
	}
}
func (t coverageTargets) addAll(doc *schemaDoc, parent string, all *xsdAll) {
	if all == nil {
		return
	}
	for i, element := range all.Elements {
		t.addElement(doc, fmt.Sprintf("%s/all[%d]", parent, i), element, false)
	}
}
func (t coverageTargets) addChoice(doc *schemaDoc, parent string, choice *xsdChoice) {
	if choice == nil {
		return
	}
	id := coverageID(doc, "choice", parent)
	t.choices[id] = struct{}{}
	for i, element := range choice.Elements {
		t.addElement(doc, fmt.Sprintf("%s/choice[%d]", parent, i), element, true)
	}
}

func extensionAttributes(typ xsdComplexType) []xsdAttribute {
	if typ.SimpleContent != nil && typ.SimpleContent.Extension != nil {
		return typ.SimpleContent.Extension.Attributes
	}
	return nil
}

func fixtureCoverage(samples string) (map[string]struct{}, map[string]struct{}, error) {
	elements, attributes := map[string]struct{}{}, map[string]struct{}{}
	err := filepath.WalkDir(samples, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".xml" {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		decoder := xml.NewDecoder(file)
		for {
			token, err := decoder.Token()
			if err != nil {
				break
			}
			if start, ok := token.(xml.StartElement); ok {
				elements[start.Name.Local] = struct{}{}
				for _, a := range start.Attr {
					attributes[a.Name.Local] = struct{}{}
				}
			}
		}
		return nil
	})
	return elements, attributes, err
}

func coverageCategoryFor(targets map[string]struct{}, observed map[string]struct{}, attribute bool) (coverageCategory, []string) {
	uncovered := make([]string, 0)
	exercised := 0
	for id := range targets {
		parts := strings.Split(id, "|")
		name := parts[len(parts)-1]
		if !attribute {
			name = name[strings.LastIndex(name, "/")+1:]
		}
		if attribute {
			name = strings.TrimPrefix(name[strings.LastIndex(name, "/@"):], "/@")
		}
		if _, ok := observed[name]; ok {
			exercised++
		} else {
			uncovered = append(uncovered, id)
		}
	}
	sort.Strings(uncovered)
	percent := 100.0
	if len(targets) > 0 {
		percent = float64(exercised) * 100 / float64(len(targets))
	}
	return coverageCategory{Total: len(targets), Exercised: exercised, Percent: percent}, uncovered
}

func buildSchemaCoverage(xsdDir, samples string) (schemaCoverage, error) {
	index, err := buildSchemaIndex(xsdDir)
	if err != nil {
		return schemaCoverage{}, err
	}
	targets := schemaCoverageTargets(index)
	elements, attributes, err := fixtureCoverage(samples)
	if err != nil {
		return schemaCoverage{}, err
	}
	result := schemaCoverage{SchemaVersion: 1, Categories: map[string]coverageCategory{}, Uncovered: map[string][]string{}}
	fixtureCount := 0
	_ = filepath.WalkDir(samples, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(path) == ".xml" {
			fixtureCount++
		}
		return err
	})
	result.Fixtures = fixtureCount
	for _, item := range []struct {
		name   string
		values map[string]struct{}
		attrs  bool
	}{{"elements", targets.elements, false}, {"optionalElements", targets.optional, false}, {"repeatableElements", targets.repeatable, false}, {"attributes", targets.attributes, true}, {"optionalAttributes", targets.optionalAttributes, true}, {"complexTypes", targets.complexTypes, false}, {"choiceSites", targets.choices, false}, {"choiceAlternatives", targets.choiceAlternatives, false}, {"wildcards", targets.wildcards, false}} {
		observed := elements
		if item.attrs {
			observed = attributes
		}
		category, uncovered := coverageCategoryFor(item.values, observed, item.attrs)
		result.Categories[item.name] = category
		result.Uncovered[item.name] = uncovered
	}
	return result, nil
}

func writeCoverageMarkdown(path string, report schemaCoverage) error {
	var out strings.Builder
	out.WriteString("# UBL 2.1 schema coverage\n\n")
	out.WriteString(fmt.Sprintf("Fixtures analyzed: %d\n\n", report.Fixtures))
	out.WriteString("| Construct | Exercised | Total | Coverage |\n|---|---:|---:|---:|\n")
	keys := make([]string, 0, len(report.Categories))
	for key := range report.Categories {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		c := report.Categories[key]
		out.WriteString(fmt.Sprintf("| %s | %d | %d | %.1f%% |\n", key, c.Exercised, c.Total, c.Percent))
	}
	return os.WriteFile(path, []byte(out.String()), 0o644)
}

func runCoverage(args []string) error {
	flags := flag.NewFlagSet("coverage", flag.ContinueOnError)
	xsdDir := flags.String("xsd", "XSD", "path to the UBL XSD tree")
	samples := flags.String("samples", filepath.Join("createsamples", "samples"), "tracked structural corpus directory")
	jsonPath := flags.String("json", "", "required JSON report path")
	markdownPath := flags.String("markdown", "", "required Markdown report path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *jsonPath == "" || *markdownPath == "" {
		return fmt.Errorf("--json and --markdown are required")
	}
	report, err := buildSchemaCoverage(*xsdDir, *samples)
	if err != nil {
		return err
	}
	data, err := marshalCoverage(report)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*jsonPath), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*markdownPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(*jsonPath, data, 0o644); err != nil {
		return err
	}
	return writeCoverageMarkdown(*markdownPath, report)
}

func marshalCoverage(report schemaCoverage) ([]byte, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	return append(data, '\n'), err
}
