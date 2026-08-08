package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

func runConvert(args []string) error {
	fs := newFlagSet("convert")

	output := fs.String("o", "", "output file (default: stdout)")
	format := fs.String("f", "", "output format: json, ndjson, csv (default: inferred from -o)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return errors.New("usage: auditlog convert <input> [-o output] [-f format]")
	}

	report, err := loadFile(fs.Arg(0))
	if err != nil {
		return err
	}

	fmtValue := *format
	if fmtValue == "" && *output != "" {
		fmtValue = inferFormat(*output)
	}

	if fmtValue == "" {
		fmtValue = "json"
	}

	writer, closer, err := openOutput(*output)
	if err != nil {
		return err
	}

	defer func() { _ = closer.Close() }()

	switch fmtValue {
	case "json":
		return report.WriteJSON(writer)
	case "ndjson":
		return report.WriteNDJSON(writer)
	case "csv":
		return report.WriteCSV(writer)
	case "tsv":
		return report.WriteTSV(writer)
	default:
		return fmt.Errorf("unsupported format %q (use: json, ndjson, csv, tsv)", fmtValue)
	}
}

func inferFormat(path string) string {
	switch {
	case strings.HasSuffix(path, ".json"):
		return "json"
	case strings.HasSuffix(path, ".ndjson"), strings.HasSuffix(path, ".jsonl"):
		return "ndjson"
	case strings.HasSuffix(path, ".csv"):
		return "csv"
	case strings.HasSuffix(path, ".tsv"):
		return "tsv"
	default:
		return ""
	}
}

func openOutput(path string) (io.Writer, io.Closer, error) {
	if path == "" || path == "-" {
		return os.Stdout, nopCloser{}, nil
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("create output %s: %w", path, err)
	}

	return file, file, nil
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }
