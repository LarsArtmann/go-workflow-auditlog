// Package forward streams workflow audit events to a PapDashboard
// audit-run collector: batches events per run, POSTs them over a unix
// socket (zero config — the default local socket is probed once) or HTTP,
// and marks run completion via the explicit terminal marker (workflow runs
// have no root-scope shutdown to derive it from). Stdlib-only on purpose:
// this is a PUBLIC repo and must stay fetchable through proxy.golang.org.
//
// Zero-code wiring: live.New auto-attaches an enabled Forwarder to the
// audit pipeline; plain consumers compose it themselves:
//
//	fwd := forward.New("") // source defaults to the executable name
//	if fwd.Enabled() {
//		auditor, _ := auditlog.New(auditlog.Config{
//			OnEvent: fwd.OnEvent, // combine with your own callback as needed
//		})
//		defer fwd.Shutdown(context.Background())
//	}
//
// Target selection (WORKFLOW_AUDITLOG_FORWARD_TARGET):
//
//	unset          probe $XDG_RUNTIME_DIR/papdashboard/audit-runs.sock once;
//	              forwarding stays off when it is absent or dead
//	off, disabled  never forward
//	unix:///path   explicit unix socket
//	/path          bare absolute path is a unix socket
//	http(s)://…    remote collector (pair with WORKFLOW_AUDITLOG_FORWARD_API_KEY)
package forward

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// Env knobs: target selection, HTTP bearer key, and the source label the
// collector's UI shows for this workflow.
const (
	EnvTarget = "WORKFLOW_AUDITLOG_FORWARD_TARGET"
	EnvAPIKey = "WORKFLOW_AUDITLOG_FORWARD_API_KEY"
	EnvSource = "WORKFLOW_AUDITLOG_FORWARD_SOURCE"
)

// Batching bounds: the flusher wakes on the earlier of the ticker or a full
// batch; the channel cap bounds memory under burst load (drop-oldest).
const (
	flushInterval  = 250 * time.Millisecond
	maxBatchEvents = 200
	channelCap     = 4096

	// dialTimeout bounds socket connection establishment (probe + dial).
	dialTimeout = 150 * time.Millisecond

	// requestTimeout bounds one POST (the collector answers fast; a stuck
	// socket must not stall the flusher for long).
	requestTimeout = 5 * time.Second

	// ingestPath is the collector's ingest route (TCP and socket-local).
	ingestPath = "/events"

	// unixHost routes socket URLs through the custom dialer.
	unixHost = "papdashboard.internal"
)

// envelope is the PapDashboard ingest contract (kind "workflow"). Events
// ride auditlog.Event's own snake_case JSON tags verbatim — the collector's
// RawEvent is a structural superset of them.
type envelope struct {
	Kind     string           `json:"kind"`
	SourceID string           `json:"sourceId"`
	RunID    string           `json:"runId"`
	Events   []auditlog.Event `json:"events"`
	Complete bool             `json:"complete"`
}

// Forwarder batches workflow audit events per run and forwards them to a
// PapDashboard collector. OnEvent never blocks: events land in a bounded
// channel; overflow drops the OLDEST pending event (the local live
// dashboard keeps the full history).
type Forwarder struct {
	url    string
	apiKey string
	client *http.Client

	sourceID string

	events   chan auditlog.Event
	complete chan struct{}
	lastRun  atomic.Value // string: most recent RunID seen
	dropped  atomic.Int64
	closed   atomic.Bool

	closeOnce sync.Once
	stop      chan struct{} // closed by Shutdown to signal the flusher
	stopped   chan struct{} // closed by the flusher after its final flush
}

// New resolves the target from the environment and returns a Forwarder.
// sourceID identifies this workflow in the collector's UI ("" falls back to
// WORKFLOW_AUDITLOG_FORWARD_SOURCE, then the executable name). The returned
// Forwarder is always non-nil; when no target resolves it is disabled and
// every method is a safe no-op.
func New(sourceID string) *Forwarder {
	return NewWithTarget(os.Getenv(EnvTarget), sourceID)
}

// NewWithTarget builds a Forwarder for an explicit target string (same
// syntax as WORKFLOW_AUDITLOG_FORWARD_TARGET; "" probes the default socket).
func NewWithTarget(target, sourceID string) *Forwarder {
	url, client := resolveTarget(target)
	if url == "" {
		return &Forwarder{}
	}

	if sourceID == "" {
		sourceID = os.Getenv(EnvSource)
	}

	if sourceID == "" {
		if executable, err := os.Executable(); err == nil {
			sourceID = filepath.Base(executable)
		} else {
			sourceID = "unknown"
		}
	}

	f := &Forwarder{
		url:      url,
		apiKey:   os.Getenv(EnvAPIKey),
		client:   client,
		sourceID: sourceID,
		events:   make(chan auditlog.Event, channelCap),
		complete: make(chan struct{}, 1),
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}

	go f.loop()

	return f
}

// resolveTarget maps a target spec onto an ingest URL + HTTP client, or
// ("", nil) when forwarding is off. Unix targets route a synthetic host
// through a dialer bound to the socket path; the default (empty spec)
// probes the conventional socket once and stays off when dead.
func resolveTarget(target string) (string, *http.Client) {
	socketTarget := func(path string) (string, *http.Client) {
		return "http://" + unixHost + ingestPath, unixClient(path)
	}

	switch {
	case strings.EqualFold(target, "off"), strings.EqualFold(target, "disabled"):
		return "", nil

	case strings.HasPrefix(target, "unix://"):
		return socketTarget(strings.TrimPrefix(target, "unix://"))

	case strings.HasPrefix(target, "http://"), strings.HasPrefix(target, "https://"):
		return strings.TrimRight(target, "/") + ingestPath, &http.Client{Timeout: requestTimeout}

	case strings.HasPrefix(target, "/"):
		return socketTarget(target)

	default:
		path := DefaultSocketPath()
		if !probeSocket(path) {
			return "", nil
		}

		return socketTarget(path)
	}
}

// DefaultSocketPath is the conventional PapDashboard audit-run socket,
// mirroring the server's default (runtime dir first, /tmp fallback).
func DefaultSocketPath() string {
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "papdashboard", "audit-runs.sock")
	}

	return filepath.Join(os.TempDir(), "papdashboard", "audit-runs.sock")
}

// unixClient builds an HTTP client whose dialer ignores the synthetic host
// and connects to the given socket path.
func unixClient(socketPath string) *http.Client {
	return &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				dialer := &net.Dialer{Timeout: dialTimeout}

				return dialer.DialContext(ctx, "unix", socketPath)
			},
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     30 * time.Second,
		},
	}
}

// probeSocket reports whether a unix socket answers a dial.
func probeSocket(path string) bool {
	conn, err := net.DialTimeout("unix", path, dialTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()

	return true
}

// Enabled reports whether a target resolved and forwarding is active.
func (f *Forwarder) Enabled() bool { return f.url != "" }

// Dropped returns how many events were dropped from a full buffer.
func (f *Forwarder) Dropped() int64 { return f.dropped.Load() }

// OnEvent enqueues one event (auditlog.Config.OnEvent-compatible). It never
// blocks: a full buffer drops the oldest pending event.
func (f *Forwarder) OnEvent(evt auditlog.Event) {
	if !f.Enabled() || f.closed.Load() {
		return
	}

	if runID := string(evt.RunID); runID != "" {
		f.lastRun.Store(runID)
	}

	select {
	case f.events <- evt:
		return
	default:
	}

	// Buffer full: drop the oldest, then retry once.
	select {
	case <-f.events:
		f.dropped.Add(1)
	default:
	}

	select {
	case f.events <- evt:
	default:
		f.dropped.Add(1)
	}
}

// Complete marks the most recently seen run as finished. Workflow runs have
// no derivable terminal event, so the explicit marker (driven by
// live.Server.SignalComplete) is the ONLY completion signal.
func (f *Forwarder) Complete() {
	if !f.Enabled() || f.closed.Load() {
		return
	}

	select {
	case f.complete <- struct{}{}:
	default:
	}
}

// Shutdown flushes everything still buffered and stops the flusher. Safe to
// call multiple times; blocks until the final flush finished or ctx ends.
func (f *Forwarder) Shutdown(ctx context.Context) error {
	if !f.Enabled() {
		return nil
	}

	f.closeOnce.Do(func() {
		f.closed.Store(true)
		close(f.stop)
	})

	select {
	case <-f.stopped:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("forwarder shutdown: %w", ctx.Err())
	}
}

// loop is the flusher: ticker-batched sends plus a final shutdown flush.
func (f *Forwarder) loop() {
	defer close(f.stopped)

	pending := make([]auditlog.Event, 0, maxBatchEvents)
	var wantComplete bool

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	drain := func() {
		for {
			select {
			case evt := <-f.events:
				pending = append(pending, evt)
			case <-f.complete:
				wantComplete = true
			default:
				return
			}
		}
	}

	for {
		select {
		case <-f.stop:
			drain()
			f.flush(pending, wantComplete)

			return
		case <-ticker.C:
			drain()
			f.flush(pending, wantComplete)
			pending = pending[:0]
			wantComplete = false
		}
	}
}

// flush groups the buffer by run id and POSTs one envelope per run; the
// completion marker rides the LAST run's envelope (the run just finished).
func (f *Forwarder) flush(buffer []auditlog.Event, complete bool) {
	if len(buffer) == 0 && !complete {
		return
	}

	byRun := map[string][]auditlog.Event{}
	order := []string{}

	for _, evt := range buffer {
		runID := string(evt.RunID)
		if _, seen := byRun[runID]; !seen {
			order = append(order, runID)
		}

		byRun[runID] = append(byRun[runID], evt)
	}

	for index, runID := range order {
		f.post(envelope{
			Kind:     "workflow",
			SourceID: f.sourceID,
			RunID:    runID,
			Events:   byRun[runID],
			Complete: complete && index == len(order)-1,
		})
	}

	if complete && len(order) == 0 {
		runID, _ := f.lastRun.Load().(string)
		if runID != "" {
			f.post(envelope{Kind: "workflow", SourceID: f.sourceID, RunID: runID, Complete: true})
		}
	}
}

// post sends one batch; failures are silent (the local dashboard remains
// the source of truth, and the collector dedups any later retry).
func (f *Forwarder) post(payload envelope) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, f.url, bytes.NewReader(body))
	if err != nil {
		return
	}

	request.Header.Set("Content-Type", "application/json")

	if f.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+f.apiKey)
	}

	response, err := f.client.Do(request)
	if err != nil {
		return
	}
	defer func() { _, _ = io.Copy(io.Discard, response.Body) }() //nolint:errcheck // drain for keep-alive
}
