package regression

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
)

const maxNormalizedResultBytes = 2 << 20

// EquivalenceSummary records the deterministic semantic comparison between a
// frozen monolithic result set and a candidate validation topology.
type EquivalenceSummary struct {
	SchemaVersion        int      `json:"schemaVersion"`
	Total                int      `json:"total"`
	Equivalent           int      `json:"equivalent"`
	Changed              int      `json:"changed"`
	Missing              int      `json:"missing"`
	Unexpected           int      `json:"unexpected"`
	ChangedFixtureIDs    []string `json:"changedFixtureIds"`
	MissingFixtureIDs    []string `json:"missingFixtureIds"`
	UnexpectedFixtureIDs []string `json:"unexpectedFixtureIds"`
}

// CompareResultSets requires the candidate to contain the same normalized
// fixture identities, expectations, outcomes, ordered stages, and exact rule
// IDs as the frozen baseline. Line order is deliberately ignored.
func CompareResultSets(baselinePath, candidatePath string) (EquivalenceSummary, error) {
	summary := EquivalenceSummary{
		SchemaVersion:        1,
		ChangedFixtureIDs:    []string{},
		MissingFixtureIDs:    []string{},
		UnexpectedFixtureIDs: []string{},
	}
	baseline, err := readNormalizedResultSet(baselinePath)
	if err != nil {
		return summary, fmt.Errorf("read baseline result set: %w", err)
	}
	summary.Total = len(baseline)
	candidate, err := readNormalizedResultSet(candidatePath)
	if err != nil {
		return summary, fmt.Errorf("read candidate result set: %w", err)
	}

	for fixtureID, expected := range baseline {
		actual, exists := candidate[fixtureID]
		if !exists {
			summary.MissingFixtureIDs = append(summary.MissingFixtureIDs, fixtureID)
			continue
		}
		if !reflect.DeepEqual(expected, actual) {
			summary.ChangedFixtureIDs = append(summary.ChangedFixtureIDs, fixtureID)
			continue
		}
		summary.Equivalent++
	}
	for fixtureID := range candidate {
		if _, exists := baseline[fixtureID]; !exists {
			summary.UnexpectedFixtureIDs = append(summary.UnexpectedFixtureIDs, fixtureID)
		}
	}
	sort.Strings(summary.ChangedFixtureIDs)
	sort.Strings(summary.MissingFixtureIDs)
	sort.Strings(summary.UnexpectedFixtureIDs)
	summary.Changed = len(summary.ChangedFixtureIDs)
	summary.Missing = len(summary.MissingFixtureIDs)
	summary.Unexpected = len(summary.UnexpectedFixtureIDs)
	if summary.Changed+summary.Missing+summary.Unexpected > 0 {
		return summary, fmt.Errorf(
			"validation semantics differ from monolithic baseline: changed=%d missing=%d unexpected=%d fixtures=%s",
			summary.Changed, summary.Missing, summary.Unexpected, joinEquivalenceFailures(summary),
		)
	}
	return summary, nil
}

// WriteEquivalenceSummary writes a stable JSON artifact for CI diagnostics.
func WriteEquivalenceSummary(path string, summary EquivalenceSummary) error {
	if path == "" {
		return errors.New("equivalence summary path is required")
	}
	content, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("encode equivalence summary: %w", err)
	}
	content = append(content, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create equivalence summary directory: %w", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write equivalence summary: %w", err)
	}
	return nil
}

func readNormalizedResultSet(path string) (map[string]NormalizedResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	results := map[string]NormalizedResult{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), maxNormalizedResultBytes)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			return nil, fmt.Errorf("line %d is empty", lineNumber)
		}
		if err := ValidateNormalizedResult(line); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		var result NormalizedResult
		if err := json.Unmarshal(line, &result); err != nil {
			return nil, fmt.Errorf("decode line %d: %w", lineNumber, err)
		}
		if _, duplicate := results[result.FixtureID]; duplicate {
			return nil, fmt.Errorf("duplicate fixture id %q", result.FixtureID)
		}
		results[result.FixtureID] = result
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan normalized results: %w", err)
	}
	if len(results) == 0 {
		return nil, errors.New("normalized result set is empty")
	}
	return results, nil
}

func joinEquivalenceFailures(summary EquivalenceSummary) string {
	values := make([]string, 0, summary.Changed+summary.Missing+summary.Unexpected)
	for _, fixtureID := range summary.ChangedFixtureIDs {
		values = append(values, "changed:"+fixtureID)
	}
	for _, fixtureID := range summary.MissingFixtureIDs {
		values = append(values, "missing:"+fixtureID)
	}
	for _, fixtureID := range summary.UnexpectedFixtureIDs {
		values = append(values, "unexpected:"+fixtureID)
	}
	return fmt.Sprintf("%q", values)
}
