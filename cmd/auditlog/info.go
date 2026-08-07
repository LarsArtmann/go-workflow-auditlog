package main

import (
	"fmt"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func runInfo(args []string) error {
	report, _, err := loadSingleReportSubcommand("info", args, "usage: auditlog info <file>")
	if err != nil {
		return err
	}

	fmt.Printf("version:       %s\n", report.Version)
	fmt.Printf("workflow:      %s\n", report.WorkflowID)

	if report.RunID != "" {
		fmt.Printf("run id:        %s\n", report.RunID)
	}

	fmt.Printf("exported at:   %s\n", report.ExportedAt.Format("2006-01-02 15:04:05 MST"))

	fmt.Printf("\nsteps:         %d\n", report.StepCount)
	fmt.Printf("events:        %d\n", report.EventCount)

	if report.DroppedEventCount > 0 {
		fmt.Printf("dropped:       %d (MaxEvents cap reached)\n", report.DroppedEventCount)
	}

	fmt.Println("\nstatus breakdown:")

	if report.SucceededCount > 0 {
		fmt.Printf("  succeeded:   %d\n", report.SucceededCount)
	}

	if report.FailedCount > 0 {
		fmt.Printf("  failed:      %d\n", report.FailedCount)
	}

	if report.CanceledCount > 0 {
		fmt.Printf("  canceled:    %d\n", report.CanceledCount)
	}

	if report.SkippedCount > 0 {
		fmt.Printf("  skipped:     %d\n", report.SkippedCount)
	}

	if report.PendingCount > 0 {
		fmt.Printf("  pending:     %d\n", report.PendingCount)
	}

	if report.RunningCount > 0 {
		fmt.Printf("  running:     %d\n", report.RunningCount)
	}

	fmt.Printf("\ntotal duration:   %.2f ms\n", report.TotalDurationMs)
	fmt.Printf("wall clock:       %.2f ms\n", report.WallClockDurationMs)

	if report.CriticalPathDurationMs > 0 {
		fmt.Printf("critical path:    %.2f ms\n", report.CriticalPathDurationMs)
	}

	if report.PeakConcurrency > 0 {
		fmt.Printf("peak concurrency: %d\n", report.PeakConcurrency)
	}

	fmt.Printf("succeeded:        %v\n", report.WorkflowSucceeded)

	if report.FailureSummary != "" {
		fmt.Printf("\nfailure summary:  %s\n", report.FailureSummary)
	}

	if failed := report.FailedSteps(); len(failed) > 0 {
		fmt.Printf("\nfailed steps (%d):\n", len(failed))

		for _, step := range failed {
			if step.Error != nil {
				fmt.Printf("  - %s: %s\n", step.Name, *step.Error)
			} else {
				fmt.Printf("  - %s\n", step.Name)
			}
		}
	}

	if retried := report.RetriedSteps(); len(retried) > 0 {
		fmt.Printf("\nretried steps (%d):\n", len(retried))

		for _, step := range retried {
			fmt.Printf("  - %s (attempts=%d/%d)\n", step.Name, step.AttemptCount, step.MaxAttempts)
		}
	}

	if len(report.Steps) > 0 {
		fmt.Println("\nstep list:")
		printSteps(report)
	}

	return nil
}

func printSteps(report auditlog.WorkflowReport) {
	for _, step := range report.Steps {
		dur := ""
		if step.DurationMs != nil {
			dur = fmt.Sprintf("%.2fms", *step.DurationMs)
		}

		fmt.Printf("  - %-20s [%s] %s\n", step.Name, step.Status, dur)
	}
}
