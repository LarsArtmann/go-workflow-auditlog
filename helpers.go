package auditlog

import (
	"fmt"
	"io"
	"os"
	"regexp"

	atomicwrite "github.com/larsartmann/go-atomic-write"
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

// WriteToFile creates a file at path and writes to it atomically via a
// streaming callback. Delegates to go-atomic-write for TOCTOU-safe writes
// with fsync durability and cross-platform atomic rename. Errors are wrapped
// with ErrExportWriteFailed for errors.Is compatibility.
func WriteToFile(path string, fn func(io.Writer) error) error {
	err := atomicwrite.WriteFunc(path, fn, atomicwrite.Fingerprint{})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrExportWriteFailed, err)
	}

	return nil
}
