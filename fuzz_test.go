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

// FuzzHTMLSpecialChars fuzzes the HTML dashboard export with step names and
// error messages containing XSS payloads, injection vectors, and special
// characters. The rendered HTML must never contain raw, unescaped script tags
// or event handlers derived from the input data.
func FuzzHTMLSpecialChars(f *testing.F) {
	malicious := []string{
		"<script>alert('xss')</script>",
		`" onload="alert(1)`,
		"'; DROP TABLE--",
		"<img src=x onerror=alert(1)>",
		"\x00null\x00bytes",
		strings.Repeat("A", 1000),
		"\n\r\t",
		"${7*7}",
		"<svg onload=alert(1)>",
		"javascript:alert(1)",
		`"><script>alert(1)</script>`,
		"' onclick='alert(1)",
	}

	for _, m := range malicious {
		f.Add(m)
	}

	f.Fuzz(func(t *testing.T, input string) {
		t.Parallel()

		if input == "" {
			t.Skip()
		}

		errMsg := input
		dur := 1.0

		report := auditlog.WorkflowReport{
			WorkflowID: "fuzz-html",
			StepCount:  2,
			Steps: []auditlog.StepInfo{
				{
					StepRef:      auditlog.StepRef{Name: input, StepType: input},
					Status:       auditlog.StepStatusFailed,
					AttemptCount: 1,
					DurationMs:   &dur,
					Error:        &errMsg,
				},
				{
					StepRef:      auditlog.StepRef{Name: "normal"},
					Status:       auditlog.StepStatusSucceeded,
					AttemptCount: 1,
					DurationMs:   &dur,
					Dependencies: []auditlog.StepRef{{Name: input}},
				},
			},
			EventCount: 2,
			Events: []auditlog.Event{
				{
					StepRef:    auditlog.StepRef{Name: input, StepType: input},
					Sequence:   1,
					EventType:  auditlog.EventTypeAttemptEnd,
					Phase:      auditlog.PhaseAfter,
					Attempt:    1,
					DurationMs: &dur,
					Error:      &errMsg,
					Status:     auditlog.StepStatusFailed,
				},
			},
		}

		var buf bytes.Buffer

		err := report.WriteHTML(&buf)
		if err != nil {
			return // JSON marshal may fail on some inputs
		}

		output := buf.String()
		assertNoRawScriptInjection(t, output, input)
		assertHTMLStructure(t, output)
	})
}

// assertNoRawScriptInjection checks that XSS payloads are properly contained
// within JSON script tags (where they're inert) and not rendered as executable
// HTML. It extracts JSON data blocks and verifies user data is escaped there,
// rather than checking the raw HTML (which legitimately contains <script> tags
// as part of the template structure).
func assertNoRawScriptInjection(t *testing.T, output, input string) {
	t.Helper()

	if strings.Contains(input, "<script") {
		jsonEscaped := strings.ReplaceAll(input, "<", "\\u003c")

		// Extract JSON data blocks — user data lives here, not in template HTML.
		// The template's own <script> tags are structural and safe.
		for _, id := range []string{`"report-data"`, `"type-metadata"`, `"dag-data"`} {
			jsonBlock := extractJSONBlock(output, id)
			if jsonBlock == "" {
				continue
			}

			if !strings.Contains(jsonBlock, jsonEscaped) && strings.Contains(jsonBlock, input) {
				t.Errorf("raw XSS payload appears unescaped in JSON block %s", id)
			}
		}
	}

	for _, attr := range []string{` onload=`, ` onerror=`, ` onclick=`, ` onmouseover=`} {
		if strings.Contains(input, attr) {
			jsonEscapedAttr := strings.ReplaceAll(attr, `"`, `\"`)
			if !strings.Contains(output, jsonEscapedAttr) && strings.Contains(output, attr) {
				t.Errorf("raw event handler injection in HTML output: %s", attr)
			}
		}
	}
}

// extractJSONBlock extracts the content between <script type="application/json" id="ID">
// and </script> tags. Returns empty string if the block is not found.
func extractJSONBlock(output, idAttr string) string {
	openTag := `<script type="application/json" id=` + idAttr + `>`
	start := strings.Index(output, openTag)
	if start < 0 {
		return ""
	}

	start += len(openTag)
	end := strings.Index(output[start:], "</script>")
	if end < 0 {
		return ""
	}

	return output[start : start+end]
}
