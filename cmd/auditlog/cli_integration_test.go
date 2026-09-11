package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildCLI builds the auditlog binary and returns its path.
func buildCLI(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "auditlog")

	cmd := exec.Command("go", "build", "-o", binary, ".")

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, stderr.String())
	}

	return binary
}

// writeTestReport writes a minimal valid JSON report to a temp file.
func writeTestReport(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")

	report := `{
		"version": "0.1.0",
		"workflow_id": "test-workflow",
		"exported_at": "2026-01-01T00:00:00Z",
		"event_count": 4,
		"step_count": 2,
		"succeeded_count": 1,
		"failed_count": 1,
		"total_duration_ms": 100.0,
		"wall_clock_duration_ms": 100.0,
		"workflow_succeeded": false,
		"events": [
			{"step_name":"fetch","sequence":1,"timestamp":"2026-01-01T00:00:00Z","event_type":"attempt_start","phase":"before","attempt":1},
			{"step_name":"fetch","sequence":2,"timestamp":"2026-01-01T00:00:00.05Z","event_type":"attempt_end","phase":"after","attempt":1,"duration_ms":50,"status":"succeeded"},
			{"step_name":"save","sequence":3,"timestamp":"2026-01-01T00:00:00.05Z","event_type":"attempt_start","phase":"before","attempt":1},
			{"step_name":"save","sequence":4,"timestamp":"2026-01-01T00:00:00.1Z","event_type":"attempt_end","phase":"after","attempt":1,"duration_ms":50,"status":"failed","error":"disk full"}
		],
		"steps": [
			{"step_name":"fetch","status":"succeeded","attempt_count":1,"started_at":"2026-01-01T00:00:00Z","finished_at":"2026-01-01T00:00:00.05Z","duration_ms":50,"has_retry":false,"has_timeout":false},
			{"step_name":"save","status":"failed","attempt_count":1,"started_at":"2026-01-01T00:00:00.05Z","finished_at":"2026-01-01T00:00:00.1Z","duration_ms":50,"error":"disk full","has_retry":false,"has_timeout":false}
		]
	}`

	if err := os.WriteFile(path, []byte(report), 0o600); err != nil {
		t.Fatalf("write test report: %v", err)
	}

	return path
}

func TestCLI_Info(t *testing.T) {
	t.Parallel()

	binary := buildCLI(t)
	reportPath := writeTestReport(t)

	cmd := exec.Command(binary, "info", reportPath)

	var stdout bytes.Buffer

	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("info: %v", err)
	}

	output := stdout.String()

	if !strings.Contains(output, "test-workflow") {
		t.Errorf("expected workflow name in output, got: %s", output)
	}

	if !strings.Contains(output, "steps:") {
		t.Errorf("expected step count in output")
	}
}

func TestCLI_Validate(t *testing.T) {
	t.Parallel()

	binary := buildCLI(t)
	reportPath := writeTestReport(t)

	cmd := exec.Command(binary, "validate", reportPath)

	var stdout bytes.Buffer

	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	output := stdout.String()

	if !strings.Contains(output, "valid") {
		t.Errorf("expected 'valid' in output, got: %s", output)
	}
}

func TestCLI_Diff_NoChanges(t *testing.T) {
	t.Parallel()

	binary := buildCLI(t)
	reportPath := writeTestReport(t)

	cmd := exec.Command(binary, "diff", reportPath, reportPath)

	var stdout bytes.Buffer

	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("diff: %v", err)
	}

	output := stdout.String()

	if !strings.Contains(output, "no differences") {
		t.Errorf("expected 'no differences' in output, got: %s", output)
	}
}

func TestCLI_Convert_JSON(t *testing.T) {
	t.Parallel()

	binary := buildCLI(t)
	reportPath := writeTestReport(t)

	outPath := filepath.Join(t.TempDir(), "output.json")

	cmd := exec.Command(binary, "convert", "-o", outPath, "-f", "json", reportPath)

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("convert: %v\n%s", err, stderr.String())
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty JSON output")
	}

	if !strings.Contains(string(data), "test-workflow") {
		t.Error("expected workflow_id in converted output")
	}
}

func TestCLI_Version(t *testing.T) {
	t.Parallel()

	binary := buildCLI(t)

	cmd := exec.Command(binary, "version")

	var stdout bytes.Buffer

	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("version: %v", err)
	}

	output := stdout.String()

	if !strings.Contains(output, "auditlog") {
		t.Errorf("expected 'auditlog' in version output, got: %s", output)
	}

	if !strings.Contains(output, "0.1.0") {
		t.Errorf("expected schema version in output, got: %s", output)
	}
}

// TestCLI_ExitCodes pins the family-based exit-code contract: usage errors
// and bad input exit 1 (Rejection), retryable load failures exit 75
// (Transient), help exits 0.
func TestCLI_ExitCodes(t *testing.T) {
	t.Parallel()

	binary := buildCLI(t)

	tests := []struct {
		name string
		args []string
		want int
	}{
		{
			name: "wrong argument count is Rejection (1)",
			args: []string{"convert"},
			want: 1,
		},
		{
			name: "missing input file is Rejection (1)",
			args: []string{"info", filepath.Join(t.TempDir(), "does-not-exist.json")},
			want: 1,
		},
		{
			name: "help exits 0",
			args: []string{"convert", "-h"},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := exec.Command(binary, tt.args...).Run()

			if tt.want == 0 {
				if err != nil {
					t.Fatalf("expected exit 0, got error: %v", err)
				}

				return
			}

			exitErr, ok := errors.AsType[*exec.ExitError](err)
			if !ok {
				t.Fatalf("expected non-zero exit, got nil error")

				return
			}

			if got := exitErr.ExitCode(); got != tt.want {
				t.Errorf("exit code = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestCLI_ExitCodeTransientLoad pins the Transient (75) path: a report that
// exists but cannot be decoded is classified as retryable.
func TestCLI_ExitCodeTransientLoad(t *testing.T) {
	t.Parallel()

	binary := buildCLI(t)

	path := filepath.Join(t.TempDir(), "broken.json")

	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write broken report: %v", err)
	}

	err := exec.Command(binary, "validate", path).Run()

	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		t.Fatalf("expected non-zero exit, got nil error")
	}

	if got := exitErr.ExitCode(); got != 75 {
		t.Errorf("exit code = %d, want 75 (Transient: decode failures may succeed on retry)", got)
	}
}
