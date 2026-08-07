package main

import (
	"fmt"
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

	if diff.DurationDelta != 0 {
		sign := "+"
		if diff.DurationDelta < 0 {
			sign = ""
		}

		fmt.Printf("total duration delta: %s%.2f ms\n", sign, diff.DurationDelta)
	}

	if diff.CriticalPathDeltaMs != 0 {
		sign := "+"
		if diff.CriticalPathDeltaMs < 0 {
			sign = ""
		}

		fmt.Printf("critical path delta:  %s%.2f ms\n", sign, diff.CriticalPathDeltaMs)
	}

	if diff.PeakConcurrencyDelta != 0 {
		sign := "+"
		if diff.PeakConcurrencyDelta < 0 {
			sign = ""
		}

		fmt.Printf("peak concurrency delta: %s%d\n", sign, diff.PeakConcurrencyDelta)
	}

	if len(diff.CriticalPathStepsAdded) > 0 {
		fmt.Printf("critical path steps added: %v\n", diff.CriticalPathStepsAdded)
	}

	if len(diff.CriticalPathStepsRemoved) > 0 {
		fmt.Printf("critical path steps removed: %v\n", diff.CriticalPathStepsRemoved)
	}

	return nil
}
