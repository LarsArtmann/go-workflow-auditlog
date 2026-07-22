package auditlog

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// ErrFileExists is returned when a file already exists at the target path
// and the caller requested no-clobber behavior. Classified as Rejection —
// the caller asked for an impossible operation (writing to an existing file
// without overwrite). Consumers can match on it with [errors.Is].
var ErrFileExists = fmt.Errorf("%w: file already exists", ErrExportWriteFailed)

// CheckNoClobber returns ErrFileExists if a file already exists at path.
// Call this before Export* methods to prevent accidental overwrites:
//
//	if err := auditlog.CheckNoClobber(path); err != nil { return err }
//	report.ExportJSON(path)
func CheckNoClobber(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return fmt.Errorf("%w: %q", ErrFileExists, path)
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("%w: stat %q: %w", ErrExportWriteFailed, path, err)
	}

	return nil
}

// pointerAddressPattern matches Go's default String() format for pointers:
// "*TypeName(0x...)" — e.g., "*FetchStep(0xc0000a4000)".
var pointerAddressPattern = regexp.MustCompile(`^\*[A-Za-z_][A-Za-z0-9_]*\(0x[0-9a-f]+\)$`)

// HasPointerAddress reports whether name looks like an unoverridden Go
// String() default (e.g., "*TestStep(0xc0000a4000)"). Consumers can use
// this to detect steps that haven't implemented String() and provide a
// human-readable fallback before attaching the auditor:
//
//	if auditlog.HasPointerAddress(flow.String(step)) {
//	    flow.Name("fetch")(flow.Step(step))
//	}
func HasPointerAddress(name string) bool {
	return pointerAddressPattern.MatchString(name)
}

// NameCollisions returns step names that appear more than once in the report.
// When two steps share the same Name (which happens when step types produce
// identical String() output), diagram and table exports silently merge them.
// This method surfaces those collisions so consumers can add disambiguating
// names via flow.Name().
func (r WorkflowReport) NameCollisions() []string {
	counts := make(map[string]int)

	for _, step := range r.Steps {
		counts[step.Name]++
	}

	var collisions []string

	for name, count := range counts {
		if count > 1 {
			collisions = append(collisions, name)
		}
	}

	return collisions
}

// WriteToFile creates a file at path and calls fn with a buffered writer.
// The bufio.Writer batches small writes into 64KB blocks, reducing syscall count
// by 10-100x compared to writing directly to os.File.
//
// Writes are atomic: data is written to a temporary file in the same directory,
// then atomically renamed to the final path. A crash during write leaves the
// previous file (if any) intact rather than a partial file.
func WriteToFile(path string, fn func(io.Writer) error) error {
	dir := filepath.Dir(path)

	tmpFile, err := os.CreateTemp(dir, ".tmp-auditlog-*")
	if err != nil {
		return fmt.Errorf("%w: create temp file in %q: %w", ErrExportWriteFailed, dir, err)
	}

	tmpPath := tmpFile.Name()
	cleanup := true

	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	bw := bufio.NewWriterSize(tmpFile, fileWriteBufferSize)

	writeErr := fn(bw)

	flushErr := bw.Flush()

	closeErr := tmpFile.Close()

	if writeErr != nil {
		return writeErr
	}

	if flushErr != nil {
		return fmt.Errorf("%w: flush temp file %q: %w", ErrExportWriteFailed, tmpPath, flushErr)
	}

	if closeErr != nil {
		return fmt.Errorf("%w: close temp file %q: %w", ErrExportWriteFailed, tmpPath, closeErr)
	}

	renameErr := os.Rename(tmpPath, path)
	if renameErr != nil {
		return fmt.Errorf("%w: rename %q → %q: %w", ErrExportWriteFailed, tmpPath, path, renameErr)
	}

	cleanup = false

	return nil
}

// fileWriteBufferSize is the bufio buffer size used for atomic file exports.
const fileWriteBufferSize = 65536
