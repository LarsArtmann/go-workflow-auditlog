package auditlog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// writeEventsNDJSON writes events as newline-delimited JSON (one JSON object per line).
func writeEventsNDJSON(writer io.Writer, events []Event) error {
	buf := bufio.NewWriter(writer)
	encoder := json.NewEncoder(buf)

	for _, evt := range events {
		err := encoder.Encode(evt)
		if err != nil {
			return fmt.Errorf("encode event %d: %w", evt.Sequence, err)
		}
	}

	err := buf.Flush()
	if err != nil {
		return fmt.Errorf("flush ndjson buffer: %w", err)
	}

	return nil
}
