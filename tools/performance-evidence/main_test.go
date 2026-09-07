package main

import (
	"strings"
	"testing"
)

func TestParseSourceCapturesBenchmarksEnvironmentAndResources(t *testing.T) {
	input := `capacity_samples=100
go_version=go1.25
BenchmarkXSDStageCapacity/valid_invoice_1000_lines/c2-8 1 2000000 ns/op 18.63 docs/s 118.5 service_p95_ms 40 queue_p95_ms 158.5 e2e_p95_ms 112 cpu_ms/doc 2000 alloc_B/doc
capacity_peak_process_tree_rss_kib=456789
`
	got, err := parseSource("host", strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if got.Environment["capacity_samples"] != "100" || got.Environment["go_version"] != "go1.25" {
		t.Fatalf("environment = %#v", got.Environment)
	}
	if got.Resources["capacity_peak_process_tree_rss_kib"] != 456789 {
		t.Fatalf("resources = %#v", got.Resources)
	}
	if len(got.Benchmarks) != 1 || got.Benchmarks[0].Metrics["e2e_p95_ms"] != 158.5 {
		t.Fatalf("benchmarks = %#v", got.Benchmarks)
	}
}

func TestCompareEvidenceFlagsDirectionAwareMaterialChanges(t *testing.T) {
	previous := evidence{Sources: []source{{Name: "host", Benchmarks: []benchmark{{Name: "BenchmarkStage", Metrics: map[string]float64{
		"docs/s": 10, "service_p95_ms": 100, "concurrency": 1,
	}}}}}}
	current := evidence{Sources: []source{{Name: "host", Benchmarks: []benchmark{{Name: "BenchmarkStage", Metrics: map[string]float64{
		"docs/s": 7, "service_p95_ms": 80, "concurrency": 2,
	}}}}}}
	changes := compareEvidence(current, previous, 20)
	if len(changes) != 2 {
		t.Fatalf("changes = %#v", changes)
	}
	if changes[0].Kind != "regression" || changes[0].Metric != "docs/s" {
		t.Fatalf("first change = %#v", changes[0])
	}
	if changes[1].Kind != "improvement" || changes[1].Metric != "service_p95_ms" {
		t.Fatalf("second change = %#v", changes[1])
	}
}

func TestEvaluateCeilingsFailsBreachesAndMissingMeasurements(t *testing.T) {
	current := evidence{Sources: []source{{Resources: map[string]float64{
		"capacity_peak_process_tree_rss_kib": 900,
	}}}}
	results := evaluateCeilings(current, 800, 2_000)
	if len(results) != 2 || !results[0].Found || !results[0].Breached || !results[0].Failed {
		t.Fatalf("breached ceiling = %#v", results)
	}
	if results[1].Found || results[1].Breached || !results[1].Failed {
		t.Fatalf("missing ceiling measurement = %#v", results)
	}
}

func TestEvaluateCeilingsPassesObservedMeasurementsWithinLimits(t *testing.T) {
	current := evidence{Sources: []source{{Resources: map[string]float64{
		"capacity_peak_process_tree_rss_kib": 700,
		"capacity_cgroup_memory_peak_bytes":  1_500,
	}}}}
	results := evaluateCeilings(current, 800, 2_000)
	if len(results) != 2 || results[0].Failed || results[1].Failed {
		t.Fatalf("ceilings = %#v", results)
	}
}
