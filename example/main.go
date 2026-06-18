// Command workflow-auditlog-demo demonstrates the go-workflow-auditlog library
// with a data pipeline: fetch → validate → transform → save, with retry,
// fan-out, error handling, and audit export.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/cenkalti/backoff/v4"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

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

func (s *NotifyStep) Do(_ context.Context) error {
	fmt.Printf("  → notifying: %s\n", s.Msg)
	return nil
}

func (s *NotifyStep) String() string { return "notify" }

type FlakyStep struct {
	calls int
}

func (s *FlakyStep) Do(_ context.Context) error {
	s.calls++
	if s.calls < 3 {
		return errors.New("transient error")
	}
	fmt.Println("  → flaky step succeeded on attempt", s.calls)
	return nil
}

func (s *FlakyStep) String() string { return "flaky-api-call" }

func main() {
	ctx := context.Background()

	// Create the auditor.
	audit, err := auditlog.New(auditlog.Config{
		Enabled:    true,
		WorkflowID: "data-pipeline",
		OnEvent: func(e auditlog.Event) {
			phase := "▶"
			if e.IsAfter() {
				phase = "■"
			}
			fmt.Printf("  [audit] %s #%d %s attempt=%d step=%s",
				phase, e.Sequence, e.EventType, e.Attempt, e.StepName)
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

	// Build the workflow.
	fetch := &FetchStep{URL: "https://api.example.com/data"}
	validate := &ValidateStep{}
	transform := &TransformStep{}
	save := &SaveStep{Path: "/tmp/output.json"}
	notify := &NotifyStep{Msg: "pipeline complete"}
	flaky := &FlakyStep{}

	w := &flow.Workflow{}
	w.Add(
		// Linear pipeline: fetch → validate → transform → save
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
		flow.Step(flaky).DependsOn(fetch).Retry(func(o *flow.RetryOption) {
			o.Attempts = 5
			o.Backoff = backoff.NewExponentialBackOff()
		}),
	)

	// Attach audit callbacks BEFORE running.
	audit.Attach(w)

	// Run the workflow.
	fmt.Println("━━━ Running data pipeline workflow ━━━")
	start := time.Now()
	runErr := w.Do(ctx)
	elapsed := time.Since(start)
	fmt.Printf("━━━ Workflow completed in %v ━━━\n", elapsed)

	// Snapshot final state AFTER running.
	audit.Snapshot(w)

	// Print the report.
	report := audit.Report()
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

	fmt.Println()
	fmt.Println("━━━ Step Details ━━━")
	for _, step := range report.Steps {
		icon := step.Status.Icon()
		fmt.Printf("  %s %s [%s] attempts=%d type=%s",
			icon, step.Name, step.Status, step.AttemptCount, step.Type)

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

	if runErr != nil {
		fmt.Printf("\nWorkflow error: %v\n", runErr)
	}

	// Export to JSON.
	if len(os.Args) > 1 && os.Args[1] == "--export" {
		path := "audit-report.json"
		err := audit.ExportToFile(path)
		if err != nil {
			log.Fatalf("export error: %v", err)
		}
		fmt.Printf("\nReport exported to %s\n", path)

		ndjsonPath := "audit-events.ndjson"
		err = audit.ExportEventsToNDJSON(ndjsonPath)
		if err != nil {
			log.Fatalf("ndjson export error: %v", err)
		}
		fmt.Printf("Events exported to %s\n", ndjsonPath)
	}

	// Pretty-print a sample event.
	if len(report.Events) > 0 {
		sample, _ := json.MarshalIndent(report.Events[0], "", "  ")
		fmt.Printf("\nSample event:\n%s\n", sample)
	}
}
