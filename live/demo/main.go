// Package main is a demo of the live workflow dashboard.
//
// It runs a multi-step data pipeline with intentional delays so you can
// watch the dashboard update in real time as steps start, succeed, fail,
// retry, and reuse cached results.
//
// Run with:
//
//	GOEXPERIMENT=jsonv2 go run ./demo
//
// Then open http://localhost:8080 in your browser.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/cenkalti/backoff/v4"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	"github.com/larsartmann/go-workflow-auditlog/live"
)

// --- Demo Steps ---

type fetchStep struct{ url string }

func (s *fetchStep) String() string { return "fetch" }
func (s *fetchStep) Do(_ context.Context) error {
	time.Sleep(800 * time.Millisecond)

	return nil
}

type validateStep struct{}

func (s *validateStep) String() string { return "validate" }
func (s *validateStep) Do(_ context.Context) error {
	time.Sleep(300 * time.Millisecond)

	return nil
}

type transformStep struct{}

func (s *transformStep) String() string { return "transform" }
func (s *transformStep) Do(_ context.Context) error {
	time.Sleep(600 * time.Millisecond)

	return nil
}

type enrichStep struct{}

func (s *enrichStep) String() string { return "enrich" }
func (s *enrichStep) Do(_ context.Context) error {
	time.Sleep(700 * time.Millisecond)

	return nil
}

type flakySaveStep struct{ calls int }

func (s *flakySaveStep) String() string { return "save" }
func (s *flakySaveStep) Do(_ context.Context) error {
	s.calls++
	time.Sleep(400 * time.Millisecond)

	if s.calls < 3 {
		return errors.New("database connection timeout")
	}

	return nil
}

type notifyStep struct{}

func (s *notifyStep) String() string { return "notify" }
func (s *notifyStep) Do(_ context.Context) error {
	time.Sleep(300 * time.Millisecond)

	return nil
}

// cachedDetectStep models a warm result cache: a previous run already did the
// detect work, so this attempt reuses the stored result instead of
// re-executing it. MarkCached attributes the cache hit in the audit trail —
// the step still shows as succeeded, but honestly marked as cached.
type cachedDetectStep struct{}

func (s *cachedDetectStep) String() string { return "detect" }
func (s *cachedDetectStep) Do(ctx context.Context) error {
	time.Sleep(50 * time.Millisecond)

	return auditlog.MarkCached(ctx)
}

func main() {
	server, auditor, err := live.New(auditlog.Config{
		WorkflowID: "data-pipeline-demo",
	}, live.Config{
		Addr:              ":18080",
		HeartbeatInterval: 10 * time.Second,
	})
	if err != nil {
		log.Fatalf("create server: %v", err)
	}

	w := &flow.Workflow{}

	fetch := &fetchStep{url: "https://api.example.com/data"}
	validate := &validateStep{}
	transform := &transformStep{}
	enrich := &enrichStep{}
	save := &flakySaveStep{}
	notify := &notifyStep{}
	detect := &cachedDetectStep{}

	w.Add(
		flow.Step(detect),
		flow.Step(fetch).DependsOn(detect),
		flow.Step(validate).DependsOn(fetch),
		flow.Step(transform).DependsOn(validate),
		flow.Step(enrich).DependsOn(validate),
		flow.Step(save).DependsOn(transform, enrich).Retry(func(o *flow.RetryOption) {
			o.Attempts = 5
			o.Backoff = backoff.NewExponentialBackOff()
		}),
		flow.Step(notify).DependsOn(save),
	)

	auditor.Attach(w)
	auditor.CaptureDAG(w)

	go func() {
		fmt.Println("============================================")
		fmt.Println("  Live Dashboard: http://localhost:18080")
		fmt.Println("============================================")

		if err := server.ListenAndServe(); err != nil {
			log.Printf("server: %v", err)
		}
	}()

	time.Sleep(200 * time.Millisecond)

	fmt.Println("Starting workflow execution...")
	err = w.Do(context.Background())
	if err != nil {
		fmt.Printf("Workflow completed with error: %v\n", err)
	} else {
		fmt.Println("Workflow completed successfully!")
	}

	auditor.Snapshot(w)
	server.SignalComplete()

	report := auditor.Report()
	fmt.Printf("\nFinal: %d steps, %d events, succeeded=%v, cached=%d\n",
		report.StepCount, report.EventCount, report.WorkflowSucceeded, report.CachedStepCount)
	fmt.Println("\nDashboard is live. Press Ctrl+C to exit.")

	select {}
}
