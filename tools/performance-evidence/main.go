package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const schemaVersion = 1

type inputFlags []string

func (values *inputFlags) String() string { return strings.Join(*values, ",") }

func (values *inputFlags) Set(value string) error {
	if !strings.Contains(value, "=") {
		return fmt.Errorf("input must be NAME=PATH, got %q", value)
	}
	*values = append(*values, value)
	return nil
}

type evidence struct {
	SchemaVersion int             `json:"schemaVersion"`
	GeneratedAt   string          `json:"generatedAt"`
	Revision      string          `json:"revision,omitempty"`
	RunID         string          `json:"runId,omitempty"`
	Trigger       string          `json:"trigger,omitempty"`
	Sources       []source        `json:"sources"`
	Changes       []metricChange  `json:"materialChanges,omitempty"`
	Ceilings      []ceilingResult `json:"resourceCeilings,omitempty"`
}

type source struct {
	Name        string             `json:"name"`
	Environment map[string]string  `json:"environment,omitempty"`
	Resources   map[string]float64 `json:"resources,omitempty"`
	Benchmarks  []benchmark        `json:"benchmarks"`
}

type benchmark struct {
	Name    string             `json:"name"`
	Metrics map[string]float64 `json:"metrics"`
}

type metricChange struct {
	Source    string  `json:"source"`
	Benchmark string  `json:"benchmark"`
	Metric    string  `json:"metric"`
	Previous  float64 `json:"previous"`
	Current   float64 `json:"current"`
	DeltaPct  float64 `json:"deltaPercent"`
	Kind      string  `json:"kind"`
}

type ceilingResult struct {
	Metric   string  `json:"metric"`
	Limit    float64 `json:"limit"`
	Observed float64 `json:"observed,omitempty"`
	Found    bool    `json:"found"`
	Breached bool    `json:"breached"`
}

func main() {
	var inputs inputFlags
	var outputPath, markdownPath, baselinePath string
	var threshold, maxTreeRSSKiB, maxCgroupMemoryBytes float64
	flag.Var(&inputs, "input", "benchmark source in NAME=PATH form; repeat for multiple sources")
	flag.StringVar(&outputPath, "output", "performance-evidence.json", "JSON evidence output")
	flag.StringVar(&markdownPath, "markdown", "performance-summary.md", "Markdown summary output")
	flag.StringVar(&baselinePath, "baseline", "", "optional prior evidence JSON")
	flag.Float64Var(&threshold, "regression-threshold-percent", 25, "material change threshold")
	flag.Float64Var(&maxTreeRSSKiB, "max-tree-rss-kib", 0, "optional process-tree RSS ceiling")
	flag.Float64Var(&maxCgroupMemoryBytes, "max-cgroup-memory-bytes", 0, "optional cgroup memory ceiling")
	flag.Parse()

	if len(inputs) == 0 || threshold <= 0 {
		fail(errors.New("at least one -input and a positive threshold are required"))
	}

	current := evidence{
		SchemaVersion: schemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Revision:      os.Getenv("GITHUB_SHA"),
		RunID:         os.Getenv("GITHUB_RUN_ID"),
		Trigger:       os.Getenv("GITHUB_EVENT_NAME"),
	}
	for _, item := range inputs {
		name, path, _ := strings.Cut(item, "=")
		file, err := os.Open(path)
		if err != nil {
			fail(fmt.Errorf("open %s input: %w", name, err))
		}
		parsed, err := parseSource(name, file)
		closeErr := file.Close()
		if err != nil {
			fail(fmt.Errorf("parse %s input: %w", name, err))
		}
		if closeErr != nil {
			fail(fmt.Errorf("close %s input: %w", name, closeErr))
		}
		current.Sources = append(current.Sources, parsed)
	}

	if baselinePath != "" {
		previous, err := readEvidence(baselinePath)
		if err != nil {
			fail(err)
		}
		current.Changes = compareEvidence(current, previous, threshold)
	}
	current.Ceilings = evaluateCeilings(current, maxTreeRSSKiB, maxCgroupMemoryBytes)

	if err := writeJSON(outputPath, current); err != nil {
		fail(err)
	}
	if err := writeMarkdown(markdownPath, current, baselinePath != ""); err != nil {
		fail(err)
	}
	for _, result := range current.Ceilings {
		if result.Breached {
			os.Exit(2)
		}
	}
}

func parseSource(name string, reader io.Reader) (source, error) {
	result := source{Name: name, Environment: map[string]string{}, Resources: map[string]float64{}}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Benchmark") {
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}
			metrics := map[string]float64{}
			for index := 2; index+1 < len(fields); index += 2 {
				value, err := strconv.ParseFloat(fields[index], 64)
				if err != nil {
					continue
				}
				metrics[fields[index+1]] = value
			}
			if len(metrics) > 0 {
				result.Benchmarks = append(result.Benchmarks, benchmark{Name: fields[0], Metrics: metrics})
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || !validKey(key) {
			continue
		}
		if numeric, err := strconv.ParseFloat(value, 64); err == nil && isResourceKey(key) {
			result.Resources[key] = numeric
		} else {
			result.Environment[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return source{}, err
	}
	if len(result.Benchmarks) == 0 {
		return source{}, errors.New("no Go benchmark measurements found")
	}
	sort.Slice(result.Benchmarks, func(i, j int) bool { return result.Benchmarks[i].Name < result.Benchmarks[j].Name })
	return result, nil
}

func validKey(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && !strings.ContainsRune("._-", character) {
			return false
		}
	}
	return true
}

func isResourceKey(key string) bool {
	if key == "capacity_samples" {
		return false
	}
	return strings.HasPrefix(key, "capacity_") || strings.HasPrefix(key, "memory.") || strings.HasPrefix(key, "cpu.")
}

func readEvidence(path string) (evidence, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return evidence{}, fmt.Errorf("read baseline: %w", err)
	}
	var result evidence
	if err := json.Unmarshal(content, &result); err != nil {
		return evidence{}, fmt.Errorf("decode baseline: %w", err)
	}
	if result.SchemaVersion != schemaVersion {
		return evidence{}, fmt.Errorf("unsupported baseline schema version %d", result.SchemaVersion)
	}
	return result, nil
}

func compareEvidence(current, previous evidence, threshold float64) []metricChange {
	prior := flatten(previous)
	var changes []metricChange
	for key, value := range flatten(current) {
		old, ok := prior[key]
		if !ok || old.value == 0 || ignoredMetric(value.metric) {
			continue
		}
		delta := (value.value - old.value) / old.value * 100
		regression := delta >= threshold
		improvement := delta <= -threshold
		if higherIsBetter(value.metric) {
			regression, improvement = delta <= -threshold, delta >= threshold
		}
		if !regression && !improvement {
			continue
		}
		kind := "improvement"
		if regression {
			kind = "regression"
		}
		changes = append(changes, metricChange{
			Source: value.source, Benchmark: value.benchmark, Metric: value.metric,
			Previous: old.value, Current: value.value, DeltaPct: delta, Kind: kind,
		})
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Kind != changes[j].Kind {
			return changes[i].Kind == "regression"
		}
		left := changes[i].Source + changes[i].Benchmark + changes[i].Metric
		right := changes[j].Source + changes[j].Benchmark + changes[j].Metric
		return left < right
	})
	return changes
}

type flatMetric struct {
	source, benchmark, metric string
	value                     float64
}

func flatten(value evidence) map[string]flatMetric {
	result := map[string]flatMetric{}
	for _, source := range value.Sources {
		for metric, measurement := range source.Resources {
			key := source.Name + "\x00(run resources)\x00" + metric
			result[key] = flatMetric{source: source.Name, benchmark: "(run resources)", metric: metric, value: measurement}
		}
		for _, benchmark := range source.Benchmarks {
			for metric, measurement := range benchmark.Metrics {
				key := source.Name + "\x00" + benchmark.Name + "\x00" + metric
				result[key] = flatMetric{source: source.Name, benchmark: benchmark.Name, metric: metric, value: measurement}
			}
		}
	}
	return result
}

func ignoredMetric(metric string) bool {
	switch metric {
	case "concurrency", "input_B/doc", "semantic_lines", "history_entries":
		return true
	default:
		return false
	}
}

func higherIsBetter(metric string) bool {
	return strings.HasSuffix(metric, "/s") || strings.HasSuffix(metric, "_per_second")
}

func evaluateCeilings(value evidence, maxTreeRSSKiB, maxCgroupMemoryBytes float64) []ceilingResult {
	limits := []struct {
		metric string
		limit  float64
	}{
		{"capacity_peak_process_tree_rss_kib", maxTreeRSSKiB},
		{"capacity_cgroup_memory_peak_bytes", maxCgroupMemoryBytes},
	}
	var results []ceilingResult
	for _, configured := range limits {
		if configured.limit <= 0 {
			continue
		}
		result := ceilingResult{Metric: configured.metric, Limit: configured.limit}
		for _, source := range value.Sources {
			if observed, ok := source.Resources[configured.metric]; ok && (!result.Found || observed > result.Observed) {
				result.Found, result.Observed = true, observed
			}
		}
		result.Breached = result.Found && result.Observed > result.Limit
		results = append(results, result)
	}
	return results
}

func writeJSON(path string, value evidence) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(content, '\n'), 0o644)
}

func writeMarkdown(path string, value evidence, compared bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	var output strings.Builder
	fmt.Fprintln(&output, "# Performance and stress evidence")
	fmt.Fprintf(&output, "\nRevision: `%s`  \nTrigger: `%s`  \nGenerated: `%s`\n", value.Revision, value.Trigger, value.GeneratedAt)
	fmt.Fprintln(&output, "\n## Trend comparison")
	if !compared {
		fmt.Fprintln(&output, "\nNo compatible prior evidence was available; this run seeds the next comparison.")
	} else if len(value.Changes) == 0 {
		fmt.Fprintln(&output, "\nNo material changes crossed the configured comparison threshold.")
	} else {
		fmt.Fprintln(&output, "\nTiming changes are advisory evidence, not correctness assertions.")
		fmt.Fprintln(&output, "\n| Result | Source | Benchmark | Metric | Previous | Current | Delta |")
		fmt.Fprintln(&output, "| --- | --- | --- | --- | ---: | ---: | ---: |")
		for _, change := range value.Changes {
			mark := "improvement"
			if change.Kind == "regression" {
				mark = "⚠ regression"
			}
			fmt.Fprintf(&output, "| %s | %s | `%s` | `%s` | %.4g | %.4g | %+.1f%% |\n",
				mark, change.Source, change.Benchmark, change.Metric, change.Previous, change.Current, change.DeltaPct)
		}
	}
	fmt.Fprintln(&output, "\n## Explicit resource ceilings")
	if len(value.Ceilings) == 0 {
		fmt.Fprintln(&output, "\nNo absolute resource ceiling was configured.")
	} else {
		fmt.Fprintln(&output, "\n| Metric | Limit | Observed | Result |")
		fmt.Fprintln(&output, "| --- | ---: | ---: | --- |")
		for _, ceiling := range value.Ceilings {
			result := "not observed"
			if ceiling.Found {
				result = "pass"
			}
			if ceiling.Breached {
				result = "FAIL"
			}
			fmt.Fprintf(&output, "| `%s` | %.4g | %.4g | %s |\n", ceiling.Metric, ceiling.Limit, ceiling.Observed, result)
		}
	}
	fmt.Fprintln(&output, "\n## Current p95 evidence")
	fmt.Fprintln(&output, "\n| Source | Benchmark | Throughput | Queue p95 ms | Service p95 ms | End-to-end p95 ms | CPU ms/op | Allocated B/op |")
	fmt.Fprintln(&output, "| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |")
	for _, source := range value.Sources {
		for _, benchmark := range source.Benchmarks {
			fmt.Fprintf(&output, "| %s | `%s` | %s | %s | %s | %s | %s | %s |\n", source.Name, benchmark.Name,
				metricValue(benchmark.Metrics, "docs/s", "transitions/s", "history_entries/s"),
				metricValue(benchmark.Metrics, "queue_p95_ms"), metricValue(benchmark.Metrics, "service_p95_ms"),
				metricValue(benchmark.Metrics, "e2e_p95_ms"), metricValue(benchmark.Metrics, "cpu_ms/doc", "cpu_ms/transition", "cpu_ms/scan"),
				metricValue(benchmark.Metrics, "alloc_B/doc", "alloc_B/transition", "alloc_B/scan"))
		}
	}
	return os.WriteFile(path, []byte(output.String()), 0o644)
}

func metricValue(metrics map[string]float64, names ...string) string {
	for _, name := range names {
		if value, ok := metrics[name]; ok {
			return strconv.FormatFloat(value, 'g', 5, 64)
		}
	}
	return "—"
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "ERROR:", err)
	os.Exit(1)
}
