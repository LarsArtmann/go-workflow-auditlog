package live

import (
	"encoding/json"
	"sync"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
	corelive "github.com/larsartmann/auditlog-core/live"
)

// Hub wraps corelive.Hub, adding domain-specific OnEvent method.
type Hub struct {
	core   *corelive.Hub
	auditor *auditlog.Auditor
	mu     sync.RWMutex
}

// NewHub creates a Hub. Pass nil when using live.New() (set internally).
func NewHub(auditor *auditlog.Auditor) *Hub {
	return &Hub{
		core:    corelive.NewHub(),
		auditor: auditor,
	}
}

// SetAuditor sets the auditor after construction.
func (h *Hub) SetAuditor(auditor *auditlog.Auditor) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.auditor = auditor
}

// OnEvent broadcasts an auditlog.Event to all connected SSE clients.
func (h *Hub) OnEvent(evt auditlog.Event) {
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}

	h.core.OnEvent(data)
}

// SignalComplete marks the workflow as finished.
func (h *Hub) SignalComplete() {
	h.core.SignalComplete()
}

// IsComplete returns whether the workflow has been marked as complete.
func (h *Hub) IsComplete() bool {
	return h.core.IsComplete()
}

// ClientCount returns the number of currently connected SSE clients.
func (h *Hub) ClientCount() int {
	return h.core.ClientCount()
}

// Subscribe registers a new SSE client. For testing only.
func (h *Hub) Subscribe() *corelive.Subscriber {
	return h.core.Subscribe()
}

// Unsubscribe removes a subscriber by ID. For testing only.
func (h *Hub) Unsubscribe(id uint64) {
	h.core.Unsubscribe(id)
}
