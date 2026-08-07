// Package main implements the auditlog CLI: report conversion, inspection,
// diffing and validation built on the go-workflow-auditlog library.
//
// Usage:
//
//	auditlog info <file>                     print a report summary
//	auditlog convert <input> [-o out] [-f FMT]  convert between formats
//	auditlog diff <a> <b>                    diff two reports
//	auditlog validate <file>                 validate report consistency
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// CLIVersion is the auditlog CLI version. Overridable at build time via:
//
//	go build -ldflags "-X main.CLIVersion=v0.1.0" ./cmd/auditlog
//
//nolint:gochecknoglobals // build-time overridable via ldflags
var CLIVersion = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error

	switch cmd {
	case "info":
		err = runInfo(args)
	case "convert":
		err = runConvert(args)
	case "diff":
		err = runDiff(args)
	case "validate":
		err = runValidate(args)
	case "version", "-v", "--version":
		fmt.Printf("auditlog %s (schema %s)\n", CLIVersion, auditlog.SchemaVersion)

		return
	case "-h", "--help", "help":
		usage(os.Stdout)

		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		usage(os.Stderr)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "auditlog %s: %v\n", cmd, err)
		os.Exit(1)
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `auditlog — inspect and convert workflow auditlog reports

Usage:
  auditlog info <file>
      Print a human-readable summary of the report.

  auditlog convert <input> [-o output] [-f format]
      Convert a report between formats. Format: json, ndjson, csv.
      Input can be JSON (report) or NDJSON (events, auto-replayed).
      When -f is omitted it is inferred from the -o file extension;
      when -o is omitted output goes to stdout.

  auditlog diff <a> <b>
      Print the structural differences between two reports.

  auditlog validate <file>
      Load and validate a report (consistency + denormalized counts).

  auditlog version
      Print the CLI and schema versions.
`)
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	return fs
}
