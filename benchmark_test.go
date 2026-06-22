package auditlog_test

import (
	"fmt"
	"io"
	"testing"

	"github.com/larsartmann/go-output"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// =============================================================================
// P4-26: godoc ExampleX functions for discoverability
// =============================================================================

// ExampleWorkflowReport_Duration demonstrates how to get the wall-clock
// duration of a workflow run. This is the "how long did I wait" number,
// not the sum of individual step durations (which overcounts for parallel steps).
func ExampleWorkflowReport_Duration() {
	report := auditlog.WorkflowReport{
		WallClockDurationMs: 48800,
	}

	fmt.Printf("%.1fs\n", report.WallClockDurationMs/1000)

	// Output: 48.8s
}

// ExampleWorkflowReport_Filtered demonstrates how to filter a report
// to show only failed steps for quick debugging.
func ExampleWorkflowReport_Filtered() {
	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "fetch"}, Status: auditlog.StepStatusSucceeded},
			{StepRef: auditlog.StepRef{Name: "validate"}, Status: auditlog.StepStatusFailed},
			{StepRef: auditlog.StepRef{Name: "transform"}, Status: auditlog.StepStatusSkipped},
		},
	}

	failedOnly := report.Filtered(auditlog.WithStepsByStatus(auditlog.StepStatusFailed))

	for _, s := range failedOnly.Steps {
		fmt.Println(s.Name)
	}

	// Output: validate
}

// =============================================================================
// P4-30: Benchmarks for render/export paths
// =============================================================================

// benchmarkReport builds a report with n steps for benchmarking.
func benchmarkReport(n int) auditlog.WorkflowReport {
	steps := make([]auditlog.StepInfo, n)

	for i := range steps {
		dur := float64(i * 100)

		steps[i] = auditlog.StepInfo{
			StepRef:      auditlog.StepRef{Name: "step-" + itoa(i), StepType: "BenchStep"},
			Status:       auditlog.StepStatusSucceeded,
			AttemptCount: 1,
			HasRetry:     i%3 == 0,
			HasTimeout:   i%5 == 0,
			DurationMs:   &dur,
			Dependencies: deps(i),
			Dependents:   dependents(i, n),
		}
	}

	return auditlog.WorkflowReport{
		Version:           auditlog.SchemaVersion,
		WorkflowID:        "benchmark",
		StepCount:         n,
		Steps:             steps,
		EventCount:        n * 2,
		SucceededCount:    n,
		WorkflowSucceeded: true,
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	var buf [20]byte

	pos := len(buf)

	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}

	return string(buf[pos:])
}

func deps(i int) []auditlog.StepRef {
	if i == 0 {
		return nil
	}

	return []auditlog.StepRef{{Name: "step-" + itoa(i-1)}}
}

func dependents(i, n int) []auditlog.StepRef {
	if i >= n-1 {
		return nil
	}

	return []auditlog.StepRef{{Name: "step-" + itoa(i+1)}}
}

func BenchmarkWriteD2_LargeReport(b *testing.B) {
	report := benchmarkReport(100)

	b.ResetTimer()

	for range b.N {
		_ = report.WriteD2(io.Discard)
	}
}

func BenchmarkWriteTable_LargeReport(b *testing.B) {
	report := benchmarkReport(100)

	b.ResetTimer()

	for range b.N {
		_ = report.WriteTable(io.Discard, output.FormatMarkdown, output.RenderOptions{})
	}
}

func BenchmarkWriteTree_LargeReport(b *testing.B) {
	report := benchmarkReport(100)

	b.ResetTimer()

	for range b.N {
		_ = report.WriteTree(io.Discard)
	}
}

func BenchmarkWriteJSON_LargeReport(b *testing.B) {
	report := benchmarkReport(100)

	b.ResetTimer()

	for range b.N {
		_ = report.WriteJSON(io.Discard)
	}
}

func BenchmarkWriteMermaid_LargeReport(b *testing.B) {
	report := benchmarkReport(100)

	b.ResetTimer()

	for range b.N {
		_ = report.WriteMermaid(io.Discard)
	}
}

func BenchmarkWriteJSON_SmallReport(b *testing.B) {
	report := benchmarkReport(3)

	b.ResetTimer()

	for range b.N {
		_ = report.WriteJSON(io.Discard)
	}
}
