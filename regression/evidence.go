package regression

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	xsdschemas "github.com/PaddyFrenchman/e-ubl21/XSD"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
	"github.com/nats-io/nats.go"
)

const (
	maxLogBytes     = 4 << 20
	maxMetricsBytes = 2 << 20
	maxCommandBytes = 4 << 20
)

var (
	evidenceNamePattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	credentialURLPattern = regexp.MustCompile(`://[^/@\s]+@`)
	bearerTokenPattern   = regexp.MustCompile(`(?i)\bbearer[ \t]+[a-z0-9._~+/=-]+`)
	ublBillingStart      = regexp.MustCompile(`(?i)(?:<|\\u003c)(?:[a-z_][a-z0-9._-]*:)?(?:invoice|creditnote)\b`)
	ublBillingEnd        = regexp.MustCompile(`(?i)(?:</|\\u003c/)(?:[a-z_][a-z0-9._-]*:)?(?:invoice|creditnote)(?:>|\\u003e)`)
)

// EvidenceConfig describes diagnostic inputs collected beside normalized
// correctness artifacts. Diagnostic collection never reads fixture XML.
type EvidenceConfig struct {
	OutputDir             string
	ManifestPath          string
	Token                 string
	LogFiles              map[string]string
	MetricsEndpoints      map[string]string
	NATSURL               string
	NATSOptions           []nats.Option
	QueueNamespace        queue.Namespace
	CollectNATS           bool
	CollectKubernetes     bool
	KubernetesNamespace   string
	KubernetesSelector    string
	KubectlExecutable     string
	MinikubeExecutable    string
	JavaExecutable        string
	XMLLintExecutable     string
	DockerImageDigest     string
	StartedAt             time.Time
	EndedAt               time.Time
	SourceTimeout         time.Duration
	Client                *http.Client
	commandRunner         evidenceCommandRunner
	natsSnapshotCollector natsSnapshotCollector
}

type EvidenceReport struct {
	SchemaVersion int             `json:"schemaVersion"`
	Captures      []EvidenceEntry `json:"captures"`
}

type EvidenceEntry struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Bytes     int    `json:"bytes,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
	Error     string `json:"error,omitempty"`
}

type ToolVersion struct {
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

type AssetVersion struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}

type Environment struct {
	SchemaVersion     int          `json:"schemaVersion"`
	GitCommitSHA      string       `json:"gitCommitSha"`
	GitDirty          bool         `json:"gitDirty"`
	GeneratorVersions []string     `json:"generatorVersions"`
	UBL               AssetVersion `json:"ubl"`
	EN16931           AssetVersion `json:"en16931"`
	Saxon             AssetVersion `json:"saxon"`
	XMLResolver       AssetVersion `json:"xmlResolver"`
	Go                ToolVersion  `json:"go"`
	OperatingSystem   string       `json:"operatingSystem"`
	Architecture      string       `json:"architecture"`
	Java              ToolVersion  `json:"java"`
	XMLLint           ToolVersion  `json:"xmllint"`
	DockerImageDigest string       `json:"dockerImageDigest,omitempty"`
	Kubernetes        *ToolVersion `json:"kubernetes,omitempty"`
	Minikube          *ToolVersion `json:"minikube,omitempty"`
	NATSServer        *ToolVersion `json:"natsServer,omitempty"`
	StartedAt         string       `json:"startedAt"`
	EndedAt           string       `json:"endedAt"`
}

type evidenceCommandRunner func(context.Context, int, string, ...string) ([]byte, bool, error)
type natsSnapshotCollector func(context.Context, EvidenceConfig) ([]byte, string, error)

// CollectEvidence writes bounded diagnostic evidence alongside, but never
// into, normalized correctness results. Requested-source failures are recorded
// in collection.json and returned only after every other source is attempted.
func CollectEvidence(ctx context.Context, cfg EvidenceConfig) (EvidenceReport, error) {
	if ctx == nil {
		return EvidenceReport{}, errors.New("evidence context is nil")
	}
	if cfg.OutputDir == "" || cfg.ManifestPath == "" {
		return EvidenceReport{}, errors.New("evidence output directory and manifest path are required")
	}
	if cfg.StartedAt.IsZero() && cfg.EndedAt.IsZero() {
		now := time.Now().UTC()
		cfg.StartedAt, cfg.EndedAt = now, now
	} else if cfg.StartedAt.IsZero() {
		cfg.StartedAt = cfg.EndedAt
	} else if cfg.EndedAt.IsZero() {
		cfg.EndedAt = time.Now().UTC()
	}
	if cfg.EndedAt.Before(cfg.StartedAt) {
		return EvidenceReport{}, errors.New("evidence end time precedes start time")
	}
	manifest, err := os.ReadFile(cfg.ManifestPath)
	if err != nil {
		return EvidenceReport{}, fmt.Errorf("read evidence manifest: %w", err)
	}
	if err := ValidateFixtureManifest(manifest); err != nil {
		return EvidenceReport{}, err
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return EvidenceReport{}, fmt.Errorf("create evidence output: %w", err)
	}

	report := EvidenceReport{SchemaVersion: 1}
	var collectionErrors []error
	record := func(entry EvidenceEntry, err error) {
		report.Captures = append(report.Captures, entry)
		if err != nil {
			collectionErrors = append(collectionErrors, err)
		}
	}
	if err := writeFileAtomically(filepath.Join(cfg.OutputDir, "manifest-used.json"), manifest); err != nil {
		record(failedEvidence("manifest-used", err), err)
	} else {
		record(EvidenceEntry{Name: "manifest-used", Status: "captured", Bytes: len(manifest)}, nil)
	}

	baseRunner := cfg.commandRunner
	if baseRunner == nil {
		baseRunner = runEvidenceCommand
	}
	sourceTimeout := cfg.SourceTimeout
	if sourceTimeout <= 0 {
		sourceTimeout = 15 * time.Second
	}
	runner := func(parent context.Context, limit int, executable string, args ...string) ([]byte, bool, error) {
		commandCtx, cancel := context.WithTimeout(parent, sourceTimeout)
		defer cancel()
		return baseRunner(commandCtx, limit, executable, args...)
	}
	environment, environmentEntries := collectEnvironment(ctx, cfg, manifest, runner)
	for _, entry := range environmentEntries {
		report.Captures = append(report.Captures, entry)
		if entry.Status == "failed" {
			collectionErrors = append(collectionErrors, fmt.Errorf("%s: %s", entry.Name, entry.Error))
		}
	}
	if cfg.CollectNATS {
		collector := cfg.natsSnapshotCollector
		if collector == nil {
			collector = collectNATSSnapshot
		}
		sourceCtx, cancel := context.WithTimeout(ctx, sourceTimeout)
		content, version, snapshotErr := collector(sourceCtx, cfg)
		cancel()
		natsVersion := toolVersion(version, snapshotErr)
		environment.NATSServer = &natsVersion
		if snapshotErr != nil {
			record(failedEvidence("nats/state", snapshotErr), snapshotErr)
		} else {
			path := filepath.Join(cfg.OutputDir, "nats", "state.json")
			err = os.MkdirAll(filepath.Dir(path), 0o755)
			if err == nil {
				err = writeFileAtomically(path, content)
			}
			if err != nil {
				record(failedEvidence("nats/state", err), err)
			} else {
				record(EvidenceEntry{Name: "nats/state", Status: "captured", Bytes: len(content)}, nil)
			}
		}
	}
	environmentJSON, err := marshalEvidenceJSON(environment)
	if err != nil {
		record(failedEvidence("environment", err), err)
	} else if err := writeFileAtomically(filepath.Join(cfg.OutputDir, "environment.json"), environmentJSON); err != nil {
		record(failedEvidence("environment", err), err)
	} else {
		record(EvidenceEntry{Name: "environment", Status: "captured", Bytes: len(environmentJSON)}, nil)
	}

	for _, name := range sortedMapKeys(cfg.LogFiles) {
		entry, err := collectLogFile(cfg.OutputDir, name, cfg.LogFiles[name])
		record(entry, err)
	}
	client := cfg.Client
	if client == nil {
		client = http.DefaultClient
	}
	for _, name := range sortedMapKeys(cfg.MetricsEndpoints) {
		sourceCtx, cancel := context.WithTimeout(ctx, sourceTimeout)
		entry, err := collectMetrics(sourceCtx, client, cfg.OutputDir, cfg.Token, name, cfg.MetricsEndpoints[name])
		cancel()
		record(entry, err)
	}

	if cfg.CollectKubernetes {
		entries, errs := collectKubernetesEvidence(ctx, cfg, runner)
		report.Captures = append(report.Captures, entries...)
		collectionErrors = append(collectionErrors, errs...)
	}

	sort.Slice(report.Captures, func(i, j int) bool { return report.Captures[i].Name < report.Captures[j].Name })
	reportJSON, err := marshalEvidenceJSON(report)
	if err != nil {
		return report, errors.Join(append(collectionErrors, err)...)
	}
	if err := writeFileAtomically(filepath.Join(cfg.OutputDir, "collection.json"), reportJSON); err != nil {
		collectionErrors = append(collectionErrors, err)
	}
	return report, errors.Join(collectionErrors...)
}

func collectEnvironment(ctx context.Context, cfg EvidenceConfig, manifest []byte, runner evidenceCommandRunner) (Environment, []EvidenceEntry) {
	saxon := validationassets.Saxon()
	en := validationassets.EN16931()
	environment := Environment{
		SchemaVersion:     1,
		GeneratorVersions: manifestGeneratorVersions(manifest),
		UBL:               AssetVersion{Version: "2.1", SHA256: embeddedUBLChecksum()},
		EN16931:           AssetVersion{Version: en.Version, SHA256: en.XSLTSHA256},
		Saxon:             AssetVersion{Version: saxon.SaxonVersion, SHA256: saxon.SaxonJARSHA256},
		XMLResolver:       AssetVersion{Version: saxon.XMLResolverVersion, SHA256: saxon.XMLResolverSHA256},
		Go:                ToolVersion{Version: runtime.Version()},
		OperatingSystem:   runtime.GOOS,
		Architecture:      runtime.GOARCH,
		DockerImageDigest: strings.TrimSpace(cfg.DockerImageDigest),
		StartedAt:         cfg.StartedAt.UTC().Format(time.RFC3339Nano),
		EndedAt:           cfg.EndedAt.UTC().Format(time.RFC3339Nano),
	}
	var entries []EvidenceEntry
	gitSHA, _, gitErr := runner(ctx, 4096, "git", "rev-parse", "HEAD")
	if gitErr != nil {
		entries = append(entries, failedEvidence("environment/git-commit", gitErr))
	} else {
		environment.GitCommitSHA = strings.TrimSpace(string(gitSHA))
	}
	status, truncated, statusErr := runner(ctx, 1<<20, "git", "status", "--porcelain=v1")
	if statusErr != nil {
		entries = append(entries, failedEvidence("environment/git-status", statusErr))
	} else {
		environment.GitDirty = len(bytes.TrimSpace(status)) > 0
		entries = append(entries, EvidenceEntry{Name: "environment/git-status", Status: "captured", Bytes: len(status), Truncated: truncated})
	}
	java := cfg.JavaExecutable
	if java == "" {
		java = "java"
	}
	environment.Java = commandToolVersion(ctx, runner, java, "-version")
	if environment.Java.Error != "" {
		entries = append(entries, failedEvidence("environment/java", errors.New(environment.Java.Error)))
	}
	xmllint := cfg.XMLLintExecutable
	if xmllint == "" {
		xmllint = "xmllint"
	}
	environment.XMLLint = commandToolVersion(ctx, runner, xmllint, "--version")
	if environment.XMLLint.Error != "" {
		entries = append(entries, failedEvidence("environment/xmllint", errors.New(environment.XMLLint.Error)))
	}
	if cfg.CollectKubernetes {
		kubectl := cfg.KubectlExecutable
		if kubectl == "" {
			kubectl = "kubectl"
		}
		kubernetesVersion := commandToolVersion(ctx, runner, kubectl, "version", "-o", "json")
		environment.Kubernetes = &kubernetesVersion
		if kubernetesVersion.Error != "" {
			entries = append(entries, failedEvidence("environment/kubernetes", errors.New(kubernetesVersion.Error)))
		}
		minikube := cfg.MinikubeExecutable
		if minikube == "" {
			minikube = "minikube"
		}
		minikubeVersion := commandToolVersion(ctx, runner, minikube, "version", "--output=json")
		environment.Minikube = &minikubeVersion
		if minikubeVersion.Error != "" {
			entries = append(entries, failedEvidence("environment/minikube", errors.New(minikubeVersion.Error)))
		}
	}
	return environment, entries
}

func embeddedUBLChecksum() string {
	var paths []string
	_ = fs.WalkDir(xsdschemas.FS, ".", func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".xsd") {
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		content, err := xsdschemas.FS.ReadFile(path)
		if err != nil {
			continue
		}
		_, _ = io.WriteString(hash, path)
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(content)
		_, _ = hash.Write([]byte{0})
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func manifestGeneratorVersions(content []byte) []string {
	var document struct {
		Fixtures []struct {
			Provenance struct {
				GeneratorVersion string `json:"generatorVersion"`
			} `json:"provenance"`
		} `json:"fixtures"`
	}
	_ = json.Unmarshal(content, &document)
	set := map[string]struct{}{}
	for _, fixture := range document.Fixtures {
		if fixture.Provenance.GeneratorVersion != "" {
			set[fixture.Provenance.GeneratorVersion] = struct{}{}
		}
	}
	versions := make([]string, 0, len(set))
	for version := range set {
		versions = append(versions, version)
	}
	sort.Strings(versions)
	return versions
}

func collectLogFile(outputDir, name, source string) (EvidenceEntry, error) {
	if err := validEvidenceName(name); err != nil {
		return failedEvidence("logs/"+name, err), err
	}
	file, err := os.Open(source)
	if err != nil {
		return failedEvidence("logs/"+name, err), err
	}
	defer file.Close()
	content, truncated, err := readBounded(file, maxLogBytes)
	if err != nil {
		return failedEvidence("logs/"+name, err), err
	}
	content, redactionTruncated := redactServiceLog(content)
	truncated = truncated || redactionTruncated
	path := filepath.Join(outputDir, "logs", name+".log")
	err = os.MkdirAll(filepath.Dir(path), 0o755)
	if err == nil {
		err = writeFileAtomically(path, content)
	}
	entry := EvidenceEntry{Name: "logs/" + name, Status: "captured", Bytes: len(content), Truncated: truncated}
	if err != nil {
		return failedEvidence(entry.Name, err), err
	}
	return entry, nil
}

func collectMetrics(ctx context.Context, client *http.Client, outputDir, token, name, endpoint string) (EvidenceEntry, error) {
	if err := validEvidenceName(name); err != nil {
		return failedEvidence("metrics/"+name, err), err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return failedEvidence("metrics/"+name, err), err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(req)
	if err != nil {
		return failedEvidence("metrics/"+name, err), err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("metrics endpoint returned HTTP %d", response.StatusCode)
		return failedEvidence("metrics/"+name, err), err
	}
	content, truncated, err := readBounded(response.Body, maxMetricsBytes)
	if err != nil {
		return failedEvidence("metrics/"+name, err), err
	}
	path := filepath.Join(outputDir, "metrics", name+".prom")
	err = os.MkdirAll(filepath.Dir(path), 0o755)
	if err == nil {
		err = writeFileAtomically(path, content)
	}
	entry := EvidenceEntry{Name: "metrics/" + name, Status: "captured", Bytes: len(content), Truncated: truncated}
	if err != nil {
		return failedEvidence(entry.Name, err), err
	}
	return entry, nil
}

func redactServiceLog(content []byte) ([]byte, bool) {
	var output bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 4096), 256<<10)
	inUBLDocument := false
	for scanner.Scan() {
		line := scanner.Text()
		lower := strings.ToLower(line)
		startsDocument := ublBillingStart.MatchString(line)
		endsDocument := ublBillingEnd.MatchString(line) || (startsDocument && (strings.Contains(line, "/>") || strings.Contains(lower, `/\u003e`)))
		if inUBLDocument || startsDocument || strings.Contains(lower, "urn:oasis:names:specification:ubl:schema:xsd:") {
			if !inUBLDocument || startsDocument {
				output.WriteString("[REDACTED UBL XML]\n")
			}
			inUBLDocument = !endsDocument
			continue
		}
		output.WriteString(redactBearer(line))
		output.WriteByte('\n')
	}
	truncated := scanner.Err() != nil
	if truncated {
		output.WriteString("[REDACTED OVERSIZED LOG LINE]\n")
	}
	if output.Len() > maxLogBytes {
		return output.Bytes()[:maxLogBytes], true
	}
	return output.Bytes(), truncated
}

func redactBearer(line string) string {
	return bearerTokenPattern.ReplaceAllString(line, "Bearer [REDACTED]")
}

func readBounded(reader io.Reader, limit int64) ([]byte, bool, error) {
	content, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(content)) > limit {
		return content[:limit], true, nil
	}
	return content, false, nil
}

func runEvidenceCommand(ctx context.Context, limit int, executable string, args ...string) ([]byte, bool, error) {
	var output boundedCommandBuffer
	output.limit = limit
	command := exec.CommandContext(ctx, executable, args...)
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	return output.Bytes(), output.truncated, err
}

type boundedCommandBuffer struct {
	bytes.Buffer
	limit     int
	truncated bool
}

func (buffer *boundedCommandBuffer) Write(content []byte) (int, error) {
	original := len(content)
	remaining := buffer.limit - buffer.Len()
	if remaining <= 0 {
		buffer.truncated = true
		return original, nil
	}
	if len(content) > remaining {
		_, _ = buffer.Buffer.Write(content[:remaining])
		buffer.truncated = true
		return original, nil
	}
	_, _ = buffer.Buffer.Write(content)
	return original, nil
}

func commandToolVersion(ctx context.Context, runner evidenceCommandRunner, executable string, args ...string) ToolVersion {
	content, _, err := runner(ctx, 32<<10, executable, args...)
	if err != nil {
		return ToolVersion{Error: boundedError(err)}
	}
	return ToolVersion{Version: strings.TrimSpace(string(content))}
}

func toolVersion(version string, err error) ToolVersion {
	if err != nil {
		return ToolVersion{Error: boundedError(err)}
	}
	return ToolVersion{Version: strings.TrimSpace(version)}
}

func boundedError(err error) string {
	if err == nil {
		return ""
	}
	message := credentialURLPattern.ReplaceAllString(err.Error(), "://[REDACTED]@")
	message = redactBearer(message)
	if len(message) > 1024 {
		message = message[:1024]
	}
	return message
}

func marshalEvidenceJSON(value any) ([]byte, error) {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func failedEvidence(name string, err error) EvidenceEntry {
	return EvidenceEntry{Name: name, Status: "failed", Error: boundedError(err)}
}

func validEvidenceName(name string) error {
	if !evidenceNamePattern.MatchString(name) {
		return fmt.Errorf("invalid evidence source name %q", name)
	}
	return nil
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
