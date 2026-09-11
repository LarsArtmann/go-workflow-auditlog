package main

import (
	"fmt"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func runDiff(args []string) error {
	fs, err := parseFlagSet("diff", args, 2, "usage: auditlog diff <baseline> <current>")
	if err != nil {
		return err
	}

	baseline, err := loadFile(fs.Arg(0))
	if err != nil {
		return err
	}

	current, err := loadFile(fs.Arg(1))
	if err != nil {
		return err
	}

	diff := baseline.Diff(current)

	if diff.IsEmpty() {
		fmt.Println("no differences")

		return nil
	}

	fmt.Printf("workflow diff: %s -> %s\n\n", baseline.WorkflowID, current.WorkflowID)

	if len(diff.AddedSteps) > 0 {
		fmt.Printf("added steps (%d):\n", len(diff.AddedSteps))

		for _, s := range diff.AddedSteps {
			fmt.Printf("  + %s [%s]\n", s.Name, s.Status)
		}

		fmt.Println()
	}

	if len(diff.RemovedSteps) > 0 {
		fmt.Printf("removed steps (%d):\n", len(diff.RemovedSteps))

		for _, s := range diff.RemovedSteps {
			fmt.Printf("  - %s [%s]\n", s.Name, s.Status)
		}

		fmt.Println()
	}

	if len(diff.StatusChanged) > 0 {
		fmt.Printf("status changed (%d):\n", len(diff.StatusChanged))

		for _, s := range diff.StatusChanged {
			fmt.Printf("  ~ %s: %s -> %s\n", s.Name, s.OldStatus, s.Status)
		}

		fmt.Println()
	}

	printDeltaLines(diff)
	printMembershipLines(diff)

	return nil
}

// printDeltaLines prints the signed aggregate delta lines (duration, critical
// path, peak concurrency, cached steps). Positive deltas carry an explicit +
// prefix so direction is always visible.
func printDeltaLines(diff auditlog.DiffResult) {
	if diff.DurationDelta != 0 {
		fmt.Printf("total duration delta: %s%.2f ms\n", deltaSign(diff.DurationDelta), diff.DurationDelta)
	}

	if diff.CriticalPathDeltaMs != 0 {
		fmt.Printf("critical path delta:  %s%.2f ms\n", deltaSign(diff.CriticalPathDeltaMs), diff.CriticalPathDeltaMs)
	}

	if diff.PeakConcurrencyDelta != 0 {
		fmt.Printf("peak concurrency delta: %s%d\n", deltaSign(diff.PeakConcurrencyDelta), diff.PeakConcurrencyDelta)
	}

	if diff.CachedStepCountDelta != 0 {
		fmt.Printf("cached steps delta:    %s%d (results reused, not re-verified)\n",
			deltaSign(diff.CachedStepCountDelta), diff.CachedStepCountDelta)
	}
}

// printMembershipLines prints name-list membership changes (critical path and
// cached attribution) between the two runs.
func printMembershipLines(diff auditlog.DiffResult) {
	if len(diff.CriticalPathStepsAdded) > 0 {
		fmt.Printf("critical path steps added: %v\n", diff.CriticalPathStepsAdded)
	}

	if len(diff.CriticalPathStepsRemoved) > 0 {
		fmt.Printf("critical path steps removed: %v\n", diff.CriticalPathStepsRemoved)
	}

	if len(diff.CachedStepsAdded) > 0 {
		fmt.Printf("newly cached steps: %v\n", diff.CachedStepsAdded)
	}

	if len(diff.CachedStepsRemoved) > 0 {
		fmt.Printf("no longer cached steps: %v\n", diff.CachedStepsRemoved)
	}
}

// deltaSign returns the sign prefix for a delta value: "+" for zero and
// positive values (negatives carry their own "-").
func deltaSign[T int | float64](v T) string {
	if v < 0 {
		return ""
	}

	return "+"
}
