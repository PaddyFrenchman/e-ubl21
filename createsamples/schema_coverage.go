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
	SchemaVersion          int                           `json:"schemaVersion"`
	Fixtures               int                           `json:"fixtures"`
	Categories             map[string]coverageCategory   `json:"categories"`
	Uncovered              map[string][]string           `json:"uncovered"`
	Unparseable            map[string]string             `json:"unparseable"`
	FixtureObservations    map[string]fixtureObservation `json:"fixtureObservations"`
	FixtureTargets         map[string][]string           `json:"fixtureTargets"`
	MaximumCardinality     map[string]int                `json:"maximumCardinality"`
	ExercisedCardinalities map[string][]int              `json:"exercisedCardinalities"`
	RequiredCardinalities  map[string][]int              `json:"requiredCardinalities"`
	CardinalityGaps        []string                      `json:"cardinalityGaps"`
	MaximumDepth           map[string]int                `json:"maximumDepth"`
}

// cardinalityResidual is a target-level, non-waiving disposition for a
// repeatable particle that has not met the structural cardinality contract.
// A residual is never an approval: its only disposition is to add an
// XSD-valid fixture that exercises each missing cardinality.
type cardinalityResidual struct {
	Target      string `json:"target"`
	Exercised   []int  `json:"exercised"`
	Required    []int  `json:"required"`
	Missing     []int  `json:"missing"`
	Disposition string `json:"disposition"`
	Approved    bool   `json:"approved"`
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
	elements, roots, required, optional, repeatable, sequenceMembers, allMembers, attributes, requiredAttributes, optionalAttributes, choices, choiceAlternatives, wildcards, inheritance, substitutions, recursiveEdges, complexTypes map[string]struct{}
	repeatableBounds                                                                                                                                                                                                                   map[string]occurrenceBounds
}

// occurrenceBounds are the effective bounds after enclosing model groups.
// An unbounded maximum is represented by -1.
type occurrenceBounds struct{ min, max int }

func newCoverageTargets() coverageTargets {
	return coverageTargets{elements: map[string]struct{}{}, roots: map[string]struct{}{}, required: map[string]struct{}{}, optional: map[string]struct{}{}, repeatable: map[string]struct{}{}, repeatableBounds: map[string]occurrenceBounds{}, sequenceMembers: map[string]struct{}{}, allMembers: map[string]struct{}{}, attributes: map[string]struct{}{}, requiredAttributes: map[string]struct{}{}, optionalAttributes: map[string]struct{}{}, choices: map[string]struct{}{}, choiceAlternatives: map[string]struct{}{}, wildcards: map[string]struct{}{}, inheritance: map[string]struct{}{}, substitutions: map[string]struct{}{}, recursiveEdges: map[string]struct{}{}, complexTypes: map[string]struct{}{}}
}

func coverageID(doc *schemaDoc, kind, name string) string {
	return doc.TargetNamespace + "|" + doc.File + "|" + kind + "|" + name
}
func complexTypeEdges(index schemaIndex, doc *schemaDoc, typ xsdComplexType) []string {
	var out []string
	var add func([]xsdElementDecl)
	add = func(elements []xsdElementDecl) {
		for _, e := range elements {
			d, resolved, _, _ := elementDeclaration(index, doc, e)
			ns, name := directTypeName(d, resolved)
			if targetDoc := index.ByTargetNS[ns]; name != "" && targetDoc != nil {
				if _, ok := targetDoc.Types[name]; !ok {
					continue
				}
				out = append(out, qualifiedKey(ns, name))
			}
		}
	}
	var sequence func(*xsdSequence)
	var choice func(*xsdChoice)
	var all func(*xsdAll)
	sequence = func(p *xsdSequence) {
		if p != nil {
			add(p.Elements)
			for i := range p.Sequences {
				sequence(&p.Sequences[i])
			}
			for i := range p.Choices {
				choice(&p.Choices[i])
			}
			for i := range p.Alls {
				all(&p.Alls[i])
			}
		}
	}
	choice = func(p *xsdChoice) {
		if p != nil {
			add(p.Elements)
			for i := range p.Sequences {
				sequence(&p.Sequences[i])
			}
			for i := range p.Choices {
				choice(&p.Choices[i])
			}
			for i := range p.Alls {
				all(&p.Alls[i])
			}
		}
	}
	all = func(p *xsdAll) {
		if p != nil {
			add(p.Elements)
			for i := range p.Sequences {
				sequence(&p.Sequences[i])
			}
			for i := range p.Choices {
				choice(&p.Choices[i])
			}
			for i := range p.Alls {
				all(&p.Alls[i])
			}
		}
	}
	sequence(typ.Sequence)
	choice(typ.Choice)
	all(typ.All)
	if typ.ComplexContent != nil && typ.ComplexContent.Extension != nil {
		sequence(typ.ComplexContent.Extension.Sequence)
		choice(typ.ComplexContent.Extension.Choice)
		all(typ.ComplexContent.Extension.All)
	}
	return out
}
func reachable(graph map[string][]string, from, want string, seen map[string]bool) bool {
	if from == want {
		return true
	}
	if seen[from] {
		return false
	}
	seen[from] = true
	for _, next := range graph[from] {
		if reachable(graph, next, want, seen) {
			return true
		}
	}
	return false
}
func (t coverageTargets) addRecursiveTypeEdges(index schemaIndex) {
	graph := map[string][]string{}
	for _, doc := range index.ByFile {
		for name, typ := range doc.Types {
			graph[qualifiedKey(doc.TargetNamespace, name)] = complexTypeEdges(index, doc, typ)
		}
	}
	for _, doc := range index.ByFile {
		for name, typ := range doc.Types {
			owner := qualifiedKey(doc.TargetNamespace, name)
			for _, next := range complexTypeEdges(index, doc, typ) {
				if reachable(graph, next, owner, map[string]bool{}) {
					t.recursiveEdges[coverageID(doc, "recursive", name+"->"+next)] = struct{}{}
				}
			}
		}
	}
}
func schemaCoverageTargets(index schemaIndex) coverageTargets {
	t := newCoverageTargets()
	t.addRecursiveTypeEdges(index)
	for sourcePath, doc := range index.ByFile {
		for name, element := range doc.Elements {
			if element.SubstitutionGroup != "" {
				ns, head := qualifyRef(element.SubstitutionGroup, doc.TargetNamespace)
				t.substitutions[coverageID(doc, "substitution", name+"->"+ns+"#"+head)] = struct{}{}
			}
			t.addElement(doc, "global/"+name, element, false)
			if strings.Contains(filepath.ToSlash(sourcePath), "/maindoc/") {
				t.roots[coverageID(doc, "element", "global/"+name)] = struct{}{}
			}
		}
		for name, typ := range doc.Types {
			t.complexTypes[coverageID(doc, "complexType", name)] = struct{}{}
			if base := typeBase(typ); base != "" {
				ns, baseName := qualifyRef(base, doc.TargetNamespace)
				t.inheritance[coverageID(doc, "inheritance", name+"->"+ns+"#"+baseName)] = struct{}{}
			}
			for _, attribute := range typeAttributes(typ) {
				if attribute.Name == "" {
					continue
				}
				id := coverageID(doc, "attribute", name+"/@"+attribute.Name)
				t.attributes[id] = struct{}{}
				if attribute.Use == "required" {
					t.requiredAttributes[id] = struct{}{}
				} else {
					t.optionalAttributes[id] = struct{}{}
				}
			}
			t.addSequence(doc, name, typ.Sequence)
			if typ.Sequence != nil {
				for _, e := range typ.Sequence.Elements {
					t.addRecursiveEdge(index, doc, name, e)
				}
			}
			if typ.All != nil {
				for _, e := range typ.All.Elements {
					t.addRecursiveEdge(index, doc, name, e)
				}
			}
			if typ.Choice != nil {
				for _, e := range typ.Choice.Elements {
					t.addRecursiveEdge(index, doc, name, e)
				}
			}
			t.addAll(doc, name, typ.All)
			t.addChoice(doc, name, typ.Choice)
			for _, p := range typeParticleSets(typ) {
				t.addSequence(doc, name, p.sequence)
				t.addAll(doc, name, p.all)
				t.addChoice(doc, name, p.choice)
			}
		}
	}
	return t
}

type particleSet struct {
	sequence *xsdSequence
	choice   *xsdChoice
	all      *xsdAll
}

func typeParticleSets(typ xsdComplexType) []particleSet {
	out := []particleSet{{typ.Sequence, typ.Choice, typ.All}}
	if typ.ComplexContent != nil {
		if x := typ.ComplexContent.Extension; x != nil {
			out = append(out, particleSet{x.Sequence, x.Choice, x.All})
		}
		if x := typ.ComplexContent.Restriction; x != nil {
			out = append(out, particleSet{x.Sequence, x.Choice, x.All})
		}
	}
	return out
}

func directTypeName(doc *schemaDoc, element xsdElementDecl) (string, string) {
	if element.Type == "" {
		return "", ""
	}
	return qualifyRef(element.Type, doc.TargetNamespace)
}
func (t coverageTargets) addRecursiveEdge(index schemaIndex, doc *schemaDoc, owner string, element xsdElementDecl) {
	ns, name := directTypeName(doc, element)
	if name == "" {
		return
	}
	if ns == doc.TargetNamespace && name == owner {
		t.recursiveEdges[coverageID(doc, "recursive", owner+"->"+name)] = struct{}{}
	}
}

type occurrence struct {
	optional, repeatable bool
	bounds               occurrenceBounds
}

func withOccurs(parent occurrence, min, max string) occurrence {
	if parent.bounds.max == 0 {
		parent.bounds = occurrenceBounds{min: 1, max: 1}
	}
	childMin := parseOccursInt(min, 1)
	childMax := parseMaxOccurs(max)
	if max == "unbounded" {
		childMax = -1
	} else if max == "" {
		childMax = 1
	}
	bounds := occurrenceBounds{min: parent.bounds.min * childMin, max: parent.bounds.max * childMax}
	if parent.bounds.max < 0 || childMax < 0 {
		bounds.max = -1
	}
	return occurrence{parent.optional || childMin == 0, bounds.max < 0 || bounds.max > 1, bounds}
}
func currentOccurrence(values []occurrence) occurrence {
	if len(values) > 0 {
		return values[0]
	}
	return occurrence{bounds: occurrenceBounds{min: 1, max: 1}}
}

func (t coverageTargets) addElement(doc *schemaDoc, parent string, element xsdElementDecl, choice bool, inherited ...occurrence) {
	occurs := withOccurs(currentOccurrence(inherited), element.MinOccurs, element.MaxOccurs)
	name := element.Name
	if element.Ref != "" {
		name = localPart(element.Ref)
	}
	if name == "" {
		return
	}
	id := coverageID(doc, "element", parent+"/"+name)
	t.elements[id] = struct{}{}
	if element.ComplexType != nil {
		t.complexTypes[strings.Replace(id, "|element|", "|complexType|inline/", 1)] = struct{}{}
	}
	if !occurs.optional {
		t.required[id] = struct{}{}
	}
	if strings.Contains(parent, "/sequence[") {
		t.sequenceMembers[id] = struct{}{}
	}
	if strings.Contains(parent, "/all[") {
		t.allMembers[id] = struct{}{}
	}
	if occurs.optional {
		t.optional[id] = struct{}{}
	}
	if occurs.repeatable {
		t.repeatable[id] = struct{}{}
		t.repeatableBounds[id] = occurs.bounds
	}
	if choice {
		t.choiceAlternatives[id] = struct{}{}
	}
}
func (t coverageTargets) addSequence(doc *schemaDoc, parent string, sequence *xsdSequence, inherited ...occurrence) {
	if sequence == nil {
		return
	}
	occurs := withOccurs(currentOccurrence(inherited), sequence.MinOccurs, sequence.MaxOccurs)
	for i, element := range sequence.Elements {
		t.addElement(doc, fmt.Sprintf("%s/sequence[%d]", parent, i), element, false, occurs)
	}
	for i := range sequence.Sequences {
		t.addSequence(doc, fmt.Sprintf("%s/sequence[%d]", parent, i), &sequence.Sequences[i], occurs)
	}
	for i := range sequence.Choices {
		t.addChoice(doc, fmt.Sprintf("%s/choice[%d]", parent, i), &sequence.Choices[i], occurs)
	}
	for i := range sequence.Alls {
		t.addAll(doc, fmt.Sprintf("%s/all[%d]", parent, i), &sequence.Alls[i], occurs)
	}
	if sequence.Any != nil {
		t.wildcards[coverageID(doc, "any", parent+"/sequence")] = struct{}{}
	}
}
func (t coverageTargets) addAll(doc *schemaDoc, parent string, all *xsdAll, inherited ...occurrence) {
	if all == nil {
		return
	}
	occurs := withOccurs(currentOccurrence(inherited), all.MinOccurs, all.MaxOccurs)
	for i, element := range all.Elements {
		t.addElement(doc, fmt.Sprintf("%s/all[%d]", parent, i), element, false, occurs)
	}
	for i := range all.Sequences {
		t.addSequence(doc, fmt.Sprintf("%s/sequence[%d]", parent, i), &all.Sequences[i], occurs)
	}
	for i := range all.Choices {
		t.addChoice(doc, fmt.Sprintf("%s/choice[%d]", parent, i), &all.Choices[i], occurs)
	}
	if all.Any != nil {
		t.wildcards[coverageID(doc, "any", parent+"/all")] = struct{}{}
	}
}
func (t coverageTargets) addChoice(doc *schemaDoc, parent string, choice *xsdChoice, inherited ...occurrence) {
	if choice == nil {
		return
	}
	occurs := withOccurs(currentOccurrence(inherited), choice.MinOccurs, choice.MaxOccurs)
	id := coverageID(doc, "choice", parent)
	t.choices[id] = struct{}{}
	// A required choice requires one alternative, not every alternative. Only a
	// single-alternative choice can make its contained element individually
	// required.
	alternatives := len(choice.Elements) + len(choice.Sequences) + len(choice.Choices) + len(choice.Alls)
	alternativeOccurs := occurs
	if alternatives > 1 || choice.Any != nil {
		alternativeOccurs.optional = true
	}
	for i, element := range choice.Elements {
		t.addElement(doc, fmt.Sprintf("%s/choice[%d]", parent, i), element, true, alternativeOccurs)
	}
	for i := range choice.Sequences {
		t.choiceAlternatives[coverageID(doc, "choiceAlternative", fmt.Sprintf("%s/sequence[%d]", parent, i))] = struct{}{}
		t.addSequence(doc, fmt.Sprintf("%s/sequence[%d]", parent, i), &choice.Sequences[i], alternativeOccurs)
	}
	for i := range choice.Choices {
		t.choiceAlternatives[coverageID(doc, "choiceAlternative", fmt.Sprintf("%s/choice[%d]", parent, i))] = struct{}{}
		t.addChoice(doc, fmt.Sprintf("%s/choice[%d]", parent, i), &choice.Choices[i], alternativeOccurs)
	}
	for i := range choice.Alls {
		t.choiceAlternatives[coverageID(doc, "choiceAlternative", fmt.Sprintf("%s/all[%d]", parent, i))] = struct{}{}
		t.addAll(doc, fmt.Sprintf("%s/all[%d]", parent, i), &choice.Alls[i], alternativeOccurs)
	}
	if choice.Any != nil {
		t.wildcards[coverageID(doc, "any", parent+"/choice")] = struct{}{}
	}
}

func typeBase(typ xsdComplexType) string {
	if typ.ComplexContent != nil {
		if typ.ComplexContent.Extension != nil {
			return typ.ComplexContent.Extension.Base
		}
		if typ.ComplexContent.Restriction != nil {
			return typ.ComplexContent.Restriction.Base
		}
	}
	if typ.SimpleContent != nil {
		if typ.SimpleContent.Extension != nil {
			return typ.SimpleContent.Extension.Base
		}
		if typ.SimpleContent.Restriction != nil {
			return typ.SimpleContent.Restriction.Base
		}
	}
	return ""
}

func typeAttributes(typ xsdComplexType) []xsdAttribute {
	out := append([]xsdAttribute{}, typ.Attributes...)
	if typ.ComplexContent != nil {
		if typ.ComplexContent.Extension != nil {
			out = append(out, typ.ComplexContent.Extension.Attributes...)
		}
		if typ.ComplexContent.Restriction != nil {
			out = append(out, typ.ComplexContent.Restriction.Attributes...)
		}
	}
	if typ.SimpleContent != nil {
		if typ.SimpleContent.Extension != nil {
			out = append(out, typ.SimpleContent.Extension.Attributes...)
		}
		if typ.SimpleContent.Restriction != nil {
			out = append(out, typ.SimpleContent.Restriction.Attributes...)
		}
	}
	return out
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
		relativePath, err := fixtureReportPath(samples, path)
		if err != nil {
			return err
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
				unparseable[relativePath] = err.Error()
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
		observations[relativePath] = fixtureObservation{Elements: sortedSet(fixtureElements), Attributes: sortedSet(fixtureAttributes)}
		for name := range fixtureAttributes {
			attributes[name] = struct{}{}
		}
		return nil
	})
	return elements, attributes, unparseable, observations, err
}

// fixtureReportPath keeps coverage artifacts independent of the caller's
// staging directory. Corpus fixture IDs and manifest paths are relative too,
// so this is the only stable representation for persisted coverage evidence.
func fixtureReportPath(samples, path string) (string, error) {
	relative, err := filepath.Rel(samples, path)
	if err != nil {
		return "", fmt.Errorf("make fixture path relative to corpus root: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("fixture path %q is outside corpus root %q", path, samples)
	}
	return filepath.ToSlash(relative), nil
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

func structuralFixtureTargets(index schemaIndex, samples string) (map[string]fixtureMatch, map[string]string, error) {
	out, bad := map[string]fixtureMatch{}, map[string]string{}
	processed := 0
	fmt.Println("Coverage: matching structural fixtures")
	err := filepath.WalkDir(samples, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() || filepath.Ext(path) != ".xml" {
			return nil
		}
		relativePath, e := fixtureReportPath(samples, path)
		if e != nil {
			return e
		}
		m, e := matchFixtureDetailed(index, path)
		if e != nil {
			bad[relativePath] = e.Error()
			return nil
		}
		out[relativePath] = m
		processed++
		if processed%100 == 0 {
			fmt.Printf("Coverage: matched %d fixtures\n", processed)
		}
		return nil
	})
	if err == nil {
		fmt.Printf("Coverage: matched %d fixtures\n", processed)
	}
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
	fixturePaths := map[string]struct{}{}
	for path := range observations {
		fixturePaths[path] = struct{}{}
	}
	for path := range unparseable {
		fixturePaths[path] = struct{}{}
	}
	result := schemaCoverage{SchemaVersion: 5, Fixtures: len(fixturePaths), Categories: map[string]coverageCategory{}, Uncovered: map[string][]string{}, Unparseable: unparseable, FixtureObservations: observations, FixtureTargets: map[string][]string{}, MaximumCardinality: map[string]int{}, ExercisedCardinalities: map[string][]int{}, RequiredCardinalities: map[string][]int{}, MaximumDepth: map[string]int{}}
	// Record every repeatable schema particle even when no fixture reaches it.
	// A zero is meaningful: it identifies an unexercised collection instead of
	// silently omitting it from the cardinality report.
	for id := range targets.repeatable {
		result.MaximumCardinality[id] = 0
		result.ExercisedCardinalities[id] = nil
		result.RequiredCardinalities[id] = requiredCardinalities(targets.repeatableBounds[id])
	}
	exercised := map[string]struct{}{}
	for path, match := range matches {
		found := match.targets
		ids := make([]string, 0, len(found))
		for id, value := range found {
			if strings.Contains(id, "|recursive|") {
				if value > result.MaximumDepth[id] {
					result.MaximumDepth[id] = value
				}
			} else if value > result.MaximumCardinality[id] {
				result.MaximumCardinality[id] = value
			}
			exercised[id] = struct{}{}
			ids = append(ids, id)
		}
		sort.Strings(ids)
		result.FixtureTargets[path] = ids
	}
	recordCardinalityObservations(&result, targets, matches)
	for _, item := range []struct {
		name   string
		values map[string]struct{}
	}{{"roots", targets.roots}, {"elements", targets.elements}, {"requiredElements", targets.required}, {"sequenceMembers", targets.sequenceMembers}, {"allMembers", targets.allMembers}, {"optionalElements", targets.optional}, {"repeatableElements", targets.repeatable}, {"attributes", targets.attributes}, {"requiredAttributes", targets.requiredAttributes}, {"optionalAttributes", targets.optionalAttributes}, {"complexTypes", targets.complexTypes}, {"choiceSites", targets.choices}, {"choiceAlternatives", targets.choiceAlternatives}, {"wildcards", targets.wildcards}, {"inheritance", targets.inheritance}, {"substitutionGroups", targets.substitutions}, {"recursiveEdges", targets.recursiveEdges}} {
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
	for id := range targets.repeatable {
		if !containsAllInts(result.ExercisedCardinalities[id], result.RequiredCardinalities[id]) {
			result.CardinalityGaps = append(result.CardinalityGaps, id)
		}
	}
	sort.Strings(result.CardinalityGaps)
	return result, nil
}

func requiredCardinalities(bounds occurrenceBounds) []int {
	maximum := bounds.max
	// The fixture cap applies only to unbounded particles. A finite schema
	// maximum is itself the boundary this acceptance check must require.
	if maximum < 0 {
		maximum = maxOccursSample
	}
	required := []int{}
	if bounds.min == 0 {
		required = append(required, 0)
	}
	if bounds.min <= 1 && maximum >= 1 {
		required = append(required, 1)
	}
	if maximum >= 2 {
		// Keep the ordinary multiple case distinct from the boundary case.
		required = append(required, 2)
	}
	if maximum >= 2 && maximum != 2 {
		required = append(required, maximum)
	}
	return required
}

func appendUniqueInt(values []int, value int) []int {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func containsAllInts(values, required []int) bool {
	for _, want := range required {
		found := false
		for _, value := range values {
			if value == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func particleParentComplexTypeID(particleID string) string {
	parts := strings.Split(particleID, "|")
	if len(parts) != 4 {
		return ""
	}
	parent := parts[3]
	if slash := strings.Index(parent, "/"); slash >= 0 {
		parent = parent[:slash]
	}
	return parts[0] + "|" + parts[1] + "|complexType|" + parent
}

// recordCardinalityObservations retains every count seen beneath each parent
// instance. It adds an explicit zero for each instantiated parent that did not
// contain the particle, rather than inferring one zero from a whole fixture.
func recordCardinalityObservations(result *schemaCoverage, targets coverageTargets, matches map[string]fixtureMatch) {
	for _, match := range matches {
		for id := range targets.repeatable {
			for _, count := range match.cardinalities[id] {
				result.ExercisedCardinalities[id] = appendUniqueInt(result.ExercisedCardinalities[id], count)
			}
			parentInstances := match.parentInstances[particleParentComplexTypeID(id)]
			missing := parentInstances - len(match.cardinalities[id])
			for i := 0; i < missing; i++ {
				result.ExercisedCardinalities[id] = appendUniqueInt(result.ExercisedCardinalities[id], 0)
			}
		}
	}
	for id := range result.ExercisedCardinalities {
		sort.Ints(result.ExercisedCardinalities[id])
	}
}
func writeCoverageMarkdown(path string, report schemaCoverage) error {
	var out strings.Builder
	out.WriteString("# UBL 2.1 schema coverage\n\n")
	out.WriteString(fmt.Sprintf("Fixtures analyzed: %d\n\n", report.Fixtures))

	out.WriteString("Maximum cardinality is the greatest number of sibling matches for one schema particle under a single parent instance in a fixture. The acceptance gate records every parent-instance count and requires schema-permitted zero (when optional), one, a multiple case (two or more), and the schema maximum (capped at five for unbounded particles).\n\n")
	out.WriteString("Maximum recursive depth is the greatest number of occurrences of the repeated target type along a fixture path.\n\n")
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
	var unexercisedRepeatable []string
	for id, maximum := range report.MaximumCardinality {
		if maximum == 0 && strings.Contains(id, "|element|") {
			unexercisedRepeatable = append(unexercisedRepeatable, id)
		}
	}
	sort.Strings(unexercisedRepeatable)
	if len(unexercisedRepeatable) > 0 {
		out.WriteString("\n## Repeatable particles with zero exercised occurrences\n\n")
		for _, id := range unexercisedRepeatable {
			out.WriteString("- `" + id + "`\n")
		}
	}
	if len(report.CardinalityGaps) > 0 {
		out.WriteString("\n## Repeatable particles missing required cardinalities\n\n")
		out.WriteString("Every residual below has the non-waiving disposition `requires-xsd-valid-fixture`. No residual is approved, and this list does not relax the cardinality acceptance gate.\n\n")
		out.WriteString("| Target | Exercised | Required | Missing | Disposition | Approved |\n|---|---|---|---|---|---|\n")
		for _, residual := range cardinalityResiduals(report) {
			out.WriteString(fmt.Sprintf("| `%s` | %v | %v | %v | %s | %t |\n", residual.Target, residual.Exercised, residual.Required, residual.Missing, residual.Disposition, residual.Approved))
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

func cardinalityResiduals(report schemaCoverage) []cardinalityResidual {
	residuals := make([]cardinalityResidual, 0, len(report.CardinalityGaps))
	for _, id := range report.CardinalityGaps {
		exercised := append([]int(nil), report.ExercisedCardinalities[id]...)
		required := append([]int(nil), report.RequiredCardinalities[id]...)
		missing := make([]int, 0, len(required))
		for _, count := range required {
			if !containsAllInts(exercised, []int{count}) {
				missing = append(missing, count)
			}
		}
		residuals = append(residuals, cardinalityResidual{Target: id, Exercised: exercised, Required: required, Missing: missing, Disposition: "requires-xsd-valid-fixture", Approved: false})
	}
	sort.Slice(residuals, func(i, j int) bool { return residuals[i].Target < residuals[j].Target })
	return residuals
}

func runCoverage(args []string) error {
	flags := flag.NewFlagSet("coverage", flag.ContinueOnError)
	xsdDir := flags.String("xsd", "XSD", "path to the UBL XSD tree")
	samples := flags.String("samples", filepath.Join("createsamples", "samples-structural"), "structural corpus directory")
	jsonPath := flags.String("json", "", "required JSON report path")
	markdownPath := flags.String("markdown", "", "required Markdown report path")
	requireCardinality := flags.Bool("require-cardinality", true, "fail when a repeatable particle misses a schema-required cardinality")
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
	if err := writeCoverageMarkdown(*markdownPath, report); err != nil {
		return err
	}
	if *requireCardinality && len(report.CardinalityGaps) > 0 {
		return fmt.Errorf("cardinality acceptance failed: %d repeatable particles miss required cardinalities (see %s)", len(report.CardinalityGaps), *markdownPath)
	}
	return nil
}

func marshalCoverage(report schemaCoverage) ([]byte, error) {
	// Store the complete fixture-to-target relation without repeating long
	// namespace/path target IDs for every fixture. The catalog is a stable,
	// sorted dictionary and each fixture's values are indexes into it.
	targetSet := map[string]struct{}{}
	for _, targets := range report.FixtureTargets {
		for _, target := range targets {
			targetSet[target] = struct{}{}
		}
	}
	catalog := make([]string, 0, len(targetSet))
	for target := range targetSet {
		catalog = append(catalog, target)
	}
	sort.Strings(catalog)
	positions := make(map[string]int, len(catalog))
	for position, target := range catalog {
		positions[target] = position
	}
	fixtureTargets := make(map[string][]int, len(report.FixtureTargets))
	for fixture, targets := range report.FixtureTargets {
		indexes := make([]int, len(targets))
		for i, target := range targets {
			indexes[i] = positions[target]
		}
		fixtureTargets[fixture] = indexes
	}
	encoded := struct {
		SchemaVersion          int                           `json:"schemaVersion"`
		Fixtures               int                           `json:"fixtures"`
		Categories             map[string]coverageCategory   `json:"categories"`
		Uncovered              map[string][]string           `json:"uncovered"`
		Unparseable            map[string]string             `json:"unparseable"`
		FixtureObservations    map[string]fixtureObservation `json:"fixtureObservations"`
		TargetCatalog          []string                      `json:"targetCatalog"`
		FixtureTargetIndexes   map[string][]int              `json:"fixtureTargetIndexes"`
		MaximumCardinality     map[string]int                `json:"maximumCardinality"`
		ExercisedCardinalities map[string][]int              `json:"exercisedCardinalities"`
		RequiredCardinalities  map[string][]int              `json:"requiredCardinalities"`
		CardinalityGaps        []string                      `json:"cardinalityGaps"`
		CardinalityResiduals   []cardinalityResidual         `json:"cardinalityResiduals"`
		MaximumDepth           map[string]int                `json:"maximumDepth"`
	}{
		SchemaVersion: report.SchemaVersion, Fixtures: report.Fixtures,
		Categories: report.Categories, Uncovered: report.Uncovered, Unparseable: report.Unparseable,
		FixtureObservations: report.FixtureObservations, TargetCatalog: catalog, FixtureTargetIndexes: fixtureTargets,
		MaximumCardinality: report.MaximumCardinality, ExercisedCardinalities: report.ExercisedCardinalities,
		RequiredCardinalities: report.RequiredCardinalities, CardinalityGaps: report.CardinalityGaps, MaximumDepth: report.MaximumDepth,
		CardinalityResiduals: cardinalityResiduals(report),
	}
	data, err := json.MarshalIndent(encoded, "", "  ")
	return append(data, '\n'), err
}
