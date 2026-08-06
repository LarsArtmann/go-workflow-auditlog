package live_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
	"github.com/larsartmann/go-workflow-auditlog/live"
)

// TestHub_Drain_DeliversBufferedEvents verifies that Drain waits for
// subscriber channel buffers to empty before returning. It broadcasts
// events to a subscriber that isn't consuming, then starts Drain in a
// goroutine. A consumer goroutine drains the channel, and Drain should
// complete once the channel is empty.
func TestHub_Drain_DeliversBufferedEvents(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	sub := hub.Subscribe()

	defer hub.Unsubscribe(sub.ID())

	// Broadcast 10 events without consuming.
	for i := range 10 {
		hub.OnEvent(auditlog.Event{
			Sequence:  i + 1,
			StepRef:   auditlog.StepRef{Name: "drain-test"},
			EventType: auditlog.EventTypeAttemptStart,
			Phase:     auditlog.PhaseBefore,
		})
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	// Start consuming in a goroutine.
	go func() {
		for range sub.Events() {
		}
	}()

	// Drain should succeed once the consumer catches up.
	err := hub.Drain(ctx)

	cancel() // stop the consumer

	if err != nil {
		t.Fatalf("expected nil error from Drain, got %v", err)
	}

	if !hub.IsDraining() {
		t.Error("expected hub to be in draining state after Drain")
	}
}

// TestHub_Drain_Timeout verifies that Drain returns ctx.Err() when the
// context deadline fires before subscriber buffers empty.
func TestHub_Drain_Timeout(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	sub := hub.Subscribe()

	defer hub.Unsubscribe(sub.ID())

	// Broadcast events without consuming.
	for i := range 10 {
		hub.OnEvent(auditlog.Event{
			Sequence:  i + 1,
			StepRef:   auditlog.StepRef{Name: "drain-timeout"},
			EventType: auditlog.EventTypeAttemptStart,
			Phase:     auditlog.PhaseBefore,
		})
	}

	// Very short timeout — drain should not complete.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Millisecond)
	defer cancel()

	err := hub.Drain(ctx)
	if err == nil {
		t.Fatal("expected timeout error from Drain with short deadline")
	}
}

// TestServer_Health_ReportsDrainState verifies that the health endpoint
// includes draining and event_buffer_size fields.
func TestServer_Health_ReportsDrainState(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	auditor, err := auditlog.New(auditlog.Config{Enabled: true})
	if err != nil {
		t.Fatalf("create auditor: %v", err)
	}

	server := live.NewServer(hub, auditor, live.Config{})

	// Broadcast an event so the buffer is non-empty.
	hub.OnEvent(auditlog.Event{
		Sequence:  1,
		StepRef:   auditlog.StepRef{Name: "health-test"},
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
	})

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx := t.Context()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/health", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("health request: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	bodyStr := string(body)

	if !strings.Contains(bodyStr, "\"event_buffer_size\"") {
		t.Errorf("health response should contain event_buffer_size: %s", bodyStr)
	}

	if !strings.Contains(bodyStr, "\"draining\"") {
		t.Errorf("health response should contain draining field: %s", bodyStr)
	}
}
