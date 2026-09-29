package regression

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultPollInterval   = 100 * time.Millisecond
	defaultFixtureTimeout = 60 * time.Second
)

type ExpectedStage struct {
	Stage   string   `json:"stage"`
	Outcome string   `json:"outcome"`
	RuleIDs []string `json:"ruleIds,omitempty"`
}

type Fixture struct {
	ID             string          `json:"id"`
	DocumentType   string          `json:"documentType"`
	Profile        string          `json:"profile"`
	File           string          `json:"file"`
	SHA256         string          `json:"sha256"`
	ExpectedFinal  string          `json:"expectedFinal"`
	ExpectedStages []ExpectedStage `json:"expectedStages"`
}

type Manifest struct {
	SchemaVersion int       `json:"schemaVersion"`
	Fixtures      []Fixture `json:"fixtures"`
}

type ActualStage struct {
	Stage   string   `json:"stage"`
	Outcome string   `json:"outcome"`
	RuleIDs []string `json:"ruleIds,omitempty"`
}

type NormalizedResult struct {
	SchemaVersion  int             `json:"schemaVersion"`
	FixtureID      string          `json:"fixtureId"`
	SHA256         string          `json:"sha256"`
	Profile        string          `json:"profile"`
	ExpectedFinal  string          `json:"expectedFinal"`
	ActualFinal    string          `json:"actualFinal"`
	ExpectedStages []ExpectedStage `json:"expectedStages"`
	ActualStages   []ActualStage   `json:"actualStages"`
	Result         string          `json:"result"`
}

// RunMapEntry correlates a fixture with its volatile durable-upload identity.
// It is diagnostic evidence and must not participate in semantic comparison.
type RunMapEntry struct {
	SchemaVersion  int    `json:"schemaVersion"`
	FixtureID      string `json:"fixtureId"`
	UploadID       string `json:"uploadId,omitempty"`
	DocumentType   string `json:"documentType"`
	Profile        string `json:"profile"`
	TerminalStatus string `json:"terminalStatus"`
}

// FixtureFailure preserves an operational error without including it in semantic equivalence.
type FixtureFailure struct {
	SchemaVersion int    `json:"schemaVersion"`
	FixtureID     string `json:"fixtureId"`
	Error         string `json:"error"`
}

type Counts struct {
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

type Summary struct {
	SchemaVersion int               `json:"schemaVersion"`
	Total         int               `json:"total"`
	Passed        int               `json:"passed"`
	Failed        int               `json:"failed"`
	ByProfile     map[string]Counts `json:"byProfile"`
}

type RunnerConfig struct {
	BaseURL        string
	Token          string
	ManifestPath   string
	OutputDir      string
	Profile        string
	Client         *http.Client
	PollInterval   time.Duration
	FixtureTimeout time.Duration
}

func RunCorpus(ctx context.Context, cfg RunnerConfig) (Summary, error) {
	if ctx == nil {
		return Summary{}, errors.New("regression context is nil")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.Token) == "" || cfg.ManifestPath == "" || cfg.OutputDir == "" {
		return Summary{}, errors.New("base URL, bearer token, manifest path, and output directory are required")
	}
	baseURL, err := url.Parse(strings.TrimRight(cfg.BaseURL, "/"))
	if err != nil || (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.Host == "" {
		return Summary{}, fmt.Errorf("invalid regression base URL %q", cfg.BaseURL)
	}
	manifestBytes, err := os.ReadFile(cfg.ManifestPath)
	if err != nil {
		return Summary{}, fmt.Errorf("read fixture manifest: %w", err)
	}
	if err := ValidateFixtureManifest(manifestBytes); err != nil {
		return Summary{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return Summary{}, fmt.Errorf("decode fixture manifest: %w", err)
	}
	fixtures := selectFixtures(manifest.Fixtures, cfg.Profile)
	if len(fixtures) == 0 {
		return Summary{}, fmt.Errorf("manifest contains no fixtures for profile %q", cfg.Profile)
	}
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].ID < fixtures[j].ID })
	client := cfg.Client
	if client == nil {
		client = http.DefaultClient
	}
	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	fixtureTimeout := cfg.FixtureTimeout
	if fixtureTimeout <= 0 {
		fixtureTimeout = defaultFixtureTimeout
	}
	corpusRoot := filepath.Dir(cfg.ManifestPath)
	results := make([]NormalizedResult, 0, len(fixtures))
	runMap := make([]RunMapEntry, 0, len(fixtures))
	failures := make([]FixtureFailure, 0)
	for _, fixture := range fixtures {
		result, correlation, failure := runFixture(ctx, client, baseURL, cfg.Token, corpusRoot, fixture, pollInterval, fixtureTimeout)
		results = append(results, result)
		runMap = append(runMap, correlation)
		if failure != nil {
			failures = append(failures, *failure)
		}
	}
	summary := summarize(results)
	if err := writeArtifacts(cfg.OutputDir, results, runMap, failures, summary); err != nil {
		return Summary{}, err
	}
	if summary.Failed > 0 {
		failed := make([]string, 0, summary.Failed)
		for _, result := range results {
			if result.Result == "fail" {
				failed = append(failed, result.FixtureID)
			}
		}
		return summary, fmt.Errorf("regression mismatches: %d of %d fixtures failed: %s", summary.Failed, summary.Total, strings.Join(failed, ", "))
	}
	return summary, nil
}

func selectFixtures(fixtures []Fixture, profile string) []Fixture {
	profile = strings.TrimSpace(profile)
	selected := make([]Fixture, 0, len(fixtures))
	for _, fixture := range fixtures {
		if profile == "" || profile == "all" || fixture.Profile == profile {
			selected = append(selected, fixture)
		}
	}
	return selected
}

func runFixture(ctx context.Context, client *http.Client, baseURL *url.URL, token, corpusRoot string, fixture Fixture, pollInterval, timeout time.Duration) (NormalizedResult, RunMapEntry, *FixtureFailure) {
	result := NormalizedResult{SchemaVersion: 1, FixtureID: fixture.ID, SHA256: fixture.SHA256, Profile: fixture.Profile, ExpectedFinal: fixture.ExpectedFinal, ExpectedStages: fixture.ExpectedStages}
	correlation := RunMapEntry{SchemaVersion: 1, FixtureID: fixture.ID, DocumentType: fixture.DocumentType, Profile: fixture.Profile, TerminalStatus: "operational-failure"}
	content, err := os.ReadFile(filepath.Join(corpusRoot, filepath.FromSlash(fixture.File)))
	if err == nil {
		actualHash := fmt.Sprintf("%x", sha256.Sum256(content))
		if actualHash != fixture.SHA256 {
			err = fmt.Errorf("fixture SHA-256 is %s, manifest has %s", actualHash, fixture.SHA256)
		}
	}
	if err != nil {
		return operationalResult(result, err), correlation, fixtureFailure(fixture.ID, err)
	}
	fixtureCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	id, err := uploadFixture(fixtureCtx, client, baseURL, token, fixture, content)
	if err != nil {
		return operationalResult(result, err), correlation, fixtureFailure(fixture.ID, err)
	}
	correlation.UploadID = id
	response, err := pollFixture(fixtureCtx, client, baseURL, token, id, pollInterval)
	if err != nil {
		return operationalResult(result, err), correlation, fixtureFailure(fixture.ID, err)
	}
	correlation.TerminalStatus = response.Status
	if response.Profile != fixture.Profile {
		err := fmt.Errorf("status profile %q does not match requested %q", response.Profile, fixture.Profile)
		return operationalResult(result, err), correlation, fixtureFailure(fixture.ID, err)
	}
	result.ActualFinal, result.ActualStages = normalizeAPIResult(response, fixture.ExpectedStages)
	result.Result = compareResult(result)
	if encoded, err := json.Marshal(result); err != nil || ValidateNormalizedResult(encoded) != nil {
		err := fmt.Errorf("normalized result violates contract")
		return operationalResult(result, err), correlation, fixtureFailure(fixture.ID, err)
	}
	return result, correlation, nil
}

func fixtureFailure(fixtureID string, err error) *FixtureFailure {
	return &FixtureFailure{SchemaVersion: 1, FixtureID: fixtureID, Error: err.Error()}
}

func uploadFixture(ctx context.Context, client *http.Client, baseURL *url.URL, token string, fixture Fixture, content []byte) (string, error) {
	endpoint := *baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/api/v1/ubl/upload"
	query := endpoint.Query()
	query.Set("profile", fixture.Profile)
	query.Set("documentType", fixture.DocumentType)
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(content))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/xml")
	var accepted struct {
		ID      string `json:"id"`
		Profile string `json:"profile"`
	}
	if err := doJSON(client, req, http.StatusAccepted, &accepted); err != nil {
		return "", fmt.Errorf("upload fixture %s: %w", fixture.ID, err)
	}
	if accepted.ID == "" || accepted.Profile != fixture.Profile {
		return "", fmt.Errorf("upload fixture %s returned inconsistent id/profile", fixture.ID)
	}
	return accepted.ID, nil
}

type apiResponse struct {
	Status  string     `json:"status"`
	Profile string     `json:"profile"`
	Stages  []apiStage `json:"validationStages"`
}

type apiStage struct {
	Layer  string     `json:"layer"`
	Valid  bool       `json:"valid"`
	Issues []apiIssue `json:"issues"`
}

type apiIssue struct {
	Kind     string `json:"kind"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
}

func pollFixture(ctx context.Context, client *http.Client, baseURL *url.URL, token, id string, interval time.Duration) (apiResponse, error) {
	endpoint := *baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/api/v1/ubl/" + url.PathEscape(id)
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return apiResponse{}, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		var response apiResponse
		if err := doJSON(client, req, http.StatusOK, &response); err != nil {
			return apiResponse{}, fmt.Errorf("poll upload %s: %w", id, err)
		}
		switch response.Status {
		case "valid", "invalid", "processing_failed", "acceptance_failed":
			return response, nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return apiResponse{}, fmt.Errorf("wait for upload %s: %w", id, ctx.Err())
		case <-timer.C:
		}
	}
}

func doJSON(client *http.Client, req *http.Request, wantStatus int, target any) error {
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != wantStatus {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(target); err != nil {
		return fmt.Errorf("decode HTTP response: %w", err)
	}
	return nil
}

func normalizeAPIResult(response apiResponse, expected []ExpectedStage) (string, []ActualStage) {
	actual := make([]ActualStage, 0, len(response.Stages)+1)
	for _, stage := range response.Stages {
		normalized := ActualStage{Stage: stage.Layer, Outcome: "passed"}
		if !stage.Valid {
			normalized.Outcome = "standards-invalid"
			for _, issue := range stage.Issues {
				severity := strings.ToLower(strings.TrimSpace(issue.Severity))
				if issue.Kind == "failed-assert" && issue.Rule != "" && severity != "warning" && severity != "warn" {
					normalized.RuleIDs = append(normalized.RuleIDs, issue.Rule)
				}
			}
			sort.Strings(normalized.RuleIDs)
			normalized.RuleIDs = uniqueStrings(normalized.RuleIDs)
		}
		actual = append(actual, normalized)
	}
	switch response.Status {
	case "valid", "invalid":
		return response.Status, actual
	default:
		stage := "ubl21-xsd"
		if len(actual) < len(expected) {
			stage = expected[len(actual)].Stage
		}
		actual = append(actual, ActualStage{Stage: stage, Outcome: "operational-failure"})
		return "operational-failure", actual
	}
}

func operationalResult(result NormalizedResult, _ error) NormalizedResult {
	stage := "ubl21-xsd"
	if len(result.ExpectedStages) > 0 {
		stage = result.ExpectedStages[0].Stage
	}
	result.ActualFinal = "operational-failure"
	result.ActualStages = []ActualStage{{Stage: stage, Outcome: "operational-failure"}}
	result.Result = "fail"
	return result
}

func compareResult(result NormalizedResult) string {
	if result.ExpectedFinal != result.ActualFinal || len(result.ExpectedStages) != len(result.ActualStages) {
		return "fail"
	}
	for index := range result.ExpectedStages {
		expected := result.ExpectedStages[index]
		actual := result.ActualStages[index]
		if expected.Stage != actual.Stage || expected.Outcome != actual.Outcome || !containsAll(actual.RuleIDs, expected.RuleIDs) {
			return "fail"
		}
	}
	return "pass"
}

func containsAll(actual, expected []string) bool {
	actualSet := make(map[string]struct{}, len(actual))
	for _, value := range actual {
		actualSet[value] = struct{}{}
	}
	for _, value := range expected {
		if _, ok := actualSet[value]; !ok {
			return false
		}
	}
	return true
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func summarize(results []NormalizedResult) Summary {
	summary := Summary{SchemaVersion: 1, Total: len(results), ByProfile: map[string]Counts{}}
	for _, result := range results {
		counts := summary.ByProfile[result.Profile]
		if result.Result == "pass" {
			summary.Passed++
			counts.Passed++
		} else {
			summary.Failed++
			counts.Failed++
		}
		summary.ByProfile[result.Profile] = counts
	}
	return summary
}

func writeArtifacts(outputDir string, results []NormalizedResult, runMap []RunMapEntry, failures []FixtureFailure, summary Summary) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create regression output: %w", err)
	}
	var lines bytes.Buffer
	writer := bufio.NewWriter(&lines)
	for _, result := range results {
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if err := ValidateNormalizedResult(encoded); err != nil {
			return fmt.Errorf("validate normalized result %s: %w", result.FixtureID, err)
		}
		_, _ = writer.Write(encoded)
		_ = writer.WriteByte('\n')
	}
	_ = writer.Flush()
	var runMapLines bytes.Buffer
	runMapWriter := bufio.NewWriter(&runMapLines)
	for _, entry := range runMap {
		encoded, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		_, _ = runMapWriter.Write(encoded)
		_ = runMapWriter.WriteByte('\n')
	}
	_ = runMapWriter.Flush()
	var failureLines bytes.Buffer
	failureWriter := bufio.NewWriter(&failureLines)
	for _, failure := range failures {
		encoded, err := json.Marshal(failure)
		if err != nil {
			return err
		}
		_, _ = failureWriter.Write(encoded)
		_ = failureWriter.WriteByte('\n')
	}
	_ = failureWriter.Flush()
	summaryJSON, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	summaryJSON = append(summaryJSON, '\n')
	junitXML, err := marshalJUnit(results, summary)
	if err != nil {
		return err
	}
	for name, content := range map[string][]byte{"results.jsonl": lines.Bytes(), "run-map.jsonl": runMapLines.Bytes(), "failures.jsonl": failureLines.Bytes(), "summary.json": summaryJSON, "junit.xml": junitXML} {
		if err := writeFileAtomically(filepath.Join(outputDir, name), content); err != nil {
			return err
		}
	}
	return nil
}

type junitSuite struct {
	XMLName  xml.Name    `xml:"testsuite"`
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

func marshalJUnit(results []NormalizedResult, summary Summary) ([]byte, error) {
	suite := junitSuite{Name: "e-ubl21 regression", Tests: summary.Total, Failures: summary.Failed}
	for _, result := range results {
		item := junitCase{Name: result.FixtureID, Classname: result.Profile}
		if result.Result == "fail" {
			item.Failure = &junitFailure{Message: "normalized result mismatch", Body: fmt.Sprintf("expected %s %v; actual %s %v", result.ExpectedFinal, result.ExpectedStages, result.ActualFinal, result.ActualStages)}
		}
		suite.Cases = append(suite.Cases, item)
	}
	encoded, err := xml.MarshalIndent(suite, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), append(encoded, '\n')...), nil
}

func writeFileAtomically(path string, content []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".regression-artifact-*")
	if err != nil {
		return fmt.Errorf("create temporary artifact: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o644); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("install regression artifact %s: %w", path, err)
	}
	return nil
}
