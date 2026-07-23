package viz_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/larsartmann/go-output"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
	viz "github.com/larsartmann/go-workflow-auditlog/viz"
)

// addSucceedSteps appends n identical succeeding steps named "step-<j>" to the
// workflow.
func addSucceedSteps(w *flow.Workflow, n int) {
	for j := range n {
		s := testhelpers.NewSucceed(fmt.Sprintf("step-%d", j))
		w.Add(flow.Step(s))
	}
}

// BenchmarkInvocation measures the overhead of audit callbacks on a single
// step invocation (the hot path).
func BenchmarkInvocation(b *testing.B) {
	b.Run("disabled", func(b *testing.B) {
		a, _ := auditlog.New(auditlog.Config{Enabled: false})

		for b.Loop() {
			b.StopTimer()

			w := &flow.Workflow{}
			w.Add(flow.Step(testhelpers.NewSucceed("step")))
			a.Attach(w)
			b.StartTimer()

			_ = w.Do(context.Background())
		}
	})

	b.Run("enabled", func(b *testing.B) {
		a, _ := auditlog.New(auditlog.Config{Enabled: true})

		for b.Loop() {
			b.StopTimer()

			w := &flow.Workflow{}
			w.Add(flow.Step(testhelpers.NewSucceed("step")))
			a.Attach(w)
			b.StartTimer()

			_ = w.Do(context.Background())
			a.Snapshot(w)
		}
	})
}

// BenchmarkRegistration measures the cost of Attach (callback injection) with
// varying numbers of steps.
func BenchmarkAttach(b *testing.B) {
	for _, n := range []int{10, 50, 100} {
		b.Run(fmt.Sprintf("%d-steps", n), func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()

				a, _ := auditlog.New(auditlog.Config{Enabled: true})
				w := &flow.Workflow{}

				addSucceedSteps(w, n)

				b.StartTimer()

				a.Attach(w)
			}
		})
	}
}

// BenchmarkBuildReport measures report assembly from varying step counts.
func BenchmarkBuildReport(b *testing.B) {
	for _, n := range []int{50, 100, 500} {
		b.Run(fmt.Sprintf("%d-steps", n), func(b *testing.B) {
			a, _ := auditlog.New(auditlog.Config{Enabled: true})
			w := &flow.Workflow{}

			addSucceedSteps(w, n)

			a.Attach(w)
			_ = w.Do(context.Background())
			a.Snapshot(w)

			b.ResetTimer()

			for b.Loop() {
				_ = a.Report()
			}
		})
	}
}

// BenchmarkEventsCopy measures the cost of copying the events slice.
func BenchmarkEventsCopy(b *testing.B) {
	a, _ := auditlog.New(auditlog.Config{Enabled: true})

	w := &flow.Workflow{}
	for j := range 100 {
		w.Add(flow.Step(testhelpers.NewSucceed(fmt.Sprintf("step-%d", j))))
	}

	a.Attach(w)
	_ = w.Do(context.Background())
	a.Snapshot(w)

	b.ResetTimer()

	for b.Loop() {
		_ = a.Events()
	}
}

// BenchmarkOnEventCallback measures the overhead of the OnEvent callback.
func BenchmarkOnEventCallback(b *testing.B) {
	a, _ := auditlog.New(auditlog.Config{
		Enabled: true,
		OnEvent: func(e auditlog.Event) {}, // no-op callback
	})

	for b.Loop() {
		b.StopTimer()

		w := &flow.Workflow{}
		w.Add(flow.Step(testhelpers.NewSucceed("step")))
		a.Attach(w)
		b.StartTimer()

		_ = w.Do(context.Background())
	}
}

// BenchmarkRetryWithAudit measures the cost of auditing retried steps.
func BenchmarkRetryWithAudit(b *testing.B) {
	a, _ := auditlog.New(auditlog.Config{Enabled: true})

	for b.Loop() {
		b.StopTimer()

		w := &flow.Workflow{}
		step := &testhelpers.FlakyStep{Name: "bench-flaky", FailUntil: 2}
		testhelpers.AddRetryStep(w, step, 5)
		a.Attach(w)
		b.StartTimer()

		_ = w.Do(context.Background())
		a.Snapshot(w)
	}
}

// BenchmarkMermaidExport measures the cost of generating Mermaid diagrams.
func BenchmarkMermaidExport(b *testing.B) {
	for _, n := range []int{10, 50, 100} {
		b.Run(fmt.Sprintf("%d-steps", n), func(b *testing.B) {
			a, _ := auditlog.New(auditlog.Config{Enabled: true})
			w := &flow.Workflow{}

			var prev flow.Steper

			for j := range n {
				s := testhelpers.NewSucceed(fmt.Sprintf("step-%d", j))
				if prev != nil {
					w.Add(flow.Step(s).DependsOn(prev))
				} else {
					w.Add(flow.Step(s))
				}

				prev = s
			}

			a.Attach(w)
			_ = w.Do(context.Background())
			a.Snapshot(w)

			report := a.Report()

			b.ResetTimer()

			for b.Loop() {
				_ = viz.WriteMermaid(report, &discardWriter{})
			}
		})
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// =============================================================================
// Godoc Examples
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

// ExampleWorkflowReport_peakConcurrency demonstrates how to read the peak
// concurrency metric — the maximum number of step attempts that were in-flight
// simultaneously. This helps right-size worker pools and detect contention.
func ExampleWorkflowReport_peakConcurrency() {
	report := auditlog.WorkflowReport{
		PeakConcurrency: 4,
	}

	fmt.Printf("At most %d steps ran at the same time\n", report.PeakConcurrency)

	// Output: At most 4 steps ran at the same time
}

// ExampleWorkflowReport_criticalPathDurationMs demonstrates how to read the
// critical-path duration — the longest dependency-chain duration. This is the
// theoretical minimum wall-clock time with perfect parallelization, so it
// identifies the bottleneck path through the DAG.
func ExampleWorkflowReport_criticalPathDurationMs() {
	report := auditlog.WorkflowReport{
		TotalDurationMs:        3000, // sum of all step durations
		CriticalPathDurationMs: 1200, // longest single chain
	}

	fmt.Printf("Bottleneck path: %.0fms (sum was %.0fms)\n",
		report.CriticalPathDurationMs, report.TotalDurationMs)

	// Output: Bottleneck path: 1200ms (sum was 3000ms)
}

// ExampleWorkflowReport_wallClockDurationMs demonstrates how to read the
// wall-clock duration field directly. This is the actual elapsed time from
// the earliest to the latest event — the "how long did I wait" number —
// distinct from TotalDurationMs which sums individual step durations.
func ExampleWorkflowReport_wallClockDurationMs() {
	report := auditlog.WorkflowReport{
		TotalDurationMs:     3000, // inflated: sums parallel steps
		WallClockDurationMs: 1500, // real: earliest event → latest event
	}

	fmt.Printf("Real elapsed time: %.0fms (not %.0fms)\n",
		report.WallClockDurationMs, report.TotalDurationMs)

	// Output: Real elapsed time: 1500ms (not 3000ms)
}

// ExampleWorkflowReport_WriteTable demonstrates selecting a subset of columns
// for table export using WithColumns.
func ExampleWriteTable() {
	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{
				StepRef:  auditlog.StepRef{Name: "fetch"},
				Status:   auditlog.StepStatusSucceeded,
				HasRetry: true,
			},
		},
	}

	out, _ := viz.WriteTableString(report,
		output.FormatCSV, output.RenderOptions{},
		viz.WithColumns(viz.ColumnStep, viz.ColumnStatus),
	)

	fmt.Println(out)

	// Output:
	// Step,Status
	// fetch,succeeded
}

// ExampleWorkflowReport_WriteMermaid demonstrates setting a left-to-right
// layout direction on a Mermaid diagram using WithDirection.
func ExampleWriteMermaid() {
	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "a"}},
			{StepRef: auditlog.StepRef{Name: "b"}, Dependencies: []auditlog.StepRef{{Name: "a"}}},
		},
	}

	out, _ := viz.WriteMermaidString(report,
		viz.WithDirection(output.DirectionRight),
	)

	fmt.Println(strings.Contains(out, "flowchart LR"))

	// Output: true
}

// =============================================================================
// Export/Render Benchmarks
// =============================================================================

// benchmarkReport builds a report with n steps for benchmarking.
func benchmarkReport(n int) auditlog.WorkflowReport {
	steps := make([]auditlog.StepInfo, 0, n)

	for i := range n {
		dur := float64(i * 100)

		steps = append(steps, auditlog.StepInfo{
			StepRef:      auditlog.StepRef{Name: "step-" + itoa(i), StepType: "BenchStep"},
			Status:       auditlog.StepStatusSucceeded,
			AttemptCount: 1,
			HasRetry:     i%3 == 0,
			HasTimeout:   i%5 == 0,
			DurationMs:   &dur,
			Dependencies: deps(i),
			Dependents:   dependents(i, n),
		})
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

	for b.Loop() {
		_ = viz.WriteD2(report, io.Discard)
	}
}

func BenchmarkWriteTable_LargeReport(b *testing.B) {
	report := benchmarkReport(100)

	b.ResetTimer()

	for b.Loop() {
		_ = viz.WriteTable(report, io.Discard, output.FormatMarkdown, output.RenderOptions{})
	}
}

func BenchmarkWriteTree_LargeReport(b *testing.B) {
	report := benchmarkReport(100)

	b.ResetTimer()

	for b.Loop() {
		_ = viz.WriteTree(report, io.Discard)
	}
}

func BenchmarkWriteJSON_LargeReport(b *testing.B) {
	report := benchmarkReport(100)

	b.ResetTimer()

	for b.Loop() {
		_ = report.WriteJSON(io.Discard)
	}
}

func BenchmarkWriteMermaid_LargeReport(b *testing.B) {
	report := benchmarkReport(100)

	b.ResetTimer()

	for b.Loop() {
		_ = viz.WriteMermaid(report, io.Discard)
	}
}

func BenchmarkWriteJSON_SmallReport(b *testing.B) {
	report := benchmarkReport(3)

	b.ResetTimer()

	for b.Loop() {
		_ = report.WriteJSON(io.Discard)
	}
}
