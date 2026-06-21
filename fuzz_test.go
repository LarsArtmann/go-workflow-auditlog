package auditlog_test

import (
	"bytes"
	"strings"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// FuzzDiagramSpecialChars fuzzes diagram export (Mermaid, PlantUML, DOT, D2)
// with step names containing special characters, injection payloads, and edge
// cases. The go-output escape package must sanitize all IDs and labels so the
// rendered output is structurally valid (no broken syntax, no unescaped quotes).
func FuzzDiagramSpecialChars(f *testing.F) {
	special := []string{
		"step]",
		`step"`,
		"step-->",
		"@enduml",
		"%%",
		"step\nother", // newline injection
		"step|pipe",
		"a]b[c",
		`a"b"c`,
		`evil]"step`,   // combined bracket+quote
		`{inj}ect[ion`, //nolint:misspell // adversarial fuzz seed
		"<script>alert(1)</script>",
		strings.Repeat("A", 500),
		"step:colon",  // D2 colon injection
		"step\tother", // D2 tab injection
	}

	for _, s := range special {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, stepName string) {
		t.Parallel()

		if stepName == "" {
			t.Skip()
		}

		report := auditlog.WorkflowReport{
			WorkflowID: "fuzz-run",
			Steps: []auditlog.StepInfo{
				{
					StepRef: auditlog.StepRef{Name: stepName},
					Status:  auditlog.StepStatusSucceeded,
				},
			},
		}

		// Mermaid: structural integrity check.
		var mBuf bytes.Buffer

		mErr := report.WriteMermaid(&mBuf)
		if mErr != nil {
			t.Fatalf("WriteMermaid error: %v", mErr)
		}

		mOut := mBuf.String()
		assertContains(t, mOut, "flowchart TD", "expected 'flowchart TD' in mermaid output")

		// PlantUML: must not contain @enduml in the middle (injection).
		var pBuf bytes.Buffer

		pErr := report.WritePlantUML(&pBuf)
		if pErr != nil {
			t.Fatalf("WritePlantUML error: %v", pErr)
		}

		// DOT: must produce a valid digraph.
		var dBuf bytes.Buffer

		dErr := report.WriteGraphviz(&dBuf)
		if dErr != nil {
			t.Fatalf("WriteGraphviz error: %v", dErr)
		}

		dOut := dBuf.String()
		assertContains(t, dOut, "digraph", "expected 'digraph' in DOT output")

		// D2: must not error.
		var d2Buf bytes.Buffer

		d2Err := report.WriteD2(&d2Buf)
		if d2Err != nil {
			t.Fatalf("WriteD2 error: %v", d2Err)
		}

		d2Out := d2Buf.String()
		assertContains(t, d2Out, "fuzz-run", "expected workflow ID as title in D2 output")
	})
}
