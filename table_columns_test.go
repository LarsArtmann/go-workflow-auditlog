package auditlog_test

import (
	"strings"
	"testing"

	"github.com/larsartmann/go-output"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func TestTable_DefaultColumns(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "default-col-step")

	out, err := a.Report().WriteTableString(output.FormatCSV, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	// Default columns: Step, Status, Duration, Attempts, Retry, Timeout, Error.
	assertContains(t, out, "Step", "expected Step header")
	assertContains(t, out, "Status", "expected Status header")
	assertContains(t, out, "Duration", "expected Duration header")
	assertContains(t, out, "Attempts", "expected Attempts header")
	assertContains(t, out, "Retry", "expected Retry header")
	assertContains(t, out, "Timeout", "expected Timeout header")
	assertContains(t, out, "Error", "expected Error header")
}

func TestTable_CustomColumnSelection(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "custom-col-step")

	out, err := a.Report().WriteTableString(
		output.FormatCSV, output.RenderOptions{},
		auditlog.WithColumns(auditlog.ColumnStep, auditlog.ColumnStatus),
	)
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	// Only selected columns should appear.
	assertContains(t, out, "Step", "expected Step header")
	assertContains(t, out, "Status", "expected Status header")

	// Columns NOT selected should be absent.
	if strings.Contains(out, "Duration") {
		t.Error("Duration should not appear when not selected")
	}

	if strings.Contains(out, "Retry") {
		t.Error("Retry should not appear when not selected")
	}

	if strings.Contains(out, "Error") {
		t.Error("Error should not appear when not selected")
	}
}

func TestTable_NewColumns(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	fetch := newSucceed("fetch")
	transform := newSucceed("transform")
	save := newSucceed("save")
	addLinearChain(w, fetch, transform, save)
	runWorkflow(t, a, w)

	out, err := a.Report().WriteTableString(
		output.FormatCSV, output.RenderOptions{},
		auditlog.WithColumns(auditlog.ColumnStep, auditlog.ColumnType, auditlog.ColumnDependencies),
	)
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	assertContains(t, out, "Step", "expected Step header")
	assertContains(t, out, "Type", "expected Type header")
	assertContains(t, out, "Dependencies", "expected Dependencies header")

	// transform depends on fetch — the dependency name should appear in the table.
	assertContains(t, out, "fetch", "expected dependency name in table")
}

func TestTable_ColumnOrderRespected(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "order-step")

	// Reverse the natural order: Status, then Step.
	out, err := a.Report().WriteTableString(
		output.FormatCSV, output.RenderOptions{},
		auditlog.WithColumns(auditlog.ColumnStatus, auditlog.ColumnStep),
	)
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 1 {
		t.Fatal("expected at least a header row")
	}

	header := lines[0]

	// Status must come before Step in the header.
	statusIdx := strings.Index(header, "Status")
	stepIdx := strings.Index(header, "Step")

	if statusIdx < 0 || stepIdx < 0 {
		t.Fatalf("expected both Status and Step in header: %q", header)
	}

	if statusIdx > stepIdx {
		t.Errorf("Status should appear before Step: header=%q", header)
	}
}

func TestTable_AllColumns(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "all-cols-step")

	out, err := a.Report().WriteTableString(
		output.FormatCSV, output.RenderOptions{},
		auditlog.WithColumns(auditlog.AllTableColumns()...),
	)
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	for _, header := range []string{"Step", "Status", "Duration", "Attempts", "Max Attempts", "Retry", "Timeout", "Error", "Type", "Dependencies"} {
		if !strings.Contains(out, header) {
			t.Errorf("expected header %q in output with all columns", header)
		}
	}
}

func TestTable_MaxAttemptsColumn(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	step := newFlaky("flaky-max", 2)
	addRetryStep(w, step, 5)
	runWorkflow(t, a, w)

	out, err := a.Report().WriteTableString(
		output.FormatCSV, output.RenderOptions{},
		auditlog.WithColumns(auditlog.ColumnStep, auditlog.ColumnMaxAttempts),
	)
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	assertContains(t, out, "Max Attempts", "expected Max Attempts header")
}

func TestTable_ColumnsOnAuditor(t *testing.T) {
	t.Parallel()

	a, buf := runSingleSucceedWithBuffer(t, "auditor-col-step")

	err := a.WriteTable(buf, output.FormatCSV, output.RenderOptions{},
		auditlog.WithColumns(auditlog.ColumnStep, auditlog.ColumnStatus),
	)
	if err != nil {
		t.Fatalf("Auditor.WriteTable error: %v", err)
	}

	out := buf.String()
	assertContains(t, out, "Step", "expected Step header from Auditor")
	assertContains(t, out, "Status", "expected Status header from Auditor")
}

func TestTable_ExportWithColumns(t *testing.T) {
	t.Parallel()

	a, path := singleSucceedExportPath(t, "table-cols-export", "table.csv")

	err := a.ExportTable(path, output.FormatCSV, output.RenderOptions{},
		auditlog.WithColumns(auditlog.ColumnStep, auditlog.ColumnStatus),
	)
	if err != nil {
		t.Fatalf("ExportTable error: %v", err)
	}
}

func TestTable_EmptyColumnSelectionUsesDefaults(t *testing.T) {
	t.Parallel()

	a := runSingleSucceed(t, "empty-cols-step")

	// WriteTable with no tableOpts should produce default columns.
	out, err := a.Report().WriteTableString(output.FormatCSV, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	assertContains(t, out, "Step", "expected Step header with default columns")
	assertContains(t, out, "Error", "expected Error header with default columns")
}

// Ensures column selection also works from a replayed report (not just live capture).
func TestTable_ColumnsFromReplayedReport(t *testing.T) {
	t.Parallel()

	a, w := newAuditAndWorkflow(t)
	fetch := newSucceed("fetch-replay")
	transform := newSucceed("transform-replay")
	addDependentStep(w, fetch, transform)
	runWorkflow(t, a, w)

	report := a.Report()

	var ndjsonBuf strings.Builder

	_ = report.WriteNDJSON(&ndjsonBuf)

	events, err := auditlog.ReadEvents(strings.NewReader(ndjsonBuf.String()))
	if err != nil {
		t.Fatalf("ReadEvents error: %v", err)
	}

	replayed, err := auditlog.ReplayEvents(events)
	if err != nil {
		t.Fatalf("ReplayEvents error: %v", err)
	}

	out, err := replayed.WriteTableString(
		output.FormatCSV, output.RenderOptions{},
		auditlog.WithColumns(auditlog.ColumnStep, auditlog.ColumnDependencies),
	)
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	assertContains(t, out, "Step", "expected Step header from replayed report")
	assertContains(t, out, "Dependencies", "expected Dependencies header from replayed report")
}

func TestTable_DefaultTableColumnsVar(t *testing.T) {
	t.Parallel()

	defaults := auditlog.DefaultTableColumns

	if len(defaults) != 7 {
		t.Errorf("expected 7 default columns, got %d", len(defaults))
	}

	// Ensure it matches the original hardcoded layout.
	expected := []auditlog.TableColumn{
		auditlog.ColumnStep,
		auditlog.ColumnStatus,
		auditlog.ColumnDuration,
		auditlog.ColumnAttempts,
		auditlog.ColumnRetry,
		auditlog.ColumnTimeout,
		auditlog.ColumnError,
	}

	for i, want := range expected {
		if defaults[i] != want {
			t.Errorf("DefaultTableColumns[%d] = %v, want %v", i, defaults[i], want)
		}
	}
}

func TestTable_AllTableColumnsCount(t *testing.T) {
	t.Parallel()

	all := auditlog.AllTableColumns()
	if len(all) != 10 {
		t.Errorf("expected 10 total columns, got %d", len(all))
	}
}

func TestTable_ZeroDurationCell(t *testing.T) {
	t.Parallel()

	// A step with DurationMs=0 should produce an empty duration cell.
	zeroDur := 0.0
	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{
				StepRef:    auditlog.StepRef{Name: "instant"},
				Status:     auditlog.StepStatusSucceeded,
				DurationMs: &zeroDur,
			},
		},
	}

	out, err := report.WriteTableString(
		output.FormatCSV, output.RenderOptions{},
		auditlog.WithColumns(auditlog.ColumnStep, auditlog.ColumnDuration),
	)
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	// The data row should be "instant," (step name, empty duration).
	assertContains(t, out, "instant,", "expected step name with empty duration cell")
}

func TestTable_ColumnString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		col  auditlog.TableColumn
		want string
	}{
		{auditlog.ColumnStep, "Step"},
		{auditlog.ColumnStatus, "Status"},
		{auditlog.ColumnDuration, "Duration"},
		{auditlog.ColumnAttempts, "Attempts"},
		{auditlog.ColumnMaxAttempts, "Max Attempts"},
		{auditlog.ColumnRetry, "Retry"},
		{auditlog.ColumnTimeout, "Timeout"},
		{auditlog.ColumnError, "Error"},
		{auditlog.ColumnType, "Type"},
		{auditlog.ColumnDependencies, "Dependencies"},
	}

	for _, tt := range tests {
		if got := tt.col.String(); got != tt.want {
			t.Errorf("%d.String() = %q, want %q", tt.col, got, tt.want)
		}
	}
}

func TestTable_DefaultColumnsImmutability(t *testing.T) {
	t.Parallel()

	original := append([]auditlog.TableColumn(nil), auditlog.DefaultTableColumns...)

	// Mutate a returned-default copy via WriteTable with no column options.
	// Internally, applyTableOpts must copy DefaultTableColumns, not alias it.
	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "s"}, Status: auditlog.StepStatusSucceeded},
		},
	}

	_, err := report.WriteTableString(output.FormatCSV, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTableString: %v", err)
	}

	// Verify DefaultTableColumns was not mutated by the internal copy logic.
	for i, want := range original {
		if auditlog.DefaultTableColumns[i] != want {
			t.Errorf("DefaultTableColumns[%d] was mutated: got %v, want %v",
				i, auditlog.DefaultTableColumns[i], want)
		}
	}
}
