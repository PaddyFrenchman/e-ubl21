package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/PaddyFrenchman/e-ubl21/regression"
)

var immutableGeneratorVersion = regexp.MustCompile(`^git:(?:[a-f0-9]{40}|[a-f0-9]{64})$`)

const (
	corpusOwnershipMarkerName    = ".e-ubl21-generated-corpus"
	corpusOwnershipMarkerContent = "e-ubl21 createsamples generated corpus v1\n"
)

var generatorSourcePathspecs = []string{
	":(glob)createsamples/**/*.go",
	":(exclude,glob)createsamples/**/*_test.go",
	":(glob)peppol/*.go",
	":(exclude,glob)peppol/*_test.go",
	"validation-assets/peppol.go",
	"regression/manifest.go",
	"regression/schemas/fixture-manifest.schema.json",
	":(glob)XSD/**/*.xsd",
}

type fixtureManifest struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Fixtures      []fixtureManifestEntry `json:"fixtures"`
}

type fixtureManifestEntry struct {
	ID             string                 `json:"id"`
	DocumentType   string                 `json:"documentType"`
	Profile        string                 `json:"profile"`
	File           string                 `json:"file"`
	SHA256         string                 `json:"sha256"`
	Standards      fixtureStandards       `json:"standards"`
	Provenance     fixtureProvenance      `json:"provenance"`
	Rulesets       *fixtureRulesets       `json:"rulesets,omitempty"`
	Mutation       *fixtureMutation       `json:"mutation,omitempty"`
	ExpectedFinal  string                 `json:"expectedFinal"`
	ExpectedStages []fixtureExpectedStage `json:"expectedStages"`
	Tags           []string               `json:"tags"`
	// StructuralTargets identifies the #161 schema targets this fixture is
	// intended to exercise. It is present only in the structural UBL corpus.
	StructuralTargets []string `json:"structuralTargets,omitempty"`
}

type fixtureStandards struct {
	UBL     string  `json:"ubl"`
	EN16931 *string `json:"en16931"`
	Peppol  *string `json:"peppol"`
}

type fixtureProvenance struct {
	Kind             string `json:"kind"`
	Generator        string `json:"generator,omitempty"`
	GeneratorVersion string `json:"generatorVersion,omitempty"`
	Publisher        string `json:"publisher,omitempty"`
	SourceURL        string `json:"sourceUrl,omitempty"`
	SourceVersion    string `json:"sourceVersion,omitempty"`
	SourceSHA256     string `json:"sourceSha256,omitempty"`
	SourceMember     any    `json:"sourceMember,omitempty"`
	Extraction       string `json:"extraction,omitempty"`
	License          string `json:"license,omitempty"`
}

type fixtureRulesets struct {
	EN16931 *fixtureRuleset `json:"en16931,omitempty"`
	Peppol  *fixtureRuleset `json:"peppol,omitempty"`
}

type fixtureRuleset struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}

type fixtureMutation struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type fixtureExpectedStage struct {
	Stage   string   `json:"stage"`
	Outcome string   `json:"outcome"`
	RuleIDs []string `json:"ruleIds,omitempty"`
}

type generatedFixture struct {
	entry              fixtureManifestEntry
	content            []byte
	schemaPath         string
	expectedDiagnostic string
}

type generatedCorpus struct {
	fixtures     []generatedFixture
	manifestJSON []byte
}

// structuralManifestStore keeps the very large structural manifest out of the
// Go heap. SQLite is used as an append-only journal because the structural
// generator can produce hundreds of thousands of entries; keeping the entry
// structs (and their strings/slices) reachable until the final marshal made
// the collector repeatedly scan a multi-gigabyte object graph.
//
// This deliberately uses the sqlite3 command already available to the corpus
// tooling rather than adding a database driver to the product binary. The
// database is private to the staging directory and is deleted after its rows
// have been streamed into manifest.json.
type structuralManifestStore struct {
	path       string
	sqlitePath string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	writer     *bufio.Writer
	stderr     bytes.Buffer
	finished   bool
}

func newStructuralManifestStore(path string) (*structuralManifestStore, error) {
	sqlitePath, err := exec.LookPath("sqlite3")
	if err != nil {
		return nil, fmt.Errorf("sqlite3 is required to spool the structural fixture manifest: %w", err)
	}
	store := &structuralManifestStore{path: path, sqlitePath: sqlitePath}
	store.cmd = exec.Command(sqlitePath, "-batch", path)
	store.cmd.Stderr = &store.stderr
	store.stdin, err = store.cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open structural manifest SQLite input: %w", err)
	}
	store.writer = bufio.NewWriterSize(store.stdin, 256*1024)
	if err := store.cmd.Start(); err != nil {
		_ = store.stdin.Close()
		return nil, fmt.Errorf("start structural manifest SQLite journal: %w", err)
	}
	if _, err := store.writer.WriteString("PRAGMA journal_mode=OFF; PRAGMA synchronous=OFF; CREATE TABLE entries (ordinal INTEGER PRIMARY KEY, id TEXT NOT NULL UNIQUE, file TEXT NOT NULL UNIQUE, entry TEXT NOT NULL); BEGIN;\n"); err != nil {
		return nil, store.fail(err)
	}
	return store, nil
}

func (store *structuralManifestStore) add(entry fixtureManifestEntry) error {
	encoded, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal structural manifest entry %q: %w", entry.ID, err)
	}
	// JSON never contains literal newlines, and SQL single quotes are escaped
	// by doubling them. This keeps one entry per SQLite command without a Go
	// database-driver dependency or a live in-memory row cache.
	value := strings.ReplaceAll(string(encoded), "'", "''")
	id := strings.ReplaceAll(entry.ID, "'", "''")
	file := strings.ReplaceAll(entry.File, "'", "''")
	if _, err := store.writer.WriteString("INSERT INTO entries (id, file, entry) VALUES ('" + id + "', '" + file + "', '" + value + "');\n"); err != nil {
		return store.fail(err)
	}
	return nil
}

func (store *structuralManifestStore) fail(cause error) error {
	_ = store.stdin.Close()
	_ = store.cmd.Wait()
	return fmt.Errorf("write structural manifest SQLite journal: %w: %s", cause, strings.TrimSpace(store.stderr.String()))
}

func (store *structuralManifestStore) finish() error {
	if store.finished {
		return nil
	}
	store.finished = true
	if _, err := store.writer.WriteString("COMMIT;\n"); err != nil {
		return store.fail(err)
	}
	if err := store.writer.Flush(); err != nil {
		return store.fail(err)
	}
	if err := store.stdin.Close(); err != nil {
		return store.fail(err)
	}
	if err := store.cmd.Wait(); err != nil {
		return fmt.Errorf("finalize structural manifest SQLite journal: %w: %s", err, strings.TrimSpace(store.stderr.String()))
	}
	return nil
}

func (store *structuralManifestStore) writeManifest(path string) error {
	if err := store.finish(); err != nil {
		return err
	}
	query := exec.Command(store.sqlitePath, "-batch", "-noheader", store.path, "SELECT entry FROM entries ORDER BY ordinal;")
	output, err := query.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open structural manifest SQLite output: %w", err)
	}
	var stderr bytes.Buffer
	query.Stderr = &stderr
	if err := query.Start(); err != nil {
		return fmt.Errorf("read structural manifest SQLite journal: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		_ = output.Close()
		_ = query.Wait()
		return fmt.Errorf("create structural manifest: %w", err)
	}
	writer := bufio.NewWriterSize(file, 256*1024)
	_, err = writer.WriteString("{\n  \"schemaVersion\": 1,\n  \"fixtures\": [\n")
	first := true
	reader := bufio.NewReaderSize(output, 256*1024)
	for err == nil {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			if !first {
				_, err = writer.WriteString(",\n")
			}
			if err == nil {
				_, err = writer.WriteString("    " + strings.TrimSuffix(line, "\n"))
			}
			first = false
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			err = readErr
		}
	}
	if err == nil {
		_, err = writer.WriteString("\n  ]\n}\n")
	}
	if flushErr := writer.Flush(); err == nil {
		err = flushErr
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if waitErr := query.Wait(); err == nil {
		err = waitErr
	}
	if err != nil {
		return fmt.Errorf("stream structural manifest from SQLite journal: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (store *structuralManifestStore) remove() error {
	if !store.finished {
		_ = store.stdin.Close()
		_ = store.cmd.Wait()
	}
	return os.Remove(store.path)
}

func resolveGeneratorVersion(explicit string) (string, error) {
	if explicit != "" {
		if !immutableGeneratorVersion.MatchString(explicit) {
			return "", fmt.Errorf("generator version must be git:<40-or-64-lowercase-hex-commit>")
		}
		return explicit, nil
	}
	rootCommand := exec.Command("git", "rev-parse", "--show-toplevel")
	rootOutput, err := rootCommand.Output()
	if err != nil {
		return "", fmt.Errorf("locate generator Git checkout (use --generator-version to provide it explicitly): %w", err)
	}
	return resolveGeneratorVersionInRepo(strings.TrimSpace(string(rootOutput)))
}

func resolveGeneratorVersionInRepo(root string) (string, error) {
	statusArgs := append([]string{"status", "--porcelain", "--untracked-files=all", "--"}, generatorSourcePathspecs...)
	statusCommand := exec.Command("git", statusArgs...)
	statusCommand.Dir = root
	statusOutput, err := statusCommand.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("check generator inputs for local changes: %w: %s", err, strings.TrimSpace(string(statusOutput)))
	}
	if dirty := strings.TrimSpace(string(statusOutput)); dirty != "" {
		return "", fmt.Errorf("refuse automatic generator provenance with dirty generator, manifest-contract, or XSD inputs; commit them or deliberately use --generator-version to override:\n%s", dirty)
	}

	logArgs := append([]string{"log", "-1", "--format=%H", "--"}, generatorSourcePathspecs...)
	command := exec.Command("git", logArgs...)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("resolve generator implementation Git commit (use --generator-version to provide it explicitly): %w", err)
	}
	version := "git:" + strings.TrimSpace(string(output))
	if !immutableGeneratorVersion.MatchString(version) {
		return "", fmt.Errorf("resolved invalid generator Git commit %q", version)
	}
	return version, nil
}

func generateUBLPositiveCorpus(report schemaInventory, index schemaIndex, xsdDir string, count int, generatorVersion string) (generatedCorpus, error) {
	fixtures := make([]generatedFixture, 0, len(report.MainFiles)*count)
	for _, schema := range report.MainFiles {
		root := schema.RootElement
		if root == "" {
			root = strings.TrimSuffix(filepath.Base(schema.File), filepath.Ext(schema.File))
		}
		for sample := 1; sample <= count; sample++ {
			content, err := generateCompleteSample(index, root, xsdDir, schema.File)
			if err != nil {
				return generatedCorpus{}, fmt.Errorf("generate %s sample %d: %w", root, sample, err)
			}
			fileName := fmt.Sprintf("%s-sample-%02d.xml", root, sample)
			relativePath := filepath.ToSlash(filepath.Join(root, fileName))
			fixtureID := "ubl21." + strings.ToLower(root) + ".minimal"
			if sample > 1 {
				fixtureID = fmt.Sprintf("ubl21.%s.sample-%02d", strings.ToLower(root), sample)
			}
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
			fixtures = append(fixtures, generatedFixture{
				entry: fixtureManifestEntry{
					ID: fixtureID, DocumentType: root, Profile: "ubl21", File: relativePath, SHA256: digest,
					Standards:      fixtureStandards{UBL: "2.1"},
					Provenance:     fixtureProvenance{Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion},
					ExpectedFinal:  "valid",
					ExpectedStages: []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "passed"}},
					Tags:           []string{"positive", "ubl21"},
				},
				content: []byte(content), schemaPath: filepath.Join(xsdDir, "maindoc", schema.File),
			})
		}
	}
	return buildGeneratedCorpus(fixtures)
}

// generateAndWriteUBLStructuralCorpus streams each fixture to an isolated
// staging directory. XML is written and manifest metadata is journaled before
// the next fixture is generated, so neither is retained on the Go heap.
func generateAndWriteUBLStructuralCorpus(outputDir string, report schemaInventory, index schemaIndex, xsdDir, generatorVersion, errorJournal, resumeFrom string) error {
	if err := validateCorpusOutputDirectory(outputDir); err != nil {
		return err
	}
	absolute, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolve generated corpus output: %w", err)
	}
	parent := filepath.Dir(absolute)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create generated corpus parent: %w", err)
	}
	staging, err := os.MkdirTemp(parent, "."+filepath.Base(absolute)+".staging-*")
	if err != nil {
		return fmt.Errorf("create generated corpus staging directory: %w", err)
	}
	defer os.RemoveAll(staging)
	manifestStore, err := newStructuralManifestStore(filepath.Join(staging, ".manifest.sqlite"))
	if err != nil {
		return err
	}
	defer manifestStore.remove()
	generated := 0
	fmt.Printf("Structural corpus: streaming validated fixtures to staging directory\n")
	sink := func(fixture generatedFixture) error {
		path := filepath.Join(staging, filepath.FromSlash(fixture.entry.File))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, fixture.content, 0o644); err != nil {
			return err
		}
		generated++
		if generated%100 == 0 {
			fmt.Printf("Structural corpus: generated and validated %d fixtures\n", generated)
		}
		return nil
	}
	err = generateUBLStructuralCorpusWithSink(report, index, xsdDir, generatorVersion, sink, manifestStore, errorJournal, resumeFrom)
	if err != nil {
		return err
	}
	if resumeFrom != "" {
		fmt.Printf("Structural resume from %q completed without new failures; no partial corpus was installed. Rerun without --resume-from to regenerate and promote the complete corpus.\n", resumeFrom)
		return nil
	}
	if err := os.WriteFile(filepath.Join(staging, corpusOwnershipMarkerName), []byte(corpusOwnershipMarkerContent), 0o644); err != nil {
		return err
	}
	if err := manifestStore.writeManifest(filepath.Join(staging, "manifest.json")); err != nil {
		return err
	}
	if err := manifestStore.remove(); err != nil {
		return fmt.Errorf("remove structural manifest SQLite journal: %w", err)
	}
	fmt.Printf("Structural corpus: wrote manifest for %d fixtures; installing corpus\n", generated)
	if _, err := os.Stat(absolute); err == nil {
		backup, err := os.MkdirTemp(parent, "."+filepath.Base(absolute)+".backup-*")
		if err != nil {
			return err
		}
		if err := os.Remove(backup); err != nil {
			return err
		}
		if err := os.Rename(absolute, backup); err != nil {
			return err
		}
		if err := os.Rename(staging, absolute); err != nil {
			_ = os.Rename(backup, absolute)
			return err
		}
		return os.RemoveAll(backup)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(staging, absolute)
}

// generateUBLStructuralCorpusWithSink sends each validated structural fixture
// to its output sink and manifest journal before generating the next one.
func generateUBLStructuralCorpusWithSink(report schemaInventory, index schemaIndex, xsdDir, generatorVersion string, sink func(generatedFixture) error, manifestStore *structuralManifestStore, errorJournal, resumeFrom string) error {
	resumePhase, resumeRoot, err := parseStructuralResumeCursor(resumeFrom)
	if err != nil {
		return err
	}
	resumeStarted := resumeFrom == ""
	var problems []string
	journal := func(err error) { problems = append(problems, err.Error()) }
	finishWithJournal := func() error {
		if len(problems) == 0 {
			return nil
		}
		if errorJournal == "" {
			return fmt.Errorf("structural generation completed with %d failures", len(problems))
		}
		if err := os.MkdirAll(filepath.Dir(errorJournal), 0o755); err != nil {
			return fmt.Errorf("write structural error journal: create directory: %w", err)
		}
		content := "Structural corpus generation failures:\n\n" + strings.Join(problems, "\n\n") + "\n"
		if err := os.WriteFile(errorJournal, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write structural error journal: %w", err)
		}
		return fmt.Errorf("structural generation completed with %d failures; see %s", len(problems), errorJournal)
	}
	choiceVariants := maxChoiceAlternatives(index)
	if choiceVariants < 1 {
		choiceVariants = 1
	}
	record := func(fixture generatedFixture) error {
		if sink != nil {
			if err := sink(fixture); err != nil {
				return err
			}
			fixture.content = nil
		}
		if manifestStore == nil {
			return errors.New("structural manifest store is required")
		}
		return manifestStore.add(fixture.entry)
	}
	repeatableTargets := schemaCoverageTargets(index)
	cardinalityOwners := map[string]struct{}{}
	for id := range repeatableTargets.repeatable {
		parts := strings.Split(id, "|")
		if len(parts) != 4 {
			return fmt.Errorf("invalid repeatable target %q", id)
		}
		owner, _, _ := strings.Cut(parts[3], "/")
		cardinalityOwners[qualifiedKey(parts[0], owner)] = struct{}{}
	}
	for _, schema := range report.MainFiles {
		root := schema.RootElement
		if root == "" {
			root = strings.TrimSuffix(filepath.Base(schema.File), filepath.Ext(schema.File))
		}
		if resumePhase == "signature" || resumePhase == "cardinality" {
			continue
		}
		if !resumeStarted {
			if root != resumeRoot {
				continue
			}
			resumeStarted = true
		}
		choicePlans, err := structuralChoicePlans(index, root, xsdDir, schema.File)
		if err != nil {
			journal(fmt.Errorf("count %s choice sites: %w", root, err))
			continue
		}
		add := func(family string, choices map[int]int, recursiveDepth, facetBoundary, repeatCount int, repeatElement string, repeatElements map[string]int, tags ...string) error {
			var content string
			var err error
			if len(repeatElements) > 0 {
				content, err = generateLargeCollectionsStructuralSample(index, root, xsdDir, schema.File, repeatElements)
			} else if repeatElement != "" {
				content, err = generateLargeCollectionStructuralSample(index, root, xsdDir, schema.File, repeatElement, repeatCount)
			} else if repeatCount > 0 {
				content, err = generateCardinalityStructuralSample(index, root, xsdDir, schema.File, repeatCount)
			} else if facetBoundary > 0 {
				content, err = generateFacetBoundaryStructuralSample(index, root, xsdDir, schema.File, facetBoundary)
			} else {
				content, err = generateTargetedStructuralSample(index, root, xsdDir, schema.File, choices, recursiveDepth)
			}
			if err != nil {
				return fmt.Errorf("generate %s %s: %w", root, family, err)
			}
			if err := validateStructuralCandidate(content, filepath.Join(xsdDir, "maindoc", schema.File)); err != nil {
				return fmt.Errorf("validate %s %s: %w", root, family, err)
			}
			if err := record(generatedFixture{entry: fixtureManifestEntry{
				ID:             fmt.Sprintf("ubl21.%s.%s", strings.ToLower(root), family),
				DocumentType:   root,
				Profile:        "ubl21",
				File:           filepath.ToSlash(filepath.Join(root, fmt.Sprintf("%s-%s.xml", root, family))),
				SHA256:         fmt.Sprintf("%x", sha256.Sum256([]byte(content))),
				Standards:      fixtureStandards{UBL: "2.1"},
				Provenance:     fixtureProvenance{Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion},
				ExpectedFinal:  "valid",
				ExpectedStages: []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "passed"}},
				Tags:           append([]string{"positive", "ubl21", "structural", family}, tags...),
				// The manifest records the stable intended schema-root/family
				// targets.  The complete fixture-to-schema-target mapping belongs
				// in coverage.json; duplicating it in every manifest entry makes a
				// versioned corpus needlessly enormous.
				StructuralTargets: []string{"schema-root:" + root, "family:" + family},
			}, content: []byte(content), schemaPath: filepath.Join(xsdDir, "maindoc", schema.File)}); err != nil {
				return err
			}
			return nil
		}
		minimal, err := generateMinimalSample(index, root, xsdDir, schema.File)
		if err != nil {
			journal(fmt.Errorf("generate %s minimal: %w", root, err))
		} else if err := record(generatedFixture{entry: fixtureManifestEntry{
			ID:                fmt.Sprintf("ubl21.%s.minimal", strings.ToLower(root)),
			DocumentType:      root,
			Profile:           "ubl21",
			File:              filepath.ToSlash(filepath.Join(root, fmt.Sprintf("%s-minimal.xml", root))),
			SHA256:            fmt.Sprintf("%x", sha256.Sum256([]byte(minimal))),
			Standards:         fixtureStandards{UBL: "2.1"},
			Provenance:        fixtureProvenance{Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion},
			ExpectedFinal:     "valid",
			ExpectedStages:    []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "passed"}},
			Tags:              []string{"positive", "ubl21", "structural", "minimal", "optional-absent", "cardinality", "occurrences-0"},
			StructuralTargets: []string{"schema-root:" + root, "family:minimal"},
		}, content: []byte(minimal), schemaPath: filepath.Join(xsdDir, "maindoc", schema.File)}); err != nil {
			journal(err)
		}
		if err := add("optional-rich", nil, 1, 0, 0, "", nil, "optional-elements", "optional-attributes", "cardinality", "occurrences-1"); err != nil {
			journal(err)
		}
		// Exercise nested cardinality contracts one particle at a time.  The
		// ordinary cardinality families deliberately limit themselves to the
		// document root because repeating every nested collection at once grows
		// exponentially.  Starting from the same full structural expansion used
		// by optional-rich gives us the particles this document can reach; an
		// override then repeats just one element, preserving a small, XSD-valid
		// fixture for each required non-singleton count.
		baseline, err := generateTargetedStructuralSample(index, root, xsdDir, schema.File, nil, 1)
		if err != nil {
			journal(fmt.Errorf("generate %s cardinality baseline: %w", root, err))
			continue
		}
		matched, err := matchFixtureDetailedReader(index, strings.NewReader(baseline))
		if err != nil {
			journal(fmt.Errorf("match %s cardinality baseline: %w", root, err))
			continue
		}
		reachable := make([]string, 0, len(matched.cardinalities))
		for id := range matched.cardinalities {
			if _, ok := repeatableTargets.repeatable[id]; ok {
				reachable = append(reachable, id)
			}
		}
		sort.Strings(reachable)
		planned := map[string]map[int]struct{}{}
		for _, id := range reachable {
			parts := strings.Split(id, "|")
			if len(parts) != 4 {
				return fmt.Errorf("invalid repeatable target %q", id)
			}
			element := filepath.Base(parts[3])
			if planned[element] == nil {
				planned[element] = map[int]struct{}{}
			}
			for _, count := range requiredCardinalities(repeatableTargets.repeatableBounds[id]) {
				// The baseline covers the singleton case and the minimal family
				// covers optional zero.  Only add fixtures that demonstrate an
				// additional cardinality contract.
				if count <= 1 {
					continue
				}
				planned[element][count] = struct{}{}
			}
		}
		elements := make([]string, 0, len(planned))
		for element := range planned {
			elements = append(elements, element)
		}
		sort.Strings(elements)
		for _, element := range elements {
			counts := make([]int, 0, len(planned[element]))
			for count := range planned[element] {
				counts = append(counts, count)
			}
			sort.Ints(counts)
			for _, count := range counts {
				family := fmt.Sprintf("cardinality-target-%s-%d", strings.ToLower(element), count)
				if err := add(family, nil, 1, 0, count, element, nil, "cardinality", "targeted", fmt.Sprintf("occurrences-%d", count)); err != nil {
					journal(err)
				}
			}
		}
		// XMLDSig/XAdES is reachable only through UBL's extension wildcard.
		// Keep a focused declared-signature fixture alongside the generic foreign
		// extension payload so that schema choices are measured against real UBL
		// extension content rather than an invented private namespace.
		if root == "Invoice" {
			for variant, family := range []string{
				"key-name", "key-name-transform-wildcard",
				"rsa-key-value", "rsa-key-value-transform-wildcard",
				"retrieval-method", "retrieval-method-transform-wildcard",
				"x509-subject-name", "x509-subject-name-transform-wildcard",
				"pgp-key-id", "pgp-key-id-transform-wildcard",
				"pgp-key-packet", "pgp-key-packet-transform-wildcard",
				"spki-data", "spki-data-transform-wildcard",
				"management-data", "management-data-transform-wildcard",
				"key-info-wildcard", "key-info-wildcard-transform-wildcard",
				"dsa-key-value", "dsa-key-value-transform-wildcard",
				"key-value-wildcard", "key-value-wildcard-transform-wildcard",
				"x509-issuer-serial", "x509-issuer-serial-transform-wildcard",
				"x509-ski", "x509-ski-transform-wildcard",
				"x509-certificate", "x509-certificate-transform-wildcard",
				"x509-crl", "x509-crl-transform-wildcard",
				"x509-wildcard", "x509-wildcard-transform-wildcard",
			} {
				content, err := generateSignatureExtensionStructuralSample(index, root, xsdDir, schema.File, variant)
				if err != nil {
					journal(fmt.Errorf("generate %s signature extension %s: %w", root, family, err))
					continue
				}
				if err := validateStructuralCandidate(content, filepath.Join(xsdDir, "maindoc", schema.File)); err != nil {
					journal(fmt.Errorf("validate %s signature extension %s: %w", root, family, err))
					continue
				}
				if err := record(generatedFixture{entry: fixtureManifestEntry{
					ID:                "ubl21.invoice.signature-extension-" + family,
					DocumentType:      root,
					Profile:           "ubl21",
					File:              filepath.ToSlash(filepath.Join(root, "Invoice-signature-extension-"+family+".xml")),
					SHA256:            fmt.Sprintf("%x", sha256.Sum256([]byte(content))),
					Standards:         fixtureStandards{UBL: "2.1"},
					Provenance:        fixtureProvenance{Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion},
					ExpectedFinal:     "valid",
					ExpectedStages:    []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "passed"}},
					Tags:              []string{"positive", "ubl21", "structural", "signature-extension", "choice-alternative", "wildcard", family},
					StructuralTargets: []string{"schema-root:" + root, "family:signature-extension", "signature-variant:" + family},
				}, content: []byte(content), schemaPath: filepath.Join(xsdDir, "maindoc", schema.File)}); err != nil {
					return err
				}
			}
		}
		for planIndex, choices := range choicePlans[1:] {
			if err := add(fmt.Sprintf("choice-path-%03d", planIndex+1), choices, 1, 0, 0, "", nil, "choice-alternative", "targeted"); err != nil {
				journal(err)
			}
		}
		for boundary, family := range []string{"facet-boundary-low", "facet-boundary-high"} {
			if err := add(family, nil, 1, boundary+1, 0, "", nil, "facet-boundary"); err != nil {
				journal(err)
			}
		}
		for _, recursiveDepth := range []int{2, 3} {
			if err := add(fmt.Sprintf("recursive-depth-%d", recursiveDepth), nil, recursiveDepth, 0, 0, "", nil, "recursive"); err != nil {
				journal(err)
			}
		}
		for _, count := range []int{2, 5} {
			if err := add(fmt.Sprintf("cardinality-%d", count), nil, 1, 0, count, "", nil, "cardinality", fmt.Sprintf("occurrences-%d", count)); err != nil {
				journal(err)
			}
		}
		// Nested repeatable particles are exercised one target at a time by the
		// cardinality-target fixtures above. Repeating every nested particle in
		// one document multiplies shared aggregate content and creates hundreds
		// of thousands of fixtures across the UBL roots.
		// These fixtures exercise the progressive high-volume collection tiers
		// seen by the real service without making every repeatable branch of a
		// document enormous.  The smaller tiers make it possible to locate
		// cardinality regressions before the several-hundred-line fixtures.
		for _, large := range []struct {
			document string
			element  string
			count    int
			kind     string
		}{
			{"Invoice", "InvoiceLine", 10, "line-collection"}, {"Invoice", "InvoiceLine", 50, "line-collection"}, {"Invoice", "InvoiceLine", 100, "line-collection"}, {"Invoice", "InvoiceLine", 500, "line-collection"},
			{"CreditNote", "CreditNoteLine", 10, "line-collection"}, {"CreditNote", "CreditNoteLine", 50, "line-collection"}, {"CreditNote", "CreditNoteLine", 100, "line-collection"}, {"CreditNote", "CreditNoteLine", 250, "line-collection"},
			{"Order", "OrderLine", 10, "line-collection"}, {"Order", "OrderLine", 50, "line-collection"}, {"Order", "OrderLine", 100, "line-collection"}, {"Order", "OrderLine", 250, "line-collection"},
			// Non-line collections exercise the aggregate-heavy paths that have
			// historically been easy to overlook in document-size testing.
			{"Invoice", "AllowanceCharge", 50, "aggregate-collection"}, {"Invoice", "TaxTotal", 50, "aggregate-collection"}, {"Invoice", "BillingReference", 50, "aggregate-collection"},
			{"CreditNote", "AllowanceCharge", 50, "aggregate-collection"}, {"CreditNote", "TaxTotal", 50, "aggregate-collection"}, {"CreditNote", "BillingReference", 50, "aggregate-collection"},
			{"Order", "AllowanceCharge", 50, "aggregate-collection"}, {"Order", "TaxTotal", 50, "aggregate-collection"}, {"Order", "ProjectReference", 50, "aggregate-collection"},
			// AdditionalDocumentReference carries an Attachment; FreightInvoice
			// is the main-document root with a repeatable Shipment collection.
			{"Invoice", "AdditionalDocumentReference", 50, "attachment-collection"}, {"Order", "AdditionalDocumentReference", 50, "attachment-collection"},
			{"FreightInvoice", "Shipment", 50, "shipment-collection"},
			// A CatalogueLine carries its Item content, making this a practical
			// high-cardinality item-bearing document without violating Item's
			// own single-occurrence particle in each line.
			{"Catalogue", "CatalogueLine", 100, "item-collection"},
		} {
			if root != large.document {
				continue
			}
			if err := add(fmt.Sprintf("%s-%03d", strings.ToLower(large.element), large.count), nil, 1, 0, large.count, large.element, nil, "cardinality", "large-document", large.kind); err != nil {
				journal(err)
			}
		}
		if root == "Invoice" {
			if err := add("multi-collection-100", nil, 1, 0, 0, "", map[string]int{"InvoiceLine": 100, "AllowanceCharge": 50, "TaxTotal": 50}, "cardinality", "large-document", "line-collection", "aggregate-collection", "multi-collection"); err != nil {
				journal(err)
			}
		}
	}
	// These CAC global declarations are not referenced by any UBL main-document
	// root, but remain part of the published common-component schema. Exercise
	// each as a standalone schema root so declaration coverage does not depend
	// on inventing an unrelated business-document path to it.
	if resumePhase != "signature" && resumePhase != "cardinality" {
		for _, root := range []string{"Branch", "Communication", "Duty", "MonetaryTotal", "Payment", "Regulation", "ServiceFrequency"} {
			fixture, err := generateCommonComponentStructuralFixture(index, xsdDir, generatorVersion, root)
			if err != nil {
				journal(err)
				continue
			}
			if err := record(fixture); err != nil {
				return err
			}
		}
	}
	// XMLDSig and XAdES declarations are schema-reachable through the UBL
	// signature extension, but many are not reachable through its deliberately
	// small hand-authored payload. Generate their global roots directly so the
	// same schema matcher can qualify those contracts without fabricating them
	// inside a business document.
	signatureNamespaces := map[string]string{
		"http://www.w3.org/2000/09/xmldsig#": "xmldsig",
		"http://uri.etsi.org/01903/v1.3.2#":  "xades",
	}
	type signatureRoot struct{ namespace, name string }
	var roots []signatureRoot
	for key := range index.GlobalElements {
		namespace, name, ok := splitQualifiedKey(key)
		if !ok {
			return fmt.Errorf("invalid global element key %q", key)
		}
		if _, ok := signatureNamespaces[namespace]; ok {
			roots = append(roots, signatureRoot{namespace, name})
		}
	}
	sort.Slice(roots, func(i, j int) bool {
		if roots[i].namespace == roots[j].namespace {
			return roots[i].name < roots[j].name
		}
		return roots[i].namespace < roots[j].namespace
	})
	includePGPDataChoice := resumePhase == "" || resumePhase == "main"
	for _, root := range roots {
		if resumePhase == "cardinality" {
			continue
		}
		if resumePhase == "signature" && !resumeStarted {
			if root.name != resumeRoot {
				continue
			}
			resumeStarted = true
		}
		doc := index.GlobalElementDocs[qualifiedKey(root.namespace, root.name)]
		if doc == nil {
			continue
		}
		if root.namespace == "http://www.w3.org/2000/09/xmldsig#" && root.name == "PGPData" {
			includePGPDataChoice = true
		}
		state := newTargetedStructuralGenState(nil, 1)
		state.signatureExtension = true
		content, err := generateSampleWithState(index, root.name, xsdDir, doc.File, state)
		if err != nil {
			journal(fmt.Errorf("generate signature root %s: %w", root.name, err))
			continue
		}
		schemaPath := filepath.Join(xsdDir, "common", doc.File)
		if err := validateStructuralCandidate(content, schemaPath); err != nil {
			journal(fmt.Errorf("validate signature root %s: %w", root.name, err))
			continue
		}
		family := signatureNamespaces[root.namespace] + "-root-" + strings.ToLower(root.name)
		if err := record(generatedFixture{entry: fixtureManifestEntry{
			ID: "ubl21.signature." + family, DocumentType: root.name, Profile: "ubl21",
			File:   filepath.ToSlash(filepath.Join("SignatureSchemas", family+".xml")),
			SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(content))), Standards: fixtureStandards{UBL: "2.1"},
			Provenance:    fixtureProvenance{Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion},
			ExpectedFinal: "valid", ExpectedStages: []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "passed"}},
			Tags:              []string{"positive", "ubl21", "structural", "signature-schema", signatureNamespaces[root.namespace]},
			StructuralTargets: []string{"signature-root:" + root.name},
		}, content: []byte(content), schemaPath: schemaPath}); err != nil {
			return err
		}
	}
	if includePGPDataChoice {
		fixture, err := generateSignatureChoiceStructuralFixture(index, xsdDir, generatorVersion, "PGPData", 1)
		if err != nil {
			journal(err)
		} else if err := record(fixture); err != nil {
			return err
		}
	}
	// Keep signature cardinality witnesses bounded per repeatable schema target.
	// Exhaustively combining nested XMLDSig/XAdES choices produced tens of
	// thousands of plans and hundreds of thousands of redundant fixtures.
	coveredSignatureCardinalities := map[string]map[int]struct{}{}
	emittedSignatureZeroCardinalities := map[string]struct{}{}
	for _, root := range roots {
		doc := index.GlobalElementDocs[qualifiedKey(root.namespace, root.name)]
		if doc == nil {
			continue
		}
		probe, err := generateTargetedStructuralSample(index, root.name, xsdDir, doc.File, nil, 1)
		if err != nil {
			journal(fmt.Errorf("probe signature cardinality root %s: %w", root.name, err))
			continue
		}
		probeMatch, err := matchFixtureDetailedReader(index, strings.NewReader(probe))
		if err != nil {
			journal(fmt.Errorf("match signature cardinality root %s: %w", root.name, err))
			continue
		}
		relevant := false
		for id := range probeMatch.cardinalities {
			if _, ok := repeatableTargets.repeatable[id]; ok {
				relevant = true
				break
			}
		}
		if !relevant {
			continue
		}
		plans, err := structuralChoicePlans(index, root.name, xsdDir, doc.File)
		if err != nil {
			journal(fmt.Errorf("plan signature cardinality root %s: %w", root.name, err))
			continue
		}
		for planIndex, choices := range plans {
			baseline, err := generateTargetedStructuralSample(index, root.name, xsdDir, doc.File, choices, 1)
			if err != nil {
				journal(fmt.Errorf("generate signature cardinality baseline %s: %w", root.name, err))
				continue
			}
			matched, err := matchFixtureDetailedReader(index, strings.NewReader(baseline))
			if err != nil {
				journal(fmt.Errorf("match signature cardinality baseline %s: %w", root.name, err))
				continue
			}
			cardinalityIDs := make([]string, 0, len(matched.cardinalities))
			for id := range matched.cardinalities {
				cardinalityIDs = append(cardinalityIDs, id)
			}
			sort.Strings(cardinalityIDs)
			for _, id := range cardinalityIDs {
				values := matched.cardinalities[id]
				_, isRepeatable := repeatableTargets.repeatable[id]
				if !isRepeatable {
					continue
				}
				needsZero := false
				for _, required := range requiredCardinalities(repeatableTargets.repeatableBounds[id]) {
					if required != 0 {
						continue
					}
					if _, ok := emittedSignatureZeroCardinalities[id]; !ok {
						needsZero = true
					}
				}
				hasPositiveCount := false
				for _, value := range values {
					if value > 0 {
						hasPositiveCount = true
						break
					}
				}
				if !needsZero || !hasPositiveCount {
					continue
				}
				parts := strings.Split(id, "|")
				if len(parts) != 4 {
					return fmt.Errorf("invalid repeatable target %q", id)
				}
				element := filepath.Base(parts[3])
				content, err := generateStructuralSampleOmittingChoices(index, root.name, xsdDir, doc.File, choices, element)
				if err != nil {
					journal(fmt.Errorf("generate signature cardinality zero %s plan %d: %w", root.name, planIndex, err))
					continue
				}
				observed, err := matchFixtureDetailedReader(index, strings.NewReader(content))
				if err != nil {
					journal(fmt.Errorf("match signature cardinality zero %s plan %d: %w", root.name, planIndex, err))
					continue
				}
				parentInstances := observed.parentInstances[particleParentComplexTypeID(id)]
				zeroObserved := parentInstances > len(observed.cardinalities[id])
				if !zeroObserved {
					continue
				}
				schemaPath := schemaPathForDocument(xsdDir, doc.File)
				if err := validateStructuralCandidate(content, schemaPath); err != nil {
					journal(fmt.Errorf("validate signature cardinality zero %s plan %d: %w", root.name, planIndex, err))
					continue
				}
				family := signatureCardinalityZeroFamily(planIndex, id)
				if err := record(generatedFixture{entry: fixtureManifestEntry{
					ID: fmt.Sprintf("ubl21.signature.%s.%s", strings.ToLower(root.name), family), DocumentType: root.name, Profile: "ubl21",
					File: filepath.ToSlash(filepath.Join("SignatureSchemas", strings.ToLower(root.name)+"-"+family+".xml")), SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(content))),
					Standards: fixtureStandards{UBL: "2.1"}, Provenance: fixtureProvenance{Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion}, ExpectedFinal: "valid", ExpectedStages: []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "passed"}},
					Tags: []string{"positive", "ubl21", "structural", "signature-schema", "cardinality", "occurrences-0"}, StructuralTargets: []string{"signature-root:" + root.name, "family:" + family},
				}, content: []byte(content), schemaPath: schemaPath}); err != nil {
					return err
				}
				if coveredSignatureCardinalities[id] == nil {
					coveredSignatureCardinalities[id] = map[int]struct{}{}
				}
				coveredSignatureCardinalities[id][0] = struct{}{}
				emittedSignatureZeroCardinalities[id] = struct{}{}
			}
			baselineImproved := false
			needsDeepCardinality := false
			for id, values := range matched.cardinalities {
				if _, isRepeatable := repeatableTargets.repeatable[id]; !isRepeatable {
					continue
				}
				targetPresent := false
				for _, value := range values {
					if value > 0 {
						targetPresent = true
					}
					for _, required := range requiredCardinalities(repeatableTargets.repeatableBounds[id]) {
						if value != required {
							continue
						}
						if _, ok := coveredSignatureCardinalities[id][value]; !ok {
							if value == 1 || value == 0 {
								baselineImproved = true
							} else if value > 1 {
								needsDeepCardinality = true
							}
						}
					}
				}
				if targetPresent {
					for _, required := range requiredCardinalities(repeatableTargets.repeatableBounds[id]) {
						if required <= 1 {
							continue
						}
						if _, ok := coveredSignatureCardinalities[id][required]; !ok {
							needsDeepCardinality = true
						}
					}
				}
			}
			if baselineImproved {
				schemaPath := schemaPathForDocument(xsdDir, doc.File)
				if err := validateStructuralCandidate(baseline, schemaPath); err != nil {
					journal(fmt.Errorf("validate signature cardinality baseline %s plan %d: %w", root.name, planIndex, err))
				} else {
					family := fmt.Sprintf("signature-cardinality-plan-%05d-1", planIndex)
					if err := record(generatedFixture{entry: fixtureManifestEntry{
						ID: fmt.Sprintf("ubl21.signature.%s.%s", strings.ToLower(root.name), family), DocumentType: root.name, Profile: "ubl21",
						File: filepath.ToSlash(filepath.Join("SignatureSchemas", strings.ToLower(root.name)+"-"+family+".xml")), SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(baseline))),
						Standards: fixtureStandards{UBL: "2.1"}, Provenance: fixtureProvenance{Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion}, ExpectedFinal: "valid", ExpectedStages: []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "passed"}},
						Tags: []string{"positive", "ubl21", "structural", "signature-schema", "cardinality", "occurrences-1"}, StructuralTargets: []string{"signature-root:" + root.name, "family:" + family},
					}, content: []byte(baseline), schemaPath: schemaPath}); err != nil {
						return err
					}
					for id, values := range matched.cardinalities {
						if _, ok := repeatableTargets.repeatable[id]; !ok {
							continue
						}
						if coveredSignatureCardinalities[id] == nil {
							coveredSignatureCardinalities[id] = map[int]struct{}{}
						}
						for _, value := range values {
							for _, required := range requiredCardinalities(repeatableTargets.repeatableBounds[id]) {
								if value == required {
									coveredSignatureCardinalities[id][value] = struct{}{}
								}
							}
						}
					}
				}
			}
			if !needsDeepCardinality {
				continue
			}
			for _, count := range []int{2, 5} {
				content, err := generateDeepCardinalityStructuralSample(index, root.name, xsdDir, doc.File, choices, count)
				if err != nil {
					journal(fmt.Errorf("generate signature cardinality root %s plan %d count %d: %w", root.name, planIndex, count, err))
					continue
				}
				observed, err := matchFixtureDetailedReader(index, strings.NewReader(content))
				if err != nil {
					journal(fmt.Errorf("match signature cardinality root %s plan %d count %d: %w", root.name, planIndex, count, err))
					continue
				}
				improved := false
				for id, values := range observed.cardinalities {
					if _, isRepeatable := repeatableTargets.repeatable[id]; !isRepeatable {
						continue
					}
					if coveredSignatureCardinalities[id] == nil {
						coveredSignatureCardinalities[id] = map[int]struct{}{}
					}
					for _, value := range values {
						if value <= 1 {
							continue
						}
						for _, required := range requiredCardinalities(repeatableTargets.repeatableBounds[id]) {
							if value == required {
								if _, ok := coveredSignatureCardinalities[id][value]; !ok {
									improved = true
								}
							}
						}
					}
				}
				if !improved {
					continue
				}
				schemaPath := schemaPathForDocument(xsdDir, doc.File)
				if err := validateStructuralCandidate(content, schemaPath); err != nil {
					journal(fmt.Errorf("validate signature cardinality root %s plan %d count %d: %w", root.name, planIndex, count, err))
					continue
				}
				family := fmt.Sprintf("signature-cardinality-plan-%05d-%d", planIndex, count)
				if err := record(generatedFixture{entry: fixtureManifestEntry{
					ID: fmt.Sprintf("ubl21.signature.%s.%s", strings.ToLower(root.name), family), DocumentType: root.name, Profile: "ubl21",
					File: filepath.ToSlash(filepath.Join("SignatureSchemas", strings.ToLower(root.name)+"-"+family+".xml")), SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(content))),
					Standards: fixtureStandards{UBL: "2.1"}, Provenance: fixtureProvenance{Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion}, ExpectedFinal: "valid", ExpectedStages: []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "passed"}},
					Tags: []string{"positive", "ubl21", "structural", "signature-schema", "cardinality", fmt.Sprintf("occurrences-%d", count)}, StructuralTargets: []string{"signature-root:" + root.name, "family:" + family},
				}, content: []byte(content), schemaPath: schemaPath}); err != nil {
					return err
				}
				for id, values := range observed.cardinalities {
					if _, isRepeatable := repeatableTargets.repeatable[id]; !isRepeatable {
						continue
					}
					if coveredSignatureCardinalities[id] == nil {
						coveredSignatureCardinalities[id] = map[int]struct{}{}
					}
					for _, value := range values {
						if value > 1 {
							coveredSignatureCardinalities[id][value] = struct{}{}
						}
					}
				}
			}
		}
	}
	// A main-document expansion intentionally demotes repeated aggregate types
	// after their first full visit.  That keeps ordinary fixtures bounded, but
	// it also means a repeatable particle owned by one of those types might
	// never be a direct child of an instantiated parent.  Generate focused
	// roots for every globally-declared type that owns a repeatable particle.
	// At a root the generic cardinality state applies directly to that type's
	// particles, exercising the zero, two and capped-unbounded (five) cases
	// without multiplying unrelated nested collections.
	type cardinalityRoot struct{ namespace, name string }
	var cardinalityRoots []cardinalityRoot
	for key, declaration := range index.GlobalElements {
		doc := index.GlobalElementDocs[key]
		if doc == nil || declaration.Type == "" {
			continue
		}
		typeNamespace, typeName := qualifyRef(declaration.Type, doc.TargetNamespace)
		if _, ok := cardinalityOwners[qualifiedKey(typeNamespace, typeName)]; !ok {
			continue
		}
		namespace, name, ok := splitQualifiedKey(key)
		if !ok {
			return fmt.Errorf("invalid global element key %q", key)
		}
		cardinalityRoots = append(cardinalityRoots, cardinalityRoot{namespace: namespace, name: name})
	}
	sort.Slice(cardinalityRoots, func(i, j int) bool {
		if cardinalityRoots[i].namespace == cardinalityRoots[j].namespace {
			return cardinalityRoots[i].name < cardinalityRoots[j].name
		}
		return cardinalityRoots[i].namespace < cardinalityRoots[j].namespace
	})
	for _, root := range cardinalityRoots {
		if resumePhase == "cardinality" && !resumeStarted {
			if root.name != resumeRoot {
				continue
			}
			resumeStarted = true
		}
		doc := index.GlobalElementDocs[qualifiedKey(root.namespace, root.name)]
		if doc == nil {
			continue
		}
		rootKey := namespaceFixtureKey(root.namespace)
		for _, variant := range []struct {
			name  string
			count int
		}{
			{name: "minimal"},
			{name: "cardinality-1", count: 1},
			{name: "cardinality-2", count: 2},
			{name: "cardinality-5", count: 5},
		} {
			var content string
			var err error
			if variant.count == 0 {
				// This is the zero-occurrence witness for optional repeatable
				// particles.  A structural cardinality sample emits optional
				// elements at least once, so using one here silently turned this
				// supposed minimal fixture into another singleton fixture.
				switch root.name {
				case "CommitmentTypeIndication":
					content, err = generateStructuralSampleOmitting(index, root.name, xsdDir, doc.File, "CommitmentTypeQualifier")
				case "SPUserNotice":
					content, err = generateStructuralSampleOmitting(index, root.name, xsdDir, doc.File, "int")
				default:
					content, err = generateMinimalStructuralSample(index, root.name, xsdDir, doc.File)
				}
			} else {
				content, err = generateCardinalityStructuralSample(index, root.name, xsdDir, doc.File, variant.count)
			}
			if err != nil {
				journal(fmt.Errorf("generate cardinality root %s %s: %w", root.name, variant.name, err))
				continue
			}
			schemaPath := schemaPathForDocument(xsdDir, doc.File)
			if err := validateStructuralCandidate(content, schemaPath); err != nil {
				journal(fmt.Errorf("validate cardinality root %s %s: %w", root.name, variant.name, err))
				continue
			}
			family := "cardinality-root-" + variant.name
			if err := record(generatedFixture{entry: fixtureManifestEntry{
				ID: "ubl21.cardinality." + rootKey + "." + strings.ToLower(root.name) + "." + variant.name, DocumentType: root.name, Profile: "ubl21",
				File:   filepath.ToSlash(filepath.Join("CardinalityRoots", rootKey, root.name+"-"+variant.name+".xml")),
				SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(content))), Standards: fixtureStandards{UBL: "2.1"},
				Provenance:    fixtureProvenance{Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion},
				ExpectedFinal: "valid", ExpectedStages: []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "passed"}},
				Tags:              []string{"positive", "ubl21", "structural", "cardinality", "cardinality-root", variant.name},
				StructuralTargets: []string{"schema-root:" + root.name, "family:" + family},
			}, content: []byte(content), schemaPath: schemaPath}); err != nil {
				return err
			}
		}
	}
	if resumeFrom != "" && !resumeStarted {
		return fmt.Errorf("resume cursor %q did not match a generated structural root", resumeFrom)
	}
	if err := finishWithJournal(); err != nil {
		return err
	}
	return nil
}

// schemaPathForDocument resolves both global common-component roots and
// main-document roots. Cardinality coverage intentionally includes both.
func schemaPathForDocument(xsdDir, file string) string {
	common := filepath.Join(xsdDir, "common", file)
	if _, err := os.Stat(common); err == nil {
		return common
	}
	return filepath.Join(xsdDir, "maindoc", file)
}

func generateCommonComponentStructuralFixture(index schemaIndex, xsdDir, generatorVersion, root string) (generatedFixture, error) {
	const namespace = "urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
	doc := index.GlobalElementDocs[qualifiedKey(namespace, root)]
	if doc == nil {
		return generatedFixture{}, fmt.Errorf("common component root %s is not declared", root)
	}
	content, err := generateTargetedStructuralSample(index, root, xsdDir, doc.File, nil, 2)
	if err != nil {
		return generatedFixture{}, fmt.Errorf("generate common component root %s: %w", root, err)
	}
	schemaPath := schemaPathForDocument(xsdDir, doc.File)
	if err := validateStructuralCandidate(content, schemaPath); err != nil {
		return generatedFixture{}, fmt.Errorf("validate common component root %s: %w", root, err)
	}
	family := "common-component-root-" + strings.ToLower(root)
	return generatedFixture{entry: fixtureManifestEntry{
		ID: "ubl21.component." + strings.ToLower(root), DocumentType: root, Profile: "ubl21",
		File:   filepath.ToSlash(filepath.Join("CommonComponents", root+".xml")),
		SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(content))), Standards: fixtureStandards{UBL: "2.1"},
		Provenance:    fixtureProvenance{Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion},
		ExpectedFinal: "valid", ExpectedStages: []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "passed"}},
		Tags:              []string{"positive", "ubl21", "structural", "common-component", "standalone-root"},
		StructuralTargets: []string{"component-root:" + root, "family:" + family},
	}, content: []byte(content), schemaPath: schemaPath}, nil
}

func generateSignatureChoiceStructuralFixture(index schemaIndex, xsdDir, generatorVersion, root string, alternative int) (generatedFixture, error) {
	const namespace = "http://www.w3.org/2000/09/xmldsig#"
	doc := index.GlobalElementDocs[qualifiedKey(namespace, root)]
	if doc == nil {
		return generatedFixture{}, fmt.Errorf("XMLDSig choice root %s is not declared", root)
	}
	state := newTargetedStructuralGenState(map[int]int{0: alternative}, 1)
	content, err := generateSampleWithState(index, root, xsdDir, doc.File, state)
	if err != nil {
		return generatedFixture{}, fmt.Errorf("generate XMLDSig choice root %s alternative %d: %w", root, alternative, err)
	}
	schemaPath := schemaPathForDocument(xsdDir, doc.File)
	if err := validateStructuralCandidate(content, schemaPath); err != nil {
		return generatedFixture{}, fmt.Errorf("validate XMLDSig choice root %s alternative %d: %w", root, alternative, err)
	}
	family := fmt.Sprintf("xmldsig-root-%s-choice-%03d", strings.ToLower(root), alternative)
	return generatedFixture{entry: fixtureManifestEntry{
		ID: "ubl21.signature." + family, DocumentType: root, Profile: "ubl21",
		File:   filepath.ToSlash(filepath.Join("SignatureSchemas", family+".xml")),
		SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(content))), Standards: fixtureStandards{UBL: "2.1"},
		Provenance:    fixtureProvenance{Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion},
		ExpectedFinal: "valid", ExpectedStages: []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "passed"}},
		Tags:              []string{"positive", "ubl21", "structural", "signature-schema", "xmldsig", "choice-alternative"},
		StructuralTargets: []string{"signature-root:" + root, "family:" + family},
	}, content: []byte(content), schemaPath: schemaPath}, nil
}

// namespaceFixtureKey produces a manifest-safe identifier component from a
// namespace URI. The manifest permits separators only between non-empty
// alphanumeric segments, so URI punctuation runs must collapse to one dash.
func namespaceFixtureKey(namespace string) string {
	var out strings.Builder
	separator := false
	for _, r := range strings.ToLower(namespace) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if separator && out.Len() > 0 {
				out.WriteByte('-')
			}
			out.WriteRune(r)
			separator = false
		} else {
			separator = true
		}
	}
	return out.String()
}

func parseStructuralResumeCursor(cursor string) (phase, root string, err error) {
	if cursor == "" {
		return "", "", nil
	}
	phase, root, ok := strings.Cut(cursor, ":")
	if !ok || root == "" || (phase != "main" && phase != "signature" && phase != "cardinality") {
		return "", "", fmt.Errorf("invalid --resume-from %q; use main:<root>, signature:<root>, or cardinality:<root>", cursor)
	}
	return phase, root, nil
}

func validateStructuralCandidate(content string, schemaPath string) error {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		return fmt.Errorf("xmllint is required for structural candidate validation: %w", err)
	}
	cmd := exec.Command(xmllint, "--nonet", "--noout", "--schema", schemaPath, "-")
	// strings.Reader presents the generated buffer directly to xmllint. The
	// previous []byte conversion duplicated every structural fixture solely for
	// validation (including the multi-megabyte collection witnesses).
	cmd.Stdin = strings.NewReader(content)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("XSD validation failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func maxChoiceAlternatives(index schemaIndex) int {
	max := 0
	var visitSequence func(*xsdSequence)
	var visitAll func(*xsdAll)
	var visitChoice func(*xsdChoice)
	visitSequence = func(sequence *xsdSequence) {
		if sequence == nil {
			return
		}
		for i := range sequence.Sequences {
			visitSequence(&sequence.Sequences[i])
		}
		for i := range sequence.Choices {
			visitChoice(&sequence.Choices[i])
		}
		for i := range sequence.Alls {
			visitAll(&sequence.Alls[i])
		}
	}
	visitAll = func(all *xsdAll) {
		if all == nil {
			return
		}
		for i := range all.Sequences {
			visitSequence(&all.Sequences[i])
		}
		for i := range all.Choices {
			visitChoice(&all.Choices[i])
		}
		for i := range all.Alls {
			visitAll(&all.Alls[i])
		}
	}
	visitChoice = func(choice *xsdChoice) {
		if choice == nil {
			return
		}
		alternatives := len(choice.Elements) + len(choice.Sequences) + len(choice.Choices) + len(choice.Alls)
		if alternatives > max {
			max = alternatives
		}
		for i := range choice.Sequences {
			visitSequence(&choice.Sequences[i])
		}
		for i := range choice.Choices {
			visitChoice(&choice.Choices[i])
		}
		for i := range choice.Alls {
			visitAll(&choice.Alls[i])
		}
	}
	for _, typ := range index.GlobalTypes {
		visitSequence(typ.Sequence)
		visitChoice(typ.Choice)
		visitAll(typ.All)
	}
	return max
}

func buildGeneratedCorpus(fixtures []generatedFixture) (generatedCorpus, error) {
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].entry.ID < fixtures[j].entry.ID })
	entries := make([]fixtureManifestEntry, len(fixtures))
	files := make(map[string]string, len(fixtures))
	for index := range fixtures {
		entries[index] = fixtures[index].entry
		if previousID, duplicate := files[fixtures[index].entry.File]; duplicate {
			return generatedCorpus{}, fmt.Errorf("generated fixtures %q and %q use the same file %q", previousID, fixtures[index].entry.ID, fixtures[index].entry.File)
		}
		files[fixtures[index].entry.File] = fixtures[index].entry.ID
	}
	manifestJSON, err := json.MarshalIndent(fixtureManifest{SchemaVersion: 1, Fixtures: entries}, "", "  ")
	if err != nil {
		return generatedCorpus{}, fmt.Errorf("marshal fixture manifest: %w", err)
	}
	manifestJSON = append(manifestJSON, '\n')
	if err := regression.ValidateFixtureManifest(manifestJSON); err != nil {
		return generatedCorpus{}, fmt.Errorf("validate generated fixture manifest: %w", err)
	}
	return generatedCorpus{fixtures: fixtures, manifestJSON: manifestJSON}, nil
}

func signatureCardinalityZeroFamily(planIndex int, targetID string) string {
	targetHash := sha256.Sum256([]byte(targetID))
	return fmt.Sprintf("signature-cardinality-plan-%05d-0-target-%x", planIndex, targetHash[:6])
}

func combineGeneratedCorpora(corpora ...generatedCorpus) (generatedCorpus, error) {
	var fixtures []generatedFixture
	for _, corpus := range corpora {
		fixtures = append(fixtures, corpus.fixtures...)
	}
	return buildGeneratedCorpus(fixtures)
}

func verifyCorpusDeterminism(first, second generatedCorpus) error {
	if !bytes.Equal(first.manifestJSON, second.manifestJSON) {
		return fmt.Errorf("generated manifest differs between consecutive clean generations")
	}
	if len(first.fixtures) != len(second.fixtures) {
		return fmt.Errorf("generated fixture count differs between consecutive clean generations: %d != %d", len(first.fixtures), len(second.fixtures))
	}
	for index := range first.fixtures {
		if first.fixtures[index].entry.File != second.fixtures[index].entry.File || !bytes.Equal(first.fixtures[index].content, second.fixtures[index].content) {
			return fmt.Errorf("generated fixture %q differs between consecutive clean generations", first.fixtures[index].entry.ID)
		}
	}
	return nil
}

func validateGeneratedCorpus(xmllintPath string, corpus generatedCorpus) error {
	for _, fixture := range corpus.fixtures {
		command := exec.Command(xmllintPath, "--nonet", "--noout", "--schema", fixture.schemaPath, "-")
		command.Stdin = bytes.NewReader(fixture.content)
		output, err := command.CombinedOutput()
		diagnostics := strings.TrimSpace(string(output))
		expectsXSDPass := fixture.entry.ExpectedFinal == "valid"
		if len(fixture.entry.ExpectedStages) > 0 {
			expectsXSDPass = fixture.entry.ExpectedStages[0].Outcome == "passed"
		}
		if expectsXSDPass {
			if err != nil {
				return fmt.Errorf("xmllint validate %s against %s: %w: %s", fixture.entry.ID, fixture.schemaPath, err, diagnostics)
			}
			continue
		}
		if err == nil {
			return fmt.Errorf("xmllint unexpectedly accepted negative fixture %s against %s", fixture.entry.ID, fixture.schemaPath)
		}
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			return fmt.Errorf("run xmllint for negative fixture %s: %w", fixture.entry.ID, err)
		}
		if fixture.expectedDiagnostic == "" || !strings.Contains(diagnostics, fixture.expectedDiagnostic) {
			return fmt.Errorf("negative fixture %s failed without expected diagnostic %q: %s", fixture.entry.ID, fixture.expectedDiagnostic, diagnostics)
		}
	}
	return nil
}

func writeGeneratedCorpus(outputDir string, corpus generatedCorpus) error {
	if err := validateCorpusOutputDirectory(outputDir); err != nil {
		return err
	}
	absolute, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolve generated corpus output: %w", err)
	}
	parent := filepath.Dir(absolute)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create generated corpus parent %s: %w", parent, err)
	}
	staging, err := os.MkdirTemp(parent, "."+filepath.Base(absolute)+".staging-*")
	if err != nil {
		return fmt.Errorf("create generated corpus staging directory: %w", err)
	}
	stagingExists := true
	defer func() {
		if stagingExists {
			_ = os.RemoveAll(staging)
		}
	}()
	if err := writeCorpusFiles(staging, corpus); err != nil {
		return err
	}
	if err := validateCorpusOutputDirectory(absolute); err != nil {
		return err
	}

	_, statErr := os.Lstat(absolute)
	if errors.Is(statErr, os.ErrNotExist) {
		if err := os.Rename(staging, absolute); err != nil {
			return fmt.Errorf("install generated corpus at %s: %w", absolute, err)
		}
		stagingExists = false
	} else if statErr != nil {
		return fmt.Errorf("inspect generated corpus output %s: %w", absolute, statErr)
	} else {
		backup, err := os.MkdirTemp(parent, "."+filepath.Base(absolute)+".backup-*")
		if err != nil {
			return fmt.Errorf("reserve generated corpus backup path: %w", err)
		}
		if err := os.Remove(backup); err != nil {
			return fmt.Errorf("prepare generated corpus backup path: %w", err)
		}
		if err := os.Rename(absolute, backup); err != nil {
			return fmt.Errorf("move existing generated corpus to backup: %w", err)
		}
		if err := os.Rename(staging, absolute); err != nil {
			if restoreErr := os.Rename(backup, absolute); restoreErr != nil {
				return fmt.Errorf("install generated corpus: %w (also failed to restore previous corpus: %v)", err, restoreErr)
			}
			return fmt.Errorf("install generated corpus: %w", err)
		}
		stagingExists = false
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove replaced generated corpus backup %s: %w", backup, err)
		}
	}
	return nil
}

func writeCorpusFiles(outputDir string, corpus generatedCorpus) error {
	if err := os.WriteFile(filepath.Join(outputDir, corpusOwnershipMarkerName), []byte(corpusOwnershipMarkerContent), 0o644); err != nil {
		return fmt.Errorf("write generated corpus ownership marker: %w", err)
	}
	for _, fixture := range corpus.fixtures {
		path := filepath.Join(outputDir, filepath.FromSlash(fixture.entry.File))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create sample dir %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, fixture.content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	manifestPath := filepath.Join(outputDir, "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}
	if err := os.WriteFile(manifestPath, corpus.manifestJSON, 0o644); err != nil {
		return fmt.Errorf("write fixture manifest: %w", err)
	}
	return nil
}

func validateCorpusOutputDirectory(outputDir string) error {
	if outputDir == "" || filepath.Clean(outputDir) == "." {
		return fmt.Errorf("generated corpus output must be a dedicated non-empty directory")
	}
	absolute, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolve generated corpus output: %w", err)
	}
	volumeRoot := filepath.VolumeName(absolute) + string(filepath.Separator)
	if absolute == volumeRoot {
		return fmt.Errorf("generated corpus output must not be a filesystem root")
	}
	info, err := os.Lstat(absolute)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect generated corpus output %s: %w", absolute, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("generated corpus output %s must be a directory and not a symlink", absolute)
	}
	marker, err := os.ReadFile(filepath.Join(absolute, corpusOwnershipMarkerName))
	if err != nil || string(marker) != corpusOwnershipMarkerContent {
		return fmt.Errorf("refuse to replace %s because it is not recognizably generator-owned (missing or invalid %s)", absolute, corpusOwnershipMarkerName)
	}
	return nil
}
