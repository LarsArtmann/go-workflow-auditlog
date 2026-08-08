package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// loadFile loads a report from path (auto-detecting JSON vs NDJSON).
// A path of "-" reads from stdin. For NDJSON input, events are replayed
// into a report via ReplayEvents.
func loadFile(path string) (auditlog.WorkflowReport, error) {
	reader, closer, err := openPath(path)
	if err != nil {
		return auditlog.WorkflowReport{}, err
	}

	defer func() { _ = closer.Close() }()

	return detectAndLoad(reader, path)
}

func openPath(
	path string,
) (reader interface{ Read([]byte) (int, error) }, closer interface{ Close() error }, err error) {
	if path == "-" {
		return os.Stdin, os.Stdin, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", path, err)
	}

	return file, file, nil
}

func detectAndLoad(reader interface{ Read([]byte) (int, error) }, path string) (auditlog.WorkflowReport, error) {
	if isNDJSONPath(path) {
		events, err := auditlog.ReadEvents(reader)
		if err != nil {
			return auditlog.WorkflowReport{}, fmt.Errorf("read ndjson events: %w", err)
		}

		report, err := auditlog.ReplayEvents(events)
		if err != nil {
			return auditlog.WorkflowReport{}, fmt.Errorf("replay events: %w", err)
		}

		return report, nil
	}

	report, err := auditlog.LoadReportFromReader(reader)
	if err != nil {
		return auditlog.WorkflowReport{}, fmt.Errorf("load report: %w", err)
	}

	return report, nil
}

func isNDJSONPath(path string) bool {
	return strings.HasSuffix(path, ".ndjson") || strings.HasSuffix(path, ".jsonl")
}

func parseFlagSet(name string, args []string, expectedNArg int, usage string) (*flag.FlagSet, error) {
	fs := newFlagSet(name)

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if fs.NArg() != expectedNArg {
		return nil, errors.New(usage)
	}

	return fs, nil
}

func loadSingleReportSubcommand(name string, args []string, usage string) (auditlog.WorkflowReport, string, error) {
	fs, err := parseFlagSet(name, args, 1, usage)
	if err != nil {
		return auditlog.WorkflowReport{}, "", err
	}

	report, err := loadFile(fs.Arg(0))
	if err != nil {
		return auditlog.WorkflowReport{}, "", err
	}

	return report, fs.Arg(0), nil
}
