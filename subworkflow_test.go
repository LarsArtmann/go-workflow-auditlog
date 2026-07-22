package auditlog_test

import (
	"context"
	"testing"

	flow "github.com/Azure/go-workflow"
	testhelpers "github.com/larsartmann/go-workflow-auditlog/testhelpers"
)

// subFetchStep and subSaveStep are inner steps of a sub-workflow.
type subFetchStep struct{ name string }

func (s *subFetchStep) Do(_ context.Context) error { return nil }
func (s *subFetchStep) String() string             { return s.name }

type subSaveStep struct{ name string }

func (s *subSaveStep) Do(_ context.Context) error { return nil }
func (s *subSaveStep) String() string             { return s.name }

// compositeStep embeds a SubWorkflow with inner steps.
type compositeStep struct {
	flow.SubWorkflow
}

func (c *compositeStep) BuildStep() {
	c.Reset()
	c.Add(
		flow.Step(&subFetchStep{name: "inner-fetch"}),
		flow.Step(&subSaveStep{name: "inner-save"}).DependsOn(&subFetchStep{name: "inner-fetch"}),
	)
}

func (c *compositeStep) String() string { return "composite" }

func TestSubWorkflow_InnerStepsCaptured(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	comp := &compositeStep{}
	comp.BuildStep()
	w.Add(flow.Step(comp))

	a.Attach(w)
	_ = w.Do(context.Background())
	a.Snapshot(w)

	report := a.Report()

	// The inner steps should be visible in the report via traversal.
	foundFetch := false
	foundSave := false

	for _, step := range report.Steps {
		if step.Name == "inner-fetch" {
			foundFetch = true
		}

		if step.Name == "inner-save" {
			foundSave = true
		}
	}

	if !foundFetch {
		t.Error("expected to find inner-fetch step from sub-workflow")
	}

	if !foundSave {
		t.Error("expected to find inner-save step from sub-workflow")
	}
}

func TestSubWorkflow_CompositeStepPresent(t *testing.T) {
	t.Parallel()

	a, w := testhelpers.NewAuditAndWorkflow(t)
	comp := &compositeStep{}
	comp.BuildStep()
	w.Add(flow.Step(comp))

	a.Attach(w)
	_ = w.Do(context.Background())
	a.Snapshot(w)

	report := a.Report()

	// The composite step itself should also be present.
	found := false

	for _, step := range report.Steps {
		if step.Name == "composite" {
			found = true
		}
	}

	if !found {
		t.Error("expected to find composite root step")
	}
}
