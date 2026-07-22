package viz_test

import (
	"fmt"
	"io"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
	viz "github.com/larsartmann/go-workflow-auditlog/viz"
)

// BenchmarkRenderHTML_LargeReport measures renderHTML throughput with a
// 1000-step report — the worst-case payload for the JSON marshal + template
// formatting pipeline.
func BenchmarkRenderHTML_LargeReport(b *testing.B) {
	steps := make([]auditlog.StepInfo, 0, 1000)

	for i := range 1000 {
		dur := float64(i) * 0.1

		steps = append(steps, auditlog.StepInfo{
			StepRef:      auditlog.StepRef{Name: fmt.Sprintf("step-%04d", i), StepType: "BenchStep"},
			StepID:       i + 1,
			Status:       auditlog.StepStatusSucceeded,
			AttemptCount: 1,
			DurationMs:   &dur,
			Dependencies: []auditlog.StepRef{{Name: fmt.Sprintf("step-%04d", i-1)}},
		})
	}

	steps[0].Dependencies = nil // root step has no deps

	report := auditlog.WorkflowReport{
		Version:        auditlog.SchemaVersion,
		WorkflowID:     "bench-large",
		StepCount:      1000,
		SucceededCount: 1000,
		Steps:          steps,
	}

	b.ResetTimer()

	for range b.N {
		err := viz.WriteHTML(report, io.Discard)
		if err != nil {
			b.Fatalf("WriteHTML: %v", err)
		}
	}
}

// BenchmarkRenderHTML_SmallReport measures renderHTML throughput with a
// typical 5-step report for baseline comparison.
func BenchmarkRenderHTML_SmallReport(b *testing.B) {
	dur := 5.0
	report := auditlog.WorkflowReport{
		Version:        auditlog.SchemaVersion,
		WorkflowID:     "bench-small",
		StepCount:      3,
		SucceededCount: 3,
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "fetch"}, Status: auditlog.StepStatusSucceeded, DurationMs: &dur},
			{StepRef: auditlog.StepRef{Name: "transform"}, Status: auditlog.StepStatusSucceeded, DurationMs: &dur},
			{StepRef: auditlog.StepRef{Name: "save"}, Status: auditlog.StepStatusSucceeded, DurationMs: &dur},
		},
	}

	b.ResetTimer()

	for range b.N {
		err := viz.WriteHTML(report, io.Discard)
		if err != nil {
			b.Fatalf("WriteHTML: %v", err)
		}
	}
}
