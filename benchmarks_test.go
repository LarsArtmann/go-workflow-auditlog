package auditlog_test

import (
	"context"
	"fmt"
	"testing"

	flow "github.com/Azure/go-workflow"
	"github.com/cenkalti/backoff/v4"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// BenchmarkInvocation measures the overhead of audit callbacks on a single
// step invocation (the hot path).
func BenchmarkInvocation(b *testing.B) {
	b.Run("disabled", func(b *testing.B) {
		a, _ := auditlog.New(auditlog.Config{Enabled: false})

		for range b.N {
			b.StopTimer()

			w := &flow.Workflow{}
			w.Add(flow.Step(newSucceed("step")))
			a.Attach(w)
			b.StartTimer()

			_ = w.Do(context.Background())
		}
	})

	b.Run("enabled", func(b *testing.B) {
		a, _ := auditlog.New(auditlog.Config{Enabled: true})

		for range b.N {
			b.StopTimer()

			w := &flow.Workflow{}
			w.Add(flow.Step(newSucceed("step")))
			a.Attach(w)
			b.StartTimer()

			_ = w.Do(context.Background())
			a.Snapshot(w)
		}
	})
}

// BenchmarkRegistration measures the cost of Attach (callback injection) with
// varying numbers of steps.
func BenchmarkAttach(b *testing.B) {
	for _, n := range []int{10, 50, 100} {
		b.Run(fmt.Sprintf("%d-steps", n), func(b *testing.B) {
			for range b.N {
				b.StopTimer()

				a, _ := auditlog.New(auditlog.Config{Enabled: true})
				w := &flow.Workflow{}

				steps := make([]flow.Steper, 0, n)
				for j := range n {
					s := newSucceed(fmt.Sprintf("step-%d", j))
					steps = append(steps, s)
					w.Add(flow.Step(s))
				}

				b.StartTimer()

				a.Attach(w)
			}
		})
	}
}

// BenchmarkBuildReport measures report assembly from varying step counts.
func BenchmarkBuildReport(b *testing.B) {
	for _, n := range []int{50, 100, 500} {
		b.Run(fmt.Sprintf("%d-steps", n), func(b *testing.B) {
			a, _ := auditlog.New(auditlog.Config{Enabled: true})
			w := &flow.Workflow{}

			steps := make([]flow.Steper, 0, n)
			for j := range n {
				s := newSucceed(fmt.Sprintf("step-%d", j))
				steps = append(steps, s)
				w.Add(flow.Step(s))
			}

			a.Attach(w)
			_ = w.Do(context.Background())
			a.Snapshot(w)

			b.ResetTimer()

			for range b.N {
				_ = a.Report()
			}
		})
	}
}

// BenchmarkEventsCopy measures the cost of copying the events slice.
func BenchmarkEventsCopy(b *testing.B) {
	a, _ := auditlog.New(auditlog.Config{Enabled: true})

	w := &flow.Workflow{}
	for j := range 100 {
		w.Add(flow.Step(newSucceed(fmt.Sprintf("step-%d", j))))
	}

	a.Attach(w)
	_ = w.Do(context.Background())
	a.Snapshot(w)

	b.ResetTimer()

	for range b.N {
		_ = a.Events()
	}
}

// BenchmarkOnEventCallback measures the overhead of the OnEvent callback.
func BenchmarkOnEventCallback(b *testing.B) {
	a, _ := auditlog.New(auditlog.Config{
		Enabled: true,
		OnEvent: func(e auditlog.Event) {}, // no-op callback
	})

	for range b.N {
		b.StopTimer()

		w := &flow.Workflow{}
		w.Add(flow.Step(newSucceed("step")))
		a.Attach(w)
		b.StartTimer()

		_ = w.Do(context.Background())
	}
}

// BenchmarkRetryWithAudit measures the cost of auditing retried steps.
func BenchmarkRetryWithAudit(b *testing.B) {
	a, _ := auditlog.New(auditlog.Config{Enabled: true})

	for range b.N {
		b.StopTimer()

		w := &flow.Workflow{}
		step := &flakyStep{name: "bench-flaky", failUntil: 2}
		w.Add(
			flow.Step(step).Retry(func(o *flow.RetryOption) {
				o.Attempts = 5
				o.Backoff = backoff.NewExponentialBackOff()
			}),
		)
		a.Attach(w)
		b.StartTimer()

		_ = w.Do(context.Background())
		a.Snapshot(w)
	}
}

// BenchmarkMermaidExport measures the cost of generating Mermaid diagrams.
func BenchmarkMermaidExport(b *testing.B) {
	for _, n := range []int{10, 50, 100} {
		b.Run(fmt.Sprintf("%d-steps", n), func(b *testing.B) {
			a, _ := auditlog.New(auditlog.Config{Enabled: true})
			w := &flow.Workflow{}

			var prev flow.Steper

			for j := range n {
				s := newSucceed(fmt.Sprintf("step-%d", j))
				if prev != nil {
					w.Add(flow.Step(s).DependsOn(prev))
				} else {
					w.Add(flow.Step(s))
				}

				prev = s
			}

			a.Attach(w)
			_ = w.Do(context.Background())
			a.Snapshot(w)

			report := a.Report()

			b.ResetTimer()

			for range b.N {
				_ = report.WriteMermaid(&discardWriter{})
			}
		})
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
