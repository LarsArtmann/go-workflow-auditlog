package auditlog

import (
	"bufio"
	"bytes"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
)

// Sentinel errors for NDJSON reading.
var (
	ErrEmpty         = errors.New("ndjson input is empty")
	ErrNoEvents      = errors.New("ndjson input contains no events")
	ErrOversizedLine = errors.New("ndjson line exceeds maximum size")

	errUnknownEventType = errors.New("unknown event_type")
	errUnknownPhase     = errors.New("unknown phase")
)

// Maximum line size for NDJSON reading (1 MB).
const ndjsonMaxLineBytes = 1 << 20

// ReadEvents reads line-delimited JSON events from reader.
// Each line must be a single JSON-encoded Event object.
// Blank lines are skipped. Returns the parsed events in order.
//
// Returns ErrEmpty if the input contains no bytes, ErrNoEvents if all lines
// were blank, or ErrOversizedLine if any line exceeds 1 MB.
func ReadEvents(reader io.Reader) ([]Event, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, ndjsonMaxLineBytes), ndjsonMaxLineBytes)

	var events []Event

	sawData := false
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		// Skip blank/whitespace-only lines.
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		sawData = true

		var evt Event

		err := json.Unmarshal(line, &evt)
		if err != nil {
			return nil, fmt.Errorf("ndjson line %d: %w", lineNum, err)
		}

		if evt.EventType != "" && !evt.EventType.IsKnown() {
			return nil, fmt.Errorf("line %d: %w: %q", lineNum, errUnknownEventType, evt.EventType)
		}

		if evt.Phase != "" && !evt.Phase.IsKnown() {
			return nil, fmt.Errorf("line %d: %w: %q", lineNum, errUnknownPhase, evt.Phase)
		}

		events = append(events, evt)
	}

	err := scanner.Err()
	if err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, fmt.Errorf("%w (max %d bytes)", ErrOversizedLine, ndjsonMaxLineBytes)
		}

		return nil, fmt.Errorf("scan ndjson: %w", err)
	}

	if len(events) == 0 {
		if !sawData {
			return nil, ErrEmpty
		}

		return nil, ErrNoEvents
	}

	return events, nil
}
