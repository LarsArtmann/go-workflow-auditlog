package auditlog

import (
	"bufio"
	"encoding/json"
	"io"
)

// writeEventsNDJSON writes events as newline-delimited JSON (one JSON object per line).
func writeEventsNDJSON(writer io.Writer, events []Event) error {
	buf := bufio.NewWriter(writer)
	encoder := json.NewEncoder(buf)

	for _, evt := range events {
		err := encoder.Encode(evt)
		if err != nil {
			return err
		}
	}

	return buf.Flush()
}
