// Package auditlog provides an audit logging library for [Azure/go-workflow].
//
// It records every step execution event — attempts, retries, durations, errors,
// dependencies, and final statuses — with timestamped events and export to JSON
// and NDJSON.
//
// # Quick start
//
//	audit, _ := auditlog.New(auditlog.Config{WorkflowID: "checkout"})
//	w := &flow.Workflow{}
//	w.Add(
//		flow.Step(fetch),
//		flow.Step(save).DependsOn(fetch),
//	)
//
//	audit.Attach(w)          // inject callbacks BEFORE Do
//	err := w.Do(ctx)         // run the workflow
//	audit.Snapshot(w)        // capture final DAG state AFTER Do
//
//	report := audit.Report() // machine-readable snapshot
//	_ = audit.ExportToFile("audit.json")
package auditlog
