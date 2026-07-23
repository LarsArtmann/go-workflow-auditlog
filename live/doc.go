// Package live provides a real-time HTTP dashboard for workflow execution.
//
// It serves an interactive HTML dashboard that updates live via Server-Sent
// Events (SSE) as the workflow executes. Steps light up as they start, change
// color as they succeed or fail, and the full DAG structure snaps into place
// when the workflow completes.
//
// # Quick Start
//
//	server, auditor, err := live.New(auditlog.Config{
//		WorkflowID: "my-pipeline",
//	}, live.Config{
//		Addr: ":8080",
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	auditor.Attach(workflow)
//	go server.ListenAndServe()
//
//	fmt.Println("Live dashboard: http://localhost:8080")
//
//	workflow.Do(ctx)
//	auditor.Snapshot(workflow)
//	server.SignalComplete()
//
// # Architecture
//
// The live server uses SSE (Server-Sent Events) for real-time communication:
//
//   - GET /            - Interactive dashboard HTML (static, cached)
//   - GET /api/report  - Current report as JSON (point-in-time snapshot)
//   - GET /api/events  - SSE stream (snapshot + live events + completion)
//   - GET /api/health  - Health check
//
// SSE was chosen over WebSocket because the data flow is one-way
// (server to browser), SSE has native browser support via EventSource,
// auto-reconnects on disconnect, and requires no framing protocol.
//
// # Protocol
//
// The SSE stream sends three named event types:
//
//   - snapshot: Initial state on connect (report + events + metadata + DAG)
//   - event: Individual events as they fire during execution
//   - complete: Final report with full DAG structure after Snapshot
//
// Late clients receive the full state via the snapshot event, including all
// events captured so far. After completion, new clients get the final report.
package live
