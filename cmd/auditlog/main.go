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
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	errorfamily "github.com/larsartmann/go-error-family"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// CLIVersion is the auditlog CLI version. Overridable at build time via:
//
//	go build -ldflags "-X main.CLIVersion=v0.1.0" ./cmd/auditlog
var CLIVersion = "0.1.0"

//nolint:gochecknoinits // Application-level stdlib classification: missing files and canceled contexts must map to the right exit-code family.
func init() {
	errorfamily.RegisterStdlibDefaults(errorfamily.DefaultRegistry)
}

// usageError builds a Rejection-classified CLI usage error so bad
// invocations exit with the usage status (1) instead of the Transient
// fail-open default (75) that unclassified errors would receive.
func usageError(format string, args ...any) error {
	return errorfamily.NewRejection("auditlog.cli.usage", fmt.Sprintf(format, args...))
}

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
	case "schema":
		err = runSchema(args)
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

			if errors.Is(err, flag.ErrHelp) {
				usage(os.Stdout)

				os.Exit(0)
			}

			// Errors from the auditlog library carry their family intrinsically;
			// stdlib errors are classified via RegisterStdlibDefaults above. The
			// exit code reflects the failure family (Rejection 1, Corruption 65,
			// Infrastructure 69, Transient 75).
			os.Exit(errorfamily.ExitCode(err))
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
