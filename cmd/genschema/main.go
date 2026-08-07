// Package main is the JSON Schema generator for the workflow auditlog report format.
//
// It reflects over the library's public types (WorkflowReport, Event, StepInfo, etc.)
// and emits schema/report.schema.json so report consumers can validate exported
// JSON without the schema drifting from the Go types.
//
// Usage: go run ./cmd/genschema
package main

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"

	"github.com/invopop/jsonschema"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func main() {
	r := new(jsonschema.Reflector)
	r.DoNotReference = false
	r.ExpandedStruct = true

	schema := r.Reflect(auditlog.WorkflowReport{})
	schema.ID = "https://github.com/larsartmann/go-workflow-auditlog/schema/report.schema.json"
	schema.Title = "workflow-auditlog Report"
	schema.Description = "Report exported by the go-workflow-auditlog auditor."

	data, err := json.Marshal(schema, jsontext.WithIndentPrefix(""), jsontext.WithIndent("  "))
	if err != nil {
		dief("marshal schema: %v\n", err)
	}

	data = append(data, '\n')

	outPath := filepath.Join("schema", "report.schema.json")
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		dief("mkdir: %v\n", err)
	}

	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		dief("write schema: %v\n", err)
	}

	fmt.Printf("wrote %s (%d bytes)\n", outPath, len(data))
}

func dief(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}
