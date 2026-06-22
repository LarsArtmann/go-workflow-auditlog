package auditlog_test

import (
	"os"
	"path/filepath"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func TestCheckNoClobber_NonexistentFile_ReturnsNil(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "new-file.json")

	err := auditlog.CheckNoClobber(path)
	if err != nil {
		t.Errorf("CheckNoClobber(nonexistent) = %v, want nil", err)
	}
}

func TestCheckNoClobber_ExistingFile_ReturnsErrFileExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "existing.json")

	writeErr := os.WriteFile(path, []byte("{}"), 0o644)
	if writeErr != nil {
		t.Fatalf("setup: WriteFile error: %v", writeErr)
	}

	err := auditlog.CheckNoClobber(path)
	if err == nil {
		t.Fatal("CheckNoClobber(existing) = nil, want error")
	}
}

func TestHasPointerAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want bool
	}{
		{"*TestStep(0xc0000a4000)", true},
		{"*FetchStep(0x1400005a020)", true},
		{"*MyStep(0x0)", true}, // nil pointer — still valid format
		{"fetch-step", false},
		{"TestStep", false},
		{"", false},
		{"*TestStep()", false},       // no hex address
		{"*TestStep(0xZZZZ)", false}, // not hex
		{"plain string", false},
		{"*a(0xc0000a4000)", true}, // single-char type name
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := auditlog.HasPointerAddress(tc.name)
			if got != tc.want {
				t.Errorf("HasPointerAddress(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestNameCollisions_NoCollisions(t *testing.T) {
	t.Parallel()

	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "fetch"}},
			{StepRef: auditlog.StepRef{Name: "validate"}},
			{StepRef: auditlog.StepRef{Name: "save"}},
		},
	}

	collisions := report.NameCollisions()
	if len(collisions) != 0 {
		t.Errorf("NameCollisions() = %v, want empty", collisions)
	}
}

func TestNameCollisions_WithCollisions(t *testing.T) {
	t.Parallel()

	report := auditlog.WorkflowReport{
		Steps: []auditlog.StepInfo{
			{StepRef: auditlog.StepRef{Name: "fetch"}},
			{StepRef: auditlog.StepRef{Name: "fetch"}}, // duplicate
			{StepRef: auditlog.StepRef{Name: "validate"}},
		},
	}

	collisions := report.NameCollisions()
	if len(collisions) != 1 {
		t.Fatalf("NameCollisions() = %v (len %d), want 1 collision", collisions, len(collisions))
	}

	if collisions[0] != "fetch" {
		t.Errorf("NameCollisions()[0] = %q, want %q", collisions[0], "fetch")
	}
}
