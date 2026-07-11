package auditlog

import (
	"bufio"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
)

// writeEventsNDJSON writes events as newline-delimited JSON (one JSON object per line).
func writeEventsNDJSON(writer io.Writer, events []Event) error {
	buf := bufio.NewWriter(writer)
	encoder := jsontext.NewEncoder(buf)

	for _, evt := range events {
		err := json.MarshalEncode(encoder, evt,
			jsontext.EscapeForHTML(true),
			jsontext.EscapeForJS(true),
		)
		if err != nil {
			return fmt.Errorf("%w: encode event %d: %w", ErrRenderFailed, evt.Sequence, err)
		}
	}

	err := buf.Flush()
	if err != nil {
		return fmt.Errorf("%w: flush ndjson buffer: %w", ErrExportWriteFailed, err)
	}

	return nil
}
