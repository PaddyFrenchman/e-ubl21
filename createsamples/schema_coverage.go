package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// schemaCoverage is a deterministic inventory of the schema constructs that
// the tracked corpus can currently exercise. IDs include the declaring
// namespace and schema path, so same-named UBL declarations cannot collide.
type schemaCoverage struct {
	SchemaVersion       int                           `json:"schemaVersion"`
	Fixtures            int                           `json:"fixtures"`
	Categories          map[string]coverageCategory   `json:"categories"`
	Uncovered           map[string][]string           `json:"uncovered"`
	Unparseable         map[string]string             `json:"unparseable"`
	FixtureObservations map[string]fixtureObservation `json:"fixtureObservations"`
	FixtureTargets      map[string][]string           `json:"fixtureTargets"`
	MaximumCardinality  map[string]int                `json:"maximumCardinality"`
}

type coverageCategory struct {
	Total     int     `json:"total"`
	Exercised int     `json:"exercised"`
	Percent   float64 `json:"percent"`
}
type fixtureObservation struct {
	Elements   []string `json:"elements"`
	Attributes []string `json:"attributes"`
}

type coverageTargets struct {
	elements, required, optional, repeatable, sequenceMembers, allMembers, attributes, optionalAttributes, choices, choiceAlternatives, wildcards, inheritance, substitutions, recursiveEdges, complexTypes map[string]struct{}
}

func newCoverageTargets() coverageTargets {
	return coverageTargets{elements: map[string]struct{}{}, required: map[string]struct{}{}, optional: map[string]struct{}{}, repeatable: map[string]struct{}{}, sequenceMembers: map[string]struct{}{}, allMembers: map[string]struct{}{}, attributes: map[string]struct{}{}, optionalAttributes: map[string]struct{}{}, choices: map[string]struct{}{}, choiceAlternatives: map[string]struct{}{}, wildcards: map[string]struct{}{}, inheritance: map[string]struct{}{}, substitutions: map[string]struct{}{}, recursiveEdges: map[string]struct{}{}, complexTypes: map[string]struct{}{}}
}

func coverageID(doc *schemaDoc, kind, name string) string {
	return doc.TargetNamespace + "|" + doc.File + "|" + kind + "|" + name
}
func schemaCoverageTargets(index schemaIndex) coverageTargets {
	t := newCoverageTargets()
	for _, doc := range index.ByFile {
		for name, element := range doc.Elements {
			if element.SubstitutionGroup != "" {
				ns, head := qualifyRef(element.SubstitutionGroup, doc.TargetNamespace)
				t.substitutions[coverageID(doc, "substitution", name+"->"+ns+"#"+head)] = struct{}{}
			}
			t.addElement(doc, "global/"+name, element, false)
		}
		for name, typ := range doc.Types {
			t.complexTypes[coverageID(doc, "complexType", name)] = struct{}{}
			if typ.ComplexContent != nil {
				base := ""
				if typ.ComplexContent.Extension != nil {
					base = typ.ComplexContent.Extension.Base
				}
				if typ.ComplexContent.Restriction != nil {
					base = typ.ComplexContent.Restriction.Base
				}
				if base != "" {
					ns, baseName := qualifyRef(base, doc.TargetNamespace)
					t.inheritance[coverageID(doc, "inheritance", name+"->"+ns+"#"+baseName)] = struct{}{}
				}
			}
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
	if element.MinOccurs != "0" {
		t.required[id] = struct{}{}
	}
	if strings.Contains(parent, "/sequence[") {
		t.sequenceMembers[id] = struct{}{}
	}
	if strings.Contains(parent, "/all[") {
		t.allMembers[id] = struct{}{}
	}
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
	for i := range sequence.Sequences {
		t.addSequence(doc, fmt.Sprintf("%s/sequence[%d]", parent, i), &sequence.Sequences[i])
	}
	for i := range sequence.Choices {
		t.addChoice(doc, fmt.Sprintf("%s/choice[%d]", parent, i), &sequence.Choices[i])
	}
	for i := range sequence.Alls {
		t.addAll(doc, fmt.Sprintf("%s/all[%d]", parent, i), &sequence.Alls[i])
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
	for i := range all.Sequences {
		t.addSequence(doc, fmt.Sprintf("%s/sequence[%d]", parent, i), &all.Sequences[i])
	}
	for i := range all.Choices {
		t.addChoice(doc, fmt.Sprintf("%s/choice[%d]", parent, i), &all.Choices[i])
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
	for i := range choice.Sequences {
		t.addSequence(doc, fmt.Sprintf("%s/sequence[%d]", parent, i), &choice.Sequences[i])
	}
	for i := range choice.Choices {
		t.addChoice(doc, fmt.Sprintf("%s/choice[%d]", parent, i), &choice.Choices[i])
	}
}

func extensionAttributes(typ xsdComplexType) []xsdAttribute {
	if typ.SimpleContent != nil && typ.SimpleContent.Extension != nil {
		return typ.SimpleContent.Extension.Attributes
	}
	return nil
}

func fixtureCoverage(samples string) (map[string]struct{}, map[string]struct{}, map[string]string, map[string]fixtureObservation, error) {
	elements, attributes, unparseable, observations := map[string]struct{}{}, map[string]struct{}{}, map[string]string{}, map[string]fixtureObservation{}
	err := filepath.WalkDir(samples, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || filepath.Ext(path) != ".xml" {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		fixtureElements, fixtureAttributes := map[string]struct{}{}, map[string]struct{}{}
		decoder := xml.NewDecoder(file)
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				unparseable[filepath.ToSlash(path)] = err.Error()
				return nil
			}
			if start, ok := token.(xml.StartElement); ok {
				fixtureElements[start.Name.Space+"\x23"+start.Name.Local] = struct{}{}
				for _, a := range start.Attr {
					fixtureAttributes[a.Name.Space+"\x23"+a.Name.Local] = struct{}{}
				}
			}
		}
		for name := range fixtureElements {
			elements[name] = struct{}{}
		}
		observations[filepath.ToSlash(path)] = fixtureObservation{Elements: sortedSet(fixtureElements), Attributes: sortedSet(fixtureAttributes)}
		for name := range fixtureAttributes {
			attributes[name] = struct{}{}
		}
		return nil
	})
	return elements, attributes, unparseable, observations, err
}
func sortedSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func coverageCategoryFor(targets map[string]struct{}, observed map[string]struct{}, attribute bool) (coverageCategory, []string) {
	uncovered := make([]string, 0)
	exercised := 0
	for id := range targets {
		parts := strings.Split(id, "|")
		name := parts[len(parts)-1]
		namespace := parts[0]
		if !attribute {
			name = name[strings.LastIndex(name, "/")+1:]
			name = namespace + "\x23" + name
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

func structuralFixtureTargets(index schemaIndex, samples string) (map[string]map[string]int, map[string]string, error) {
	out, bad := map[string]map[string]int{}, map[string]string{}
	err := filepath.WalkDir(samples, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() || filepath.Ext(path) != ".xml" {
			return nil
		}
		m, e := matchFixture(index, path)
		if e != nil {
			bad[filepath.ToSlash(path)] = e.Error()
			return nil
		}
		out[filepath.ToSlash(path)] = m
		return nil
	})
	return out, bad, err
}
func buildSchemaCoverage(xsdDir, samples string) (schemaCoverage, error) {
	index, err := buildSchemaIndex(xsdDir)
	if err != nil {
		return schemaCoverage{}, err
	}
	targets := schemaCoverageTargets(index)
	_, _, unparseable, observations, err := fixtureCoverage(samples)
	if err != nil {
		return schemaCoverage{}, err
	}
	matches, bad, err := structuralFixtureTargets(index, samples)
	if err != nil {
		return schemaCoverage{}, err
	}
	for path, message := range bad {
		unparseable[path] = message
	}
	result := schemaCoverage{SchemaVersion: 2, Fixtures: len(observations) + len(unparseable), Categories: map[string]coverageCategory{}, Uncovered: map[string][]string{}, Unparseable: unparseable, FixtureObservations: observations, FixtureTargets: map[string][]string{}, MaximumCardinality: map[string]int{}}
	exercised := map[string]struct{}{}
	for path, found := range matches {
		ids := make([]string, 0, len(found))
		for id, occurrences := range found {
			if occurrences > result.MaximumCardinality[id] {
				result.MaximumCardinality[id] = occurrences
			}
			exercised[id] = struct{}{}
			ids = append(ids, id)
		}
		sort.Strings(ids)
		result.FixtureTargets[path] = ids
	}
	for _, item := range []struct {
		name   string
		values map[string]struct{}
	}{{"elements", targets.elements}, {"requiredElements", targets.required}, {"sequenceMembers", targets.sequenceMembers}, {"allMembers", targets.allMembers}, {"optionalElements", targets.optional}, {"repeatableElements", targets.repeatable}, {"attributes", targets.attributes}, {"optionalAttributes", targets.optionalAttributes}, {"complexTypes", targets.complexTypes}, {"choiceSites", targets.choices}, {"choiceAlternatives", targets.choiceAlternatives}, {"wildcards", targets.wildcards}, {"inheritance", targets.inheritance}, {"substitutionGroups", targets.substitutions}, {"recursiveEdges", targets.recursiveEdges}} {
		count := 0
		for id := range item.values {
			if _, ok := exercised[id]; ok {
				count++
			} else {
				result.Uncovered[item.name] = append(result.Uncovered[item.name], id)
			}
		}
		sort.Strings(result.Uncovered[item.name])
		pct := 100.0
		if len(item.values) > 0 {
			pct = float64(count) * 100 / float64(len(item.values))
		}
		result.Categories[item.name] = coverageCategory{len(item.values), count, pct}
	}
	return result, nil
}
func writeCoverageMarkdown(path string, report schemaCoverage) error {
	var out strings.Builder
	out.WriteString("# UBL 2.1 schema coverage\n\n")
	out.WriteString(fmt.Sprintf("Fixtures analyzed: %d\n\n", report.Fixtures))

	out.WriteString("Maximum cardinality is the greatest number of matches for one schema particle in a single fixture; nested parent-instance cardinality is reported by the exact fixture target map.\n\n")
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
	for _, key := range keys {
		if missing := report.Uncovered[key]; len(missing) > 0 {
			out.WriteString("\n## Uncovered " + key + "\n\n")
			for _, id := range missing {
				out.WriteString("- `" + id + "`\n")
			}
		}
	}
	if len(report.Unparseable) > 0 {
		out.WriteString("\n## Unparseable fixtures\n\n")
		paths := make([]string, 0, len(report.Unparseable))
		for path := range report.Unparseable {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			out.WriteString("- `" + path + "`: " + report.Unparseable[path] + "\n")
		}
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
