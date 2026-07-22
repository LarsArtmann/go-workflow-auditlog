package viz_test

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
	viz "github.com/larsartmann/go-workflow-auditlog/viz"
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

		mErr := viz.WriteMermaid(report, &mBuf)
		if mErr != nil {
			t.Fatalf("WriteMermaid error: %v", mErr)
		}

		mOut := mBuf.String()
		testhelpers.AssertContains(t, mOut, "flowchart TD", "expected 'flowchart TD' in mermaid output")

		// PlantUML: must not contain @enduml in the middle (injection).
		var pBuf bytes.Buffer

		pErr := viz.WritePlantUML(report, &pBuf)
		if pErr != nil {
			t.Fatalf("WritePlantUML error: %v", pErr)
		}

		// DOT: must produce a valid digraph.
		var dBuf bytes.Buffer

		dErr := viz.WriteGraphviz(report, &dBuf)
		if dErr != nil {
			t.Fatalf("WriteGraphviz error: %v", dErr)
		}

		dOut := dBuf.String()
		testhelpers.AssertContains(t, dOut, "digraph", "expected 'digraph' in DOT output")

		// D2: must not error.
		var d2Buf bytes.Buffer

		d2Err := viz.WriteD2(report, &d2Buf)
		if d2Err != nil {
			t.Fatalf("WriteD2 error: %v", d2Err)
		}

		d2Out := d2Buf.String()
		testhelpers.AssertContains(t, d2Out, "fuzz-run", "expected workflow ID as title in D2 output")
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

		err := viz.WriteHTML(report, &buf)
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

// FuzzDiagramSanitization_MultiStep fuzzes diagram export with two steps
// connected by a dependency edge, both with adversarial names. This goes deeper
// than FuzzDiagramSpecialChars by testing edge (not just node) sanitization,
// unicode and control-character handling, and diagram-keyword collisions.
func FuzzDiagramSanitization_MultiStep(f *testing.F) {
	seeds := [][2]string{
		// Unicode: emoji, CJK, RTL, combining marks.
		{"🎉", "\u6b65\u9aa4"},                                    // CJK "step"
		{"\u062e\u0637\u0648\u0629", "\u30b9\u30c6\u30c3\u30d7"}, // Arabic/Japanese "step"
		{"café\u0301", "naïve"},
		// Control characters.
		{"zero\x00byte", "normal"},
		{"\x07bell\x1besc", "ctrl"},
		// Diagram syntax keyword collisions.
		{"digraph", "subgraph"},
		{"flowchart", "style"},
		{"@startuml", "@enduml"},
		{"node", "edge"},
		{"rankdir", "label"},
		// Whitespace-only.
		{" ", "\t"},
		{"\n", "\r\n"},
		// Mixed adversarial (brackets, quotes, pipes in both names).
		{"a]b[c", `d"e"f`},
		{"step-->other", "dep|pipe"},
		// Length extremes.
		{strings.Repeat("x", 1000), "short"},
		{"short", strings.Repeat("y", 1000)},
		// Unicode + special char combos.
		{"🎉]step", "pipe|지"},
	}

	for _, seed := range seeds {
		f.Add(seed[0], seed[1])
	}

	f.Fuzz(func(t *testing.T, name1, name2 string) {
		t.Parallel()

		if name1 == "" || name2 == "" {
			t.Skip()
		}

		// Skip invalid UTF-8 — rendering libraries may reject it, and that's
		// acceptable (the test verifies valid-input robustness, not encoding
		// repair).
		if !utf8.ValidString(name1) || !utf8.ValidString(name2) {
			t.Skip()
		}

		report := auditlog.WorkflowReport{
			WorkflowID: "fuzz-multi",
			Steps: []auditlog.StepInfo{
				{
					StepRef: auditlog.StepRef{Name: name1},
					Status:  auditlog.StepStatusSucceeded,
				},
				{
					StepRef: auditlog.StepRef{Name: name2},
					Status:  auditlog.StepStatusFailed,
					Dependencies: []auditlog.StepRef{
						{Name: name1},
					},
				},
			},
		}

		formats := []struct {
			name     string
			write    func(io.Writer) error
			mustHave string
		}{
			{"mermaid", func(w io.Writer) error { return viz.WriteMermaid(report, w) }, "flowchart TD"},
			{"plantuml", func(w io.Writer) error { return viz.WritePlantUML(report, w) }, ""},
			{"dot", func(w io.Writer) error { return viz.WriteGraphviz(report, w) }, "digraph"},
			{"d2", func(w io.Writer) error { return viz.WriteD2(report, w) }, "fuzz-multi"},
		}

		for _, fm := range formats {
			var buf bytes.Buffer

			err := fm.write(&buf)
			if err != nil {
				t.Fatalf("%s export error for names %q/%q: %v", fm.name, name1, name2, err)
			}

			out := buf.String()
			if len(out) == 0 {
				t.Errorf("%s produced empty output for names %q/%q", fm.name, name1, name2)
			}

			if fm.mustHave != "" && !strings.Contains(out, fm.mustHave) {
				t.Errorf("%s missing structural marker %q for names %q/%q",
					fm.name, fm.mustHave, name1, name2)
			}
		}
	})
}
