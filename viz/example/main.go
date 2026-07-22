// Command workflow-auditlog-demo demonstrates the go-workflow-auditlog library
// with a data pipeline: fetch → validate → transform → save, with retry,
// fan-out, error handling, and audit export.
package main

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/cenkalti/backoff/v4"
	errorfamily "github.com/larsartmann/go-error-family"
	"github.com/larsartmann/go-output"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	viz "github.com/larsartmann/go-workflow-auditlog/viz"
)

// version is set at build time via -ldflags "-X main.version=..." (goreleaser).
var version = "dev"

// --- Domain steps ---

type FetchStep struct {
	URL  string
	Data []byte
}

func (s *FetchStep) Do(_ context.Context) error {
	fmt.Printf("  → fetching %s\n", s.URL)
	time.Sleep(10 * time.Millisecond)

	s.Data = []byte(`{"users":[1,2,3],"status":"ok"}`)

	return nil
}

func (s *FetchStep) String() string { return "fetch" }

type ValidateStep struct {
	Input []byte
	Valid bool
}

func (s *ValidateStep) Do(_ context.Context) error {
	fmt.Println("  → validating data")
	time.Sleep(5 * time.Millisecond)

	if len(s.Input) == 0 {
		return errors.New("empty input")
	}

	s.Valid = true

	return nil
}

func (s *ValidateStep) String() string { return "validate" }

// Data returns the validated input for downstream consumption.
func (s *ValidateStep) Data() []byte { return s.Input }

type TransformStep struct {
	Input []byte
	Out   []byte
}

func (s *TransformStep) Do(_ context.Context) error {
	fmt.Println("  → transforming data")
	time.Sleep(8 * time.Millisecond)

	s.Out = []byte(`transformed:` + string(s.Input))

	return nil
}

func (s *TransformStep) String() string { return "transform" }

type SaveStep struct {
	Input []byte
	Path  string
}

func (s *SaveStep) Do(_ context.Context) error {
	fmt.Printf("  → saving to %s\n", s.Path)
	time.Sleep(5 * time.Millisecond)

	return nil
}

func (s *SaveStep) String() string { return "save" }

type NotifyStep struct {
	Msg string
}

func (s *NotifyStep) Do(_ context.Context) error { //nolint:unparam
	fmt.Printf("  → notifying: %s\n", s.Msg)

	return nil
}

func (s *NotifyStep) String() string { return "notify" }

type FlakyStep struct {
	calls int
}

// flakyFailUntil is the number of attempts after which FlakyStep succeeds.
const flakyFailUntil = 3

func (s *FlakyStep) Do(_ context.Context) error {
	s.calls++
	if s.calls < flakyFailUntil {
		return errors.New("transient error")
	}

	fmt.Println("  → flaky step succeeded on attempt", s.calls)

	return nil
}

func (s *FlakyStep) String() string { return "flaky-api-call" }

// newAuditor builds the audit log Auditor used by the demo, wiring an OnEvent
// callback that pretty-prints each event to stdout.
func newAuditor() *auditlog.Auditor {
	audit, err := auditlog.New(auditlog.Config{
		Enabled:    true,
		WorkflowID: "data-pipeline",
		OnEvent: func(e auditlog.Event) {
			phase := "▶"
			if e.IsAfter() {
				phase = "■"
			}

			fmt.Printf("  [audit] %s #%d %s attempt=%d step=%s",
				phase, e.Sequence, e.EventType, e.Attempt, e.Name)

			if e.Error != nil {
				fmt.Printf(" error=%s", *e.Error)
			}

			if e.DurationMs != nil {
				fmt.Printf(" (%.2fms)", *e.DurationMs)
			}

			fmt.Println()
		},
	})
	if err != nil {
		log.Fatalf("auditlog.New error: %v", err)
	}

	return audit
}

// buildWorkflow constructs the data pipeline DAG with retry on the flaky step.
func buildWorkflow() *flow.Workflow {
	fetch := &FetchStep{URL: "https://api.example.com/data"}
	validate := &ValidateStep{}
	transform := &TransformStep{}
	save := &SaveStep{Path: "/tmp/output.json"}
	notify := &NotifyStep{Msg: "pipeline complete"}
	flaky := &FlakyStep{}

	w := &flow.Workflow{}
	w.Add(
		// Linear pipeline: fetch → validate → transform → save.
		flow.Step(fetch),
		flow.Step(validate).DependsOn(fetch).Input(func(_ context.Context, v *ValidateStep) error {
			v.Input = fetch.Data

			return nil
		}),
		flow.Step(transform).DependsOn(validate).Input(func(_ context.Context, t *TransformStep) error {
			t.Input = validate.Data()

			return nil
		}),
		flow.Step(save).DependsOn(transform).Input(func(_ context.Context, s *SaveStep) error {
			s.Input = transform.Out

			return nil
		}),

		// Notify runs after save succeeds.
		flow.Step(notify).DependsOn(save),

		// A flaky step with retry (demonstrates attempt tracking).
		// We pass a FRESH backoff instance to avoid the data race in
		// go-workflow's shared DefaultRetryOption.Backoff.
		flow.Step(flaky).DependsOn(fetch).Retry(func(o *flow.RetryOption) {
			o.Attempts = 5
			o.Backoff = backoff.NewExponentialBackOff()
		}),
	)

	return w
}

// printReportSummary prints the high-level report counters.
func printReportSummary(report auditlog.WorkflowReport) {
	fmt.Println()
	fmt.Println("━━━ Audit Report ━━━")
	fmt.Printf("Workflow:     %s\n", report.WorkflowID)
	fmt.Printf("Steps:        %d\n", report.StepCount)
	fmt.Printf("Succeeded:    %d\n", report.SucceededCount)
	fmt.Printf("Failed:       %d\n", report.FailedCount)
	fmt.Printf("Skipped:      %d\n", report.SkippedCount)
	fmt.Printf("Canceled:     %d\n", report.CanceledCount)
	fmt.Printf("Events:       %d\n", report.EventCount)
	fmt.Printf("Total time:   %.2fms\n", report.TotalDurationMs)
	fmt.Printf("Succeeded:    %v\n", report.WorkflowSucceeded)
}

// printStepDetails prints per-step information including timing, deps, and errors.
func printStepDetails(report auditlog.WorkflowReport) {
	fmt.Println()
	fmt.Println("━━━ Step Details ━━━")

	for _, step := range report.Steps {
		icon := step.Status.Icon()
		fmt.Printf("  %s %s [%s] attempts=%d type=%s",
			icon, step.Name, step.Status, step.AttemptCount, step.StepType)

		if step.DurationMs != nil {
			fmt.Printf(" (%.2fms)", *step.DurationMs)
		}

		if len(step.Dependencies) > 0 {
			fmt.Printf(" deps=%v", step.Dependencies)
		}

		if step.HasRetry {
			fmt.Printf(" retry(max=%d)", step.MaxAttempts)
		}

		if step.HasTimeout {
			fmt.Printf(" timeout")
		}

		if step.Error != nil {
			fmt.Printf(" error=%s", *step.Error)
		}

		fmt.Println()
	}
}

// maybeExport writes all export artifacts if --export is set.
func maybeExport(audit *auditlog.Auditor, args []string, report auditlog.WorkflowReport) {
	if len(args) <= 1 || args[1] != "--export" {
		return
	}

	type exportTask struct {
		name string
		fn   func() error
	}

	tasks := []exportTask{
		{"audit-report.json", func() error { return audit.ExportJSON("audit-report.json") }},
		{"audit-events.ndjson", func() error { return audit.ExportNDJSON("audit-events.ndjson") }},
		{"dag.mmd", func() error {
			return viz.ExportMermaid(report, "dag.mmd",
				viz.WithDirection(output.DirectionRight))
		}},
		{"dag.dot", func() error { return viz.ExportGraphviz(report, "dag.dot") }},
		{"dag.puml", func() error { return viz.ExportPlantUML(report, "dag.puml") }},
		{"dag.d2", func() error { return viz.ExportD2(report, "dag.d2") }},
		{"steps.csv", func() error {
			return viz.ExportTable(report, "steps.csv", output.FormatCSV, output.RenderOptions{})
		}},
		{"steps-compact.md", func() error {
			return viz.ExportTable(report, "steps-compact.md", output.FormatMarkdown, output.RenderOptions{},
				viz.WithColumns(
					viz.ColumnStep, viz.ColumnStatus, viz.ColumnDuration,
				))
		}},
		{"tree.txt", func() error { return viz.ExportTree(report, "tree.txt") }},
		{"dashboard.html", func() error { return viz.ExportHTML(report, "dashboard.html") }},
	}

	for _, task := range tasks {
		err := task.fn()
		if err != nil {
			log.Fatalf("export %s: %v", task.name, err)
		}

		fmt.Printf("Exported %s\n", task.name)
	}

	printSampleEvent(report)
}

// printSampleEvent pretty-prints the first captured event as JSON.
func printSampleEvent(report auditlog.WorkflowReport) {
	if len(report.Events) == 0 {
		return
	}

	sample, err := json.Marshal(report.Events[0],
		jsontext.WithIndentPrefix(""),
		jsontext.WithIndent("  "),
		jsontext.EscapeForHTML(true),
		jsontext.EscapeForJS(true),
	)
	if err != nil {
		log.Printf("marshal sample event: %v", err)

		return
	}

	fmt.Printf("\nSample event:\n%s\n", sample)
}

// printErrorClassification demonstrates go-error-family integration: every
// auditlog sentinel is automatically classified, so consumers get IsRetryable,
// ExitCode, and Classify without importing go-error-family themselves.
func printErrorClassification() {
	fmt.Println()
	fmt.Println("━━━ Error Classification ━━━")
	fmt.Println("(via github.com/larsartmann/go-error-family)")

	demos := []struct {
		label string
		err   error
	}{
		{"Corruption  (exit 65)", fmt.Errorf("%w: got 5, want 3", auditlog.ErrEventCountMismatch)},
		{"Rejection   (exit  1)", auditlog.ErrEmpty},
		{"Transient   (exit 75)", auditlog.ErrReportLoadFailed},
		{"Infra       (exit 69)", auditlog.ErrRenderFailed},
	}

	for _, d := range demos {
		fmt.Printf("  %s → family=%s retryable=%v exit=%d\n",
			d.label, errorfamily.Classify(d.err), errorfamily.IsRetryable(d.err), errorfamily.ExitCode(d.err))
	}
}

func main() {
	ctx := context.Background()

	audit := newAuditor()
	w := buildWorkflow()

	// Attach audit callbacks BEFORE running.
	audit.Attach(w)

	// Run the workflow.
	fmt.Println("━━━ Running data pipeline workflow ━━━")
	fmt.Printf("━━━ demo version: %s | run id: %s ━━━\n", version, audit.RunID())

	start := time.Now()
	runErr := w.Do(ctx)
	elapsed := time.Since(start)
	fmt.Printf("━━━ Workflow completed in %v ━━━\n", elapsed)

	// Snapshot final state AFTER running.
	audit.Snapshot(w)

	// Print the report.
	report := audit.Report()
	printReportSummary(report)
	printStepDetails(report)
	printErrorClassification()

	if runErr != nil {
		fmt.Printf("\nWorkflow error: %v\n", runErr)
	}

	maybeExport(audit, os.Args, report)
}
