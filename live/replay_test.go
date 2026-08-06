package live_test

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/go-sse"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	"github.com/larsartmann/go-workflow-auditlog/live"
)

// makeEvent creates a minimal auditlog.Event for testing.
func makeEvent(seq int) auditlog.Event {
	return auditlog.Event{
		Sequence:  seq,
		StepRef:   auditlog.StepRef{Name: fmt.Sprintf("step-%d", seq)},
		EventType: auditlog.EventTypeAttemptStart,
		Phase:     auditlog.PhaseBefore,
	}
}

// TestServer_SSE_ReconnectionReplay broadcasts 3 events, then connects with
// Last-Event-ID: 1 and verifies that events 2 and 3 are replayed before the
// snapshot.
func TestServer_SSE_ReconnectionReplay(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	auditor, err := auditlog.New(auditlog.Config{
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create auditor: %v", err)
	}

	server := live.NewServer(hub, auditor, live.Config{})

	// Broadcast 3 events before any client connects.
	for i := 1; i <= 3; i++ {
		hub.OnEvent(makeEvent(i))
	}

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	// Connect with Last-Event-ID: 1 — "I have event 1, send me 2+".
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	req.Header.Set("Last-Event-ID", "1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect SSE: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)

	var replayedIDs []string

	for scanner.Scan() {
		line := scanner.Text()

		if id, ok := strings.CutPrefix(line, "id: "); ok {
			replayedIDs = append(replayedIDs, id)
		}

		// Stop once we reach the snapshot — replayed events come before it.
		if strings.HasPrefix(line, "event: snapshot") {
			break
		}
	}

	if len(replayedIDs) != 2 {
		t.Fatalf("expected 2 replayed events, got %d: %v", len(replayedIDs), replayedIDs)
	}

	if replayedIDs[0] != "2" || replayedIDs[1] != "3" {
		t.Errorf("expected replayed IDs [2 3], got %v", replayedIDs)
	}
}

// TestServer_SSE_NoReplayOnInitialConnection connects without a Last-Event-ID
// header and verifies that no events are replayed — only the snapshot arrives.
func TestServer_SSE_NoReplayOnInitialConnection(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	auditor, err := auditlog.New(auditlog.Config{
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create auditor: %v", err)
	}

	server := live.NewServer(hub, auditor, live.Config{})

	// Broadcast events before connecting.
	for i := 1; i <= 3; i++ {
		hub.OnEvent(makeEvent(i))
	}

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	// Connect WITHOUT Last-Event-ID — initial connection.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect SSE: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)

	idCount := 0

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "id: ") {
			idCount++
		}

		// Once we reach the snapshot, stop — no replay should precede it.
		if strings.HasPrefix(line, "event: snapshot") {
			break
		}
	}

	if idCount != 0 {
		t.Errorf("expected 0 replayed events on initial connection, got %d", idCount)
	}
}

// TestHub_EventStore_EventsAfterUnknownID verifies that EventsAfter returns
// an empty slice when the lastID is beyond all stored events.
func TestHub_EventStore_EventsAfterUnknownID(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	hub.OnEvent(makeEvent(1))
	hub.OnEvent(makeEvent(2))

	store := hub.EventStore()

	// Request events after ID "99" — nothing should match.
	events, err := store.EventsAfter(sse.NewEventID("99"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("expected 0 events after unknown ID, got %d", len(events))
	}
}

// TestHub_EventStore_ReplayMatchingEvents verifies that EventsAfter returns
// only events with IDs strictly greater than lastID.
func TestHub_EventStore_ReplayMatchingEvents(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	for i := 1; i <= 5; i++ {
		hub.OnEvent(makeEvent(i))
	}

	store := hub.EventStore()

	// Request events after ID "2" — should get events 3, 4, 5.
	events, err := store.EventsAfter(sse.NewEventID("2"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events after ID 2, got %d", len(events))
	}

	for i, evt := range events {
		expectedID := strconv.Itoa(i + 3)

		if evt.ID.Get() != expectedID {
			t.Errorf("event %d: expected ID %s, got %s", i, expectedID, evt.ID.Get())
		}
	}
}

// TestHub_RingBufferOverflow verifies that when the buffer exceeds its
// capacity, the oldest events are evicted.
func TestHub_RingBufferOverflow(t *testing.T) {
	t.Parallel()

	// Create a hub with a tiny replay buffer.
	hub := live.NewHubWithReplay(3)

	for i := 1; i <= 5; i++ {
		hub.OnEvent(makeEvent(i))
	}

	// Only the last 3 events should remain.
	if hub.BufferedEventCount() != 3 {
		t.Fatalf("expected 3 buffered events, got %d", hub.BufferedEventCount())
	}

	store := hub.EventStore()

	// Events after ID "0" should return only events 3, 4, 5 (oldest evicted).
	events, err := store.EventsAfter(sse.NewEventID("0"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events in buffer, got %d", len(events))
	}

	expectedIDs := []string{"3", "4", "5"}

	for i, evt := range events {
		if evt.ID.Get() != expectedIDs[i] {
			t.Errorf("event %d: expected ID %s, got %s", i, expectedIDs[i], evt.ID.Get())
		}
	}
}

// TestHub_EventStore_ConcurrentReplaySafety verifies that broadcasting events
// concurrently with reading from the EventStore does not race or panic.
func TestHub_EventStore_ConcurrentReplaySafety(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()

	// Seed with some events.
	for i := 1; i <= 50; i++ {
		hub.OnEvent(makeEvent(i))
	}

	store := hub.EventStore()

	var wg sync.WaitGroup

	// Concurrent broadcasters.
	for i := range 4 {
		wg.Add(1)

		go func(offset int) {
			defer wg.Done()

			for j := range 50 {
				hub.OnEvent(makeEvent(100 + offset*50 + j))
			}
		}(i)
	}

	// Concurrent readers.
	for i := range 4 {
		wg.Add(1)

		go func(n int) {
			defer wg.Done()

			for j := range 50 {
				lastID := sse.NewEventID(strconv.Itoa(n*10 + j))
				_, _ = store.EventsAfter(lastID)
			}
		}(i)
	}

	wg.Wait()
}
