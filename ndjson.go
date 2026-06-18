package auditlog

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Sentinel errors for NDJSON reading.
var (
	ErrEmpty         = errors.New("ndjson input is empty")
	ErrNoEvents      = errors.New("ndjson input contains no events")
	ErrOversizedLine = errors.New("ndjson line exceeds maximum size")
)

const ndjsonMaxLineBytes = 1 << 20 // 1 MB

// ReadEvents reads NDJSON-encoded events from reader. Each line must be a
// valid JSON Event object. Blank lines are skipped.
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

		if len(line) == 0 {
			continue
		}

		sawData = true

		var evt Event

		err := json.Unmarshal(line, &evt)
		if err != nil {
			return nil, fmt.Errorf("ndjson line %d: %w", lineNum, err)
		}

		events = append(events, evt)
	}

	err := scanner.Err()
	if err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, fmt.Errorf("%w: max %d bytes", ErrOversizedLine, ndjsonMaxLineBytes)
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
