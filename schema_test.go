package auditlog_test

import (
	"encoding/json/v2"
	"strings"
	"testing"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func TestJSONSchema_NonEmpty(t *testing.T) {
	t.Parallel()

	schema := auditlog.JSONSchema()
	if schema == "" {
		t.Fatal("expected non-empty JSON schema")
	}

	if !strings.Contains(schema, "workflow_id") {
		t.Error("expected schema to contain workflow_id property")
	}
}

func TestJSONSchema_ValidJSON(t *testing.T) {
	t.Parallel()

	schema := auditlog.JSONSchema()

	var parsed map[string]any
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	if parsed["$schema"] == nil {
		t.Error("expected $schema field in JSON schema")
	}

	if parsed["title"] == nil {
		t.Error("expected title field in JSON schema")
	}
}
