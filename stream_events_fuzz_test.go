package auditlog_test

import (
	"strings"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// FuzzStreamEvents verifies that StreamEvents never panics on arbitrary input
// and that every invocation either delivers events to the callback or returns
// an error — never both silently dropping data.
func FuzzStreamEvents(f *testing.F) {
	seeds := []string{
		"",
		"\n\n\n",
		`{"sequence":1,"timestamp":"2026-01-01T00:00:00Z","event_type":"attempt_start","phase":"before","step_name":"s"}`,
		"not json",
		`{}`,
		`{"sequence":1}` + "\n" + `broken` + "\n",
		"\x00\x01\x02",
		strings.Repeat("a", 10000),
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		var count int

		err := auditlog.StreamEvents(
			strings.NewReader(input),
			nil,
			func(int, auditlog.Event) error {
				count++

				return nil
			},
		)

		// No panic is the primary invariant. If no error was returned,
		// at least one event must have been delivered.
		if err == nil && count == 0 && strings.TrimSpace(input) != "" {
			t.Errorf("no error and no events delivered for non-empty input")
		}
	})
}
