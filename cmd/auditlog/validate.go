package main

import (
	"fmt"
)

func runValidate(args []string) error {
	report, path, err := loadSingleReportSubcommand("validate", args, "usage: auditlog validate <file>")
	if err != nil {
		return err
	}

	if err := report.Validate(); err != nil {
		return fmt.Errorf("validate %s: %w", path, err)
	}

	fmt.Printf("%s: valid (schema %s, %d steps, %d events)\n", path, report.Version, report.StepCount, report.EventCount)

	return nil
}
