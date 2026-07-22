package auditlog

import (
	"bufio"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
)

// encodeEvent marshals a single Event as JSON to the encoder using the
// canonical escaping options (HTML + JS escaping). Returns a wrapped
// ErrRenderFailed on encoding failure. Shared by writeEventsNDJSON and
// NDJSONStreamer.OnEvent so both paths produce identical NDJSON.
func encodeEvent(encoder *jsontext.Encoder, evt Event) error {
	err := json.MarshalEncode(encoder, evt,
		jsontext.EscapeForHTML(true),
		jsontext.EscapeForJS(true),
	)
	if err != nil {
		return fmt.Errorf("%w: encode event %d: %w", ErrRenderFailed, evt.Sequence, err)
	}

	return nil
}

// writeEventsNDJSON writes events as newline-delimited JSON (one JSON object per line).
func writeEventsNDJSON(writer io.Writer, events []Event) error {
	buf := bufio.NewWriter(writer)
	encoder := jsontext.NewEncoder(buf)

	for _, evt := range events {
		err := encodeEvent(encoder, evt)
		if err != nil {
			return err
		}
	}

	err := buf.Flush()
	if err != nil {
		return fmt.Errorf("%w: flush ndjson buffer: %w", ErrExportWriteFailed, err)
	}

	return nil
}
