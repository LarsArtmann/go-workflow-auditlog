package live

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/larsartmann/go-sse"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// subscriberBufferSize is the per-client event buffer. Events that overflow
// are dropped for that client — the snapshot mechanism on reconnect will
// recover the full state.
const (
	subscriberBufferSize = 128
	eventNameEvent       = "event"
)

// BroadcastEvent carries a marshaled event payload alongside its SSE event ID.
// SSE clients use the ID for Last-Event-ID reconnection replay; WebSocket
// clients ignore it.
type BroadcastEvent struct {
	ID   sse.EventID
	Data jsontext.Value
}

// Subscriber represents a single SSE client connection.
type Subscriber struct {
	id        uint64
	ch        chan BroadcastEvent
	done      chan struct{}
	closeOnce sync.Once
}

// ID returns the subscriber's unique identifier.
func (s *Subscriber) ID() uint64 { return s.id }

// Events returns the channel that receives broadcast events.
func (s *Subscriber) Events() <-chan BroadcastEvent { return s.ch }

// Done returns a channel that is closed when the lifecycle completes
// or the subscriber is removed.
func (s *Subscriber) Done() <-chan struct{} { return s.done }

func (s *Subscriber) closeDone() {
	s.closeOnce.Do(func() { close(s.done) })
}

// Hub fans out workflow events to all connected SSE clients.
//
// The hub is safe for concurrent use. OnEvent is called from recorder
// goroutines, and Subscribe/Unsubscribe are called from HTTP handler goroutines.
type Hub struct {
	mu         sync.RWMutex
	clients    map[uint64]*Subscriber
	nextID     uint64
	complete   bool
	draining   bool
	eventSeq   atomic.Uint64
	ringBuffer *eventRingBuffer
}

// NewHub creates a Hub ready for use with the default replay buffer size.
func NewHub() *Hub {
	return NewHubWithReplay(0)
}

// NewHubWithReplay creates a Hub with a replay ring buffer of the given
// capacity. Non-positive capacity uses the default (1000).
func NewHubWithReplay(replayBufferSize int) *Hub {
	return &Hub{
		clients:    make(map[uint64]*Subscriber),
		ringBuffer: newEventRingBuffer(replayBufferSize),
	}
}

// OnEvent marshals a workflow Event to JSON, assigns it a sequential SSE
// event ID, stores it in the replay ring buffer, and broadcasts it to all
// connected clients.
func (h *Hub) OnEvent(evt auditlog.Event) {
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}

	seq := h.eventSeq.Add(1)
	id := sse.NewEventID(strconv.FormatUint(seq, 10))

	h.ringBuffer.add(sse.Event{Event: eventNameEvent, ID: id, Data: string(data)})

	h.broadcast(BroadcastEvent{ID: id, Data: data})
}

// broadcast sends a message to all connected subscribers. Non-blocking:
// if a subscriber's buffer is full, the event is dropped for that subscriber.
func (h *Hub) broadcast(msg BroadcastEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, sub := range h.clients {
		select {
		case sub.ch <- msg:
		default:
		}
	}
}

// Subscribe registers a new SSE client and returns a subscriber.
func (h *Hub) Subscribe() *Subscriber {
	h.mu.Lock()
	defer h.mu.Unlock()

	subID := h.nextID
	h.nextID++

	sub := &Subscriber{
		id:   subID,
		ch:   make(chan BroadcastEvent, subscriberBufferSize),
		done: make(chan struct{}),
	}
	h.clients[subID] = sub

	return sub
}

// Unsubscribe removes a subscriber by ID and signals its done channel.
func (h *Hub) Unsubscribe(subscriberID uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	sub, ok := h.clients[subscriberID]
	if !ok {
		return
	}

	sub.closeDone()
	delete(h.clients, subscriberID)
}

// SignalComplete marks the lifecycle as finished. All subscribers
// receive a done signal so the SSE handler can send the final report.
func (h *Hub) SignalComplete() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.complete = true

	for _, sub := range h.clients {
		sub.closeDone()
	}
}

// IsComplete returns whether the lifecycle has been marked as complete.
func (h *Hub) IsComplete() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.complete
}

// ClientCount returns the number of currently connected subscribers.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.clients)
}

// EventStore returns the replay ring buffer as an sse.EventStore for
// SSE reconnection replay via sse.Replay.
//
//nolint:ireturn // consumers need the interface for sse.Replay
func (h *Hub) EventStore() sse.EventStore {
	return h.ringBuffer
}

// BufferedEventCount returns the number of events currently stored in the
// replay ring buffer.
func (h *Hub) BufferedEventCount() int {
	return h.ringBuffer.len()
}

// drainPollInterval is how often Drain re-checks subscriber channel buffers.
const drainPollInterval = time.Millisecond

// Drain gracefully waits for all subscriber channel buffers to empty
// (consumers catch up) or the context to timeout. After drain begins, the
// hub is marked draining. Returns nil on a clean drain, or ctx.Err() if the
// deadline fires before buffers empty.
func (h *Hub) Drain(ctx context.Context) error {
	h.mu.Lock()
	h.draining = true

	subs := make([]*Subscriber, 0, len(h.clients))
	for _, sub := range h.clients {
		subs = append(subs, sub)
	}

	h.mu.Unlock()

	ticker := time.NewTicker(drainPollInterval)
	defer ticker.Stop()

	for {
		allEmpty := true

		for _, sub := range subs {
			if len(sub.ch) > 0 {
				allEmpty = false

				break
			}
		}

		if allEmpty {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("drain: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// IsDraining returns whether the hub is currently in a drain state
// (Server.Shutdown is waiting for subscriber buffers to empty).
func (h *Hub) IsDraining() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.draining
}
