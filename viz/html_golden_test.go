package viz_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
	viz "github.com/larsartmann/go-workflow-auditlog/viz"
)

// goldenExportedAt is a fixed timestamp so the rendered HTML is byte-for-byte
// reproducible across runs and machines.
var goldenExportedAt = time.Date(2026, 1, 15, 9, 30, 0, 0, time.UTC)

// goldenDurations provides stable duration pointers for the golden report.
var (
	goldenFetchMs     float64 = 5.2
	goldenTransformMs float64 = 8.7
	goldenSaveMs      float64 = 3.1
)

// goldenHTMLReport builds a deterministic, valid WorkflowReport with a
// fetch → transform → save pipeline and fixed timestamps so the golden
// test is stable across runs.
func goldenHTMLReport() auditlog.WorkflowReport {
	fetchStarted := goldenExportedAt
	fetchFinished := goldenExportedAt.Add(5 * time.Millisecond)
	transformStarted := fetchFinished
	transformFinished := transformStarted.Add(9 * time.Millisecond)
	saveStarted := transformFinished
	saveFinished := saveStarted.Add(3 * time.Millisecond)

	return auditlog.WorkflowReport{
		Version:             auditlog.SchemaVersion,
		WorkflowID:          "golden-pipeline",
		RunID:               "abcdef0123456789abcdef0123456789",
		ExportedAt:          goldenExportedAt,
		StepCount:           3,
		SucceededCount:      3,
		EventCount:          6,
		WallClockDurationMs: 17.0,
		TotalDurationMs:     17.0,
		Steps: []auditlog.StepInfo{
			{
				StepRef:      auditlog.StepRef{Name: "fetch", StepType: "FetchStep"},
				StepID:       1,
				Status:       auditlog.StepStatusSucceeded,
				AttemptCount: 1,
				StartedAt:    &fetchStarted,
				FinishedAt:   &fetchFinished,
				DurationMs:   &goldenFetchMs,
				Dependents:   []auditlog.StepRef{{Name: "transform"}},
			},
			{
				StepRef:      auditlog.StepRef{Name: "transform", StepType: "TransformStep"},
				StepID:       2,
				Status:       auditlog.StepStatusSucceeded,
				AttemptCount: 1,
				StartedAt:    &transformStarted,
				FinishedAt:   &transformFinished,
				DurationMs:   &goldenTransformMs,
				Dependencies: []auditlog.StepRef{{Name: "fetch"}},
				Dependents:   []auditlog.StepRef{{Name: "save"}},
			},
			{
				StepRef:      auditlog.StepRef{Name: "save", StepType: "SaveStep"},
				StepID:       3,
				Status:       auditlog.StepStatusSucceeded,
				AttemptCount: 1,
				StartedAt:    &saveStarted,
				FinishedAt:   &saveFinished,
				DurationMs:   &goldenSaveMs,
				Dependencies: []auditlog.StepRef{{Name: "transform"}},
			},
		},
		Events: []auditlog.Event{
			{
				StepRef:   auditlog.StepRef{Name: "fetch", StepType: "FetchStep"},
				RunID:     "abcdef0123456789abcdef0123456789",
				Sequence:  1,
				Timestamp: fetchStarted,
				EventType: auditlog.EventTypeAttemptStart,
				Phase:     auditlog.PhaseBefore,
				Attempt:   1,
			},
			{
				StepRef:    auditlog.StepRef{Name: "fetch", StepType: "FetchStep"},
				RunID:      "abcdef0123456789abcdef0123456789",
				Sequence:   2,
				Timestamp:  fetchFinished,
				EventType:  auditlog.EventTypeAttemptEnd,
				Phase:      auditlog.PhaseAfter,
				Attempt:    1,
				DurationMs: &goldenFetchMs,
				Status:     auditlog.StepStatusSucceeded,
			},
			{
				StepRef:   auditlog.StepRef{Name: "transform", StepType: "TransformStep"},
				RunID:     "abcdef0123456789abcdef0123456789",
				Sequence:  3,
				Timestamp: transformStarted,
				EventType: auditlog.EventTypeAttemptStart,
				Phase:     auditlog.PhaseBefore,
				Attempt:   1,
			},
			{
				StepRef:    auditlog.StepRef{Name: "transform", StepType: "TransformStep"},
				RunID:      "abcdef0123456789abcdef0123456789",
				Sequence:   4,
				Timestamp:  transformFinished,
				EventType:  auditlog.EventTypeAttemptEnd,
				Phase:      auditlog.PhaseAfter,
				Attempt:    1,
				DurationMs: &goldenTransformMs,
				Status:     auditlog.StepStatusSucceeded,
			},
			{
				StepRef:   auditlog.StepRef{Name: "save", StepType: "SaveStep"},
				RunID:     "abcdef0123456789abcdef0123456789",
				Sequence:  5,
				Timestamp: saveStarted,
				EventType: auditlog.EventTypeAttemptStart,
				Phase:     auditlog.PhaseBefore,
				Attempt:   1,
			},
			{
				StepRef:    auditlog.StepRef{Name: "save", StepType: "SaveStep"},
				RunID:      "abcdef0123456789abcdef0123456789",
				Sequence:   6,
				Timestamp:  saveFinished,
				EventType:  auditlog.EventTypeAttemptEnd,
				Phase:      auditlog.PhaseAfter,
				Attempt:    1,
				DurationMs: &goldenSaveMs,
				Status:     auditlog.StepStatusSucceeded,
			},
		},
	}
}

// TestReport_WriteHTML_GoldenContent renders the deterministic golden report
// and validates its structural and semantic content. This replaces the former
// byte-for-byte golden file comparison, which broke on every CSS/JS whitespace
// change or dependency update without catching real bugs.
func TestReport_WriteHTML_GoldenContent(t *testing.T) {
	t.Parallel()

	report := goldenHTMLReport()

	var buf bytes.Buffer

	err := viz.WriteHTML(report, &buf)
	if err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}

	html := buf.String()

	// --- Structural integrity ---
	if !strings.HasPrefix(html, "<!DOCTYPE html>") {
		t.Error("expected output to start with <!DOCTYPE html>")
	}

	for _, tag := range []string{"<html", "<head>", "<body>", "</html>", "Content-Security-Policy"} {
		if !strings.Contains(html, tag) {
			t.Errorf("expected %q in HTML output", tag)
		}
	}

	if openScripts := strings.Count(html, "<script"); openScripts != 5 {
		t.Errorf("expected exactly 5 <script> tags, got %d", openScripts)
	}

	if closeScripts := strings.Count(html, "</script>"); closeScripts != 5 {
		t.Errorf("expected exactly 5 </script> tags, got %d", closeScripts)
	}

	// --- JSON data blocks present ---
	for _, id := range []string{`id="report-data"`, `id="type-metadata"`, `id="dag-data"`} {
		if !strings.Contains(html, id) {
			t.Errorf("expected JSON data block %q in HTML output", id)
		}
	}

	// --- All 5 dashboard tabs present ---
	for _, tabID := range []string{`id="tab-steps"`, `id="tab-tree"`, `id="tab-graph"`, `id="tab-timeline"`, `id="tab-events"`} {
		if !strings.Contains(html, tabID) {
			t.Errorf("expected tab panel %q in HTML output", tabID)
		}
	}

	// --- Golden report content injected ---
	for _, stepName := range []string{"fetch", "transform", "save"} {
		if !strings.Contains(html, stepName) {
			t.Errorf("expected step name %q in HTML output (report JSON)", stepName)
		}
	}

	if !strings.Contains(html, "golden-pipeline") {
		t.Error("expected WorkflowID \"golden-pipeline\" in HTML output")
	}

	if !strings.Contains(html, "abcdef0123456789abcdef0123456789") {
		t.Error("expected RunID in HTML output")
	}

	if !strings.Contains(html, auditlog.SchemaVersion) {
		t.Error("expected schema version in HTML output")
	}

	// --- CSS and JS embedded ---
	if !strings.Contains(html, "<style>") || !strings.Contains(html, "</style>") {
		t.Error("expected <style> block with embedded CSS")
	}

	// Check for a known CSS variable to confirm dashboard.css is embedded
	if !strings.Contains(html, "--success") {
		t.Error("expected dashboard CSS variables in embedded <style> block")
	}

	// Check for a known JS function/variable to confirm dashboard.js is embedded
	if !strings.Contains(html, "addEventListener") {
		t.Error("expected dashboard JS in embedded <script> block")
	}

	// --- CSP policy is strict ---
	csp := "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'"
	if !strings.Contains(html, csp) {
		t.Errorf("expected strict CSP policy %q in HTML output", csp)
	}

	// --- Graph visualization enhancements present ---
	for _, marker := range []string{
		`id="graph-search"`,        // search/filter input
		`id="graph-critical-path"`, // critical path toggle button
		"computeCriticalPathSteps", // critical path algorithm
		"enhanceGraph",             // post-render enhancement function
		"critical-path-bar",        // Gantt timeline critical path CSS class
	} {
		if !strings.Contains(html, marker) {
			t.Errorf("expected graph enhancement marker %q in HTML output", marker)
		}
	}

	// --- Duration labels in DAG node data ---
	// The golden report has fetch=5.2ms, transform=8.7ms, save=3.1ms
	// These should appear as compact labels in the dag-data JSON.
	for _, dur := range []string{"5ms", "9ms", "3ms"} {
		if !strings.Contains(html, dur) {
			t.Errorf("expected compact duration label %q in DAG data", dur)
		}
	}
}
