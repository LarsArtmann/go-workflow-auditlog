package viz_test

import (
	"strings"
	"testing"

	"github.com/larsartmann/go-output"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	viz "github.com/larsartmann/go-workflow-auditlog/viz"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

func TestTable_DefaultColumns(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "default-col-step")

	out, err := viz.WriteTableString(viz, a.Report(), output.FormatCSV, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	// Default columns: Step, Status, Duration, Attempts, Retry, Timeout, Error.
	testhelpers.AssertContains(t, out, "Step", "expected Step header")
	testhelpers.AssertContains(t, out, "Status", "expected Status header")
	testhelpers.AssertContains(t, out, "Duration", "expected Duration header")
	testhelpers.AssertContains(t, out, "Attempts", "expected Attempts header")
	testhelpers.AssertContains(t, out, "Retry", "expected Retry header")
	testhelpers.AssertContains(t, out, "Timeout", "expected Timeout header")
	testhelpers.AssertContains(t, out, "Error", "expected Error header")
}

func TestTable_CustomColumnSelection(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "custom-col-step")

	out, err := viz.WriteTableString(viz, a.Report(), 
		output.FormatCSV, output.RenderOptions{},
		viz.WithColumns(viz.ColumnStep, viz.ColumnStatus),
	)
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	// Only selected columns should appear.
	testhelpers.AssertContains(t, out, "Step", "expected Step header")
	testhelpers.AssertContains(t, out, "Status", "expected Status header")

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

	a, w := testhelpers.NewAuditAndWorkflow(t)
	fetch := testhelpers.NewSucceed("fetch")
	transform := testhelpers.NewSucceed("transform")
	save := testhelpers.NewSucceed("save")
	testhelpers.AddLinearChain(w, fetch, transform, save)
	testhelpers.RunWorkflow(t, a, w)

	out, err := viz.WriteTableString(viz, a.Report(), 
		output.FormatCSV, output.RenderOptions{},
		viz.WithColumns(viz.ColumnStep, viz.ColumnType, viz.ColumnDependencies),
	)
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	testhelpers.AssertContains(t, out, "Step", "expected Step header")
	testhelpers.AssertContains(t, out, "Type", "expected Type header")
	testhelpers.AssertContains(t, out, "Dependencies", "expected Dependencies header")

	// transform depends on fetch — the dependency name should appear in the table.
	testhelpers.AssertContains(t, out, "fetch", "expected dependency name in table")
}

func TestTable_ColumnOrderRespected(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "order-step")

	// Reverse the natural order: Status, then Step.
	out, err := viz.WriteTableString(viz, a.Report(), 
		output.FormatCSV, output.RenderOptions{},
		viz.WithColumns(viz.ColumnStatus, viz.ColumnStep),
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

	a := testhelpers.RunSingleSucceed(t, "all-cols-step")

	out, err := viz.WriteTableString(viz, a.Report(), 
		output.FormatCSV, output.RenderOptions{},
		viz.WithColumns(auditlog.AllTableColumns()...),
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

	a, w := testhelpers.NewAuditAndWorkflow(t)
	step := testhelpers.NewFlaky("flaky-max", 2)
	testhelpers.AddRetryStep(w, step, 5)
	testhelpers.RunWorkflow(t, a, w)

	out, err := viz.WriteTableString(viz, a.Report(), 
		output.FormatCSV, output.RenderOptions{},
		viz.WithColumns(viz.ColumnStep, viz.ColumnMaxAttempts),
	)
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	testhelpers.AssertContains(t, out, "Max Attempts", "expected Max Attempts header")
}

func TestTable_ColumnsOnAuditor(t *testing.T) {
	t.Parallel()

	a, buf := testhelpers.RunSingleSucceedWithBuffer(t, "auditor-col-step")

	err := viz.WriteTable(a, buf, output.FormatCSV, output.RenderOptions{},
		viz.WithColumns(viz.ColumnStep, viz.ColumnStatus),
	)
	if err != nil {
		t.Fatalf("Auditor.WriteTable error: %v", err)
	}

	out := buf.String()
	testhelpers.AssertContains(t, out, "Step", "expected Step header from Auditor")
	testhelpers.AssertContains(t, out, "Status", "expected Status header from Auditor")
}

func TestTable_ExportWithColumns(t *testing.T) {
	t.Parallel()

	a, path := testhelpers.SingleSucceedExportPath(t, "table-cols-export", "table.csv")

	err := viz.ExportTable(a, path, output.FormatCSV, output.RenderOptions{},
		viz.WithColumns(viz.ColumnStep, viz.ColumnStatus),
	)
	if err != nil {
		t.Fatalf("ExportTable error: %v", err)
	}
}

func TestTable_EmptyColumnSelectionUsesDefaults(t *testing.T) {
	t.Parallel()

	a := testhelpers.RunSingleSucceed(t, "empty-cols-step")

	// WriteTable with no tableOpts should produce default columns.
	out, err := viz.WriteTableString(viz, a.Report(), output.FormatCSV, output.RenderOptions{})
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	testhelpers.AssertContains(t, out, "Step", "expected Step header with default columns")
	testhelpers.AssertContains(t, out, "Error", "expected Error header with default columns")
}

// Ensures column selection also works from a replayed report (not just live capture).
func TestTable_ColumnsFromReplayedReport(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	fetch := testhelpers.NewSucceed("fetch-replay")
	transform := testhelpers.NewSucceed("transform-replay")
	testhelpers.AddDependentStep(w, fetch, transform)
	testhelpers.RunWorkflow(t, a, w)

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

	out, err := viz.WriteTableString(replayed, 
		output.FormatCSV, output.RenderOptions{},
		viz.WithColumns(viz.ColumnStep, viz.ColumnDependencies),
	)
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	testhelpers.AssertContains(t, out, "Step", "expected Step header from replayed report")
	testhelpers.AssertContains(t, out, "Dependencies", "expected Dependencies header from replayed report")
}

func TestTable_DefaultTableColumnsVar(t *testing.T) {
	t.Parallel()

	defaults := auditlog.DefaultTableColumns

	if len(defaults) != 7 {
		t.Errorf("expected 7 default columns, got %d", len(defaults))
	}

	// Ensure it matches the original hardcoded layout.
	expected := []auditlog.TableColumn{
		viz.ColumnStep,
		viz.ColumnStatus,
		viz.ColumnDuration,
		viz.ColumnAttempts,
		viz.ColumnRetry,
		viz.ColumnTimeout,
		viz.ColumnError,
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

	out, err := viz.WriteTableString(report, 
		output.FormatCSV, output.RenderOptions{},
		viz.WithColumns(viz.ColumnStep, viz.ColumnDuration),
	)
	if err != nil {
		t.Fatalf("WriteTableString error: %v", err)
	}

	// The data row should be "instant," (step name, empty duration).
	testhelpers.AssertContains(t, out, "instant,", "expected step name with empty duration cell")
}

func TestTable_ColumnString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		col  auditlog.TableColumn
		want string
	}{
		{viz.ColumnStep, "Step"},
		{viz.ColumnStatus, "Status"},
		{viz.ColumnDuration, "Duration"},
		{viz.ColumnAttempts, "Attempts"},
		{viz.ColumnMaxAttempts, "Max Attempts"},
		{viz.ColumnRetry, "Retry"},
		{viz.ColumnTimeout, "Timeout"},
		{viz.ColumnError, "Error"},
		{viz.ColumnType, "Type"},
		{viz.ColumnDependencies, "Dependencies"},
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

	_, err := viz.WriteTableString(report, output.FormatCSV, output.RenderOptions{})
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
