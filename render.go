package auditlog

import (
	"fmt"
	"io"
)

// writeRendered invokes render, then writes the resulting string to writer with
// a trailing newline. Both the render error and the write error are wrapped
// with the auditlog sentinels (ErrRenderFailed and ErrExportWriteFailed
// respectively). The format label is interpolated into the error message so
// callers can produce format-specific error text without leaking the
// underlying renderer name into user-facing messages.
func writeRendered(writer io.Writer, format string, render func() (string, error)) error {
	out, err := render()
	if err != nil {
		return fmt.Errorf("%w: render %s: %w", ErrRenderFailed, format, err)
	}

	_, err = fmt.Fprintln(writer, out)
	if err != nil {
		return fmt.Errorf("%w: write %s output: %w", ErrExportWriteFailed, format, err)
	}

	return nil
}
