package live

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/larsartmann/go-output/daghtml"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	viz "github.com/larsartmann/go-workflow-auditlog/viz"
)

const (
	defaultReadHeaderTimeout = 5 * time.Second
	defaultHeartbeatInterval = 15 * time.Second
	defaultAddr              = ":0"
)

// ErrServerAlreadyRunning is returned when ListenAndServe is called on a
// server that is already serving.
var ErrServerAlreadyRunning = errors.New("live server is already running")

// Config controls the live dashboard server behaviour.
type Config struct {
	// Addr is the TCP address to listen on. Default ":0" (random port).
	// Use ":8080" for a fixed port.
	Addr string
	// ReadHeaderTimeout is the maximum duration for reading the request
	// headers. Default 5 seconds. Set to 0 to disable.
	ReadHeaderTimeout time.Duration
	// HeartbeatInterval is how often to send SSE keepalive comments.
	// Default 15 seconds. Set to 0 to disable heartbeats.
	HeartbeatInterval time.Duration
}

// Server serves the real-time workflow dashboard over HTTP.
// Create one with [New] or [NewServer].
type Server struct {
	hub     *Hub
	auditor *auditlog.Auditor
	config  Config

	serverMu   sync.Mutex
	httpServer *http.Server
	mux        *http.ServeMux

	dashboardHTML string
	startTime     time.Time
}

// New is the convenience constructor. It creates a Hub, wires it as the
// auditlog OnEvent callback, creates the Auditor, and returns a ready-to-use
// Server. This is the recommended way to use the live package for the common
// case where the hub is the only OnEvent consumer.
//
// For advanced cases (e.g. chaining NDJSON streaming alongside the live
// server), use [NewHub] + [NewServer] and wire callbacks manually:
//
//	hub := live.NewHub(nil) // auditor set later by NewServer
//	auditor, _ := auditlog.New(auditlog.Config{
//	    Enabled: true,
//	    OnEvent: func(evt auditlog.Event) {
//	        hub.OnEvent(evt)
//	        streamer.OnEvent(evt)
//	    },
//	})
//	server := live.NewServer(hub, auditor, live.Config{Addr: ":8080"})
func New(auditCfg auditlog.Config, serverCfg Config) (*Server, *auditlog.Auditor, error) {
	hub := NewHub(nil)

	auditCfg.OnEvent = hub.OnEvent
	auditCfg.Enabled = true

	auditor, err := auditlog.New(auditCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("create auditor: %w", err)
	}

	hub.SetAuditor(auditor)

	server := NewServer(hub, auditor, serverCfg)

	return server, auditor, nil
}

// NewServer creates a Server from an existing Hub and Auditor. Use this when
// you need to chain OnEvent callbacks (e.g. live dashboard + NDJSON streaming).
//
// See [New] for the simpler convenience constructor.
func NewServer(hub *Hub, auditor *auditlog.Auditor, cfg Config) *Server {
	if cfg.Addr == "" {
		cfg.Addr = defaultAddr
	}

	if cfg.ReadHeaderTimeout == 0 {
		cfg.ReadHeaderTimeout = defaultReadHeaderTimeout
	}

	if cfg.HeartbeatInterval == 0 {
		cfg.HeartbeatInterval = defaultHeartbeatInterval
	}

	s := &Server{
		hub:     hub,
		auditor: auditor,
		config:  cfg,
		mux:     http.NewServeMux(),
	}

	s.dashboardHTML = renderDashboardHTML()
	s.setupRoutes()

	return s
}

func (s *Server) setupRoutes() {
	s.mux.HandleFunc("/", s.handleDashboard)
	s.mux.HandleFunc("/api/report", s.handleReport)
	s.mux.HandleFunc("/api/events", s.handleSSE)
	s.mux.HandleFunc("/api/health", s.handleHealth)
}

// ListenAndServe starts the HTTP server. It blocks until Shutdown is called
// or the server encounters a fatal error.
func (s *Server) ListenAndServe() error {
	s.serverMu.Lock()

	if s.httpServer != nil {
		s.serverMu.Unlock()

		return ErrServerAlreadyRunning
	}

	s.startTime = time.Now()

	s.httpServer = &http.Server{
		Addr:              s.config.Addr,
		Handler:           s.mux,
		ReadHeaderTimeout: s.config.ReadHeaderTimeout,
	}

	s.serverMu.Unlock()

	err := s.httpServer.ListenAndServe()

	return fmt.Errorf("listen and serve: %w", err)
}

// Addr returns the server's listen address. If the server was started with
// ":0", this returns the actual address after ListenAndServe binds. Call
// after ListenAndServe (e.g. from a goroutine that checks s.httpServer).
func (s *Server) Addr() string {
	s.serverMu.Lock()
	defer s.serverMu.Unlock()

	if s.httpServer == nil {
		return s.config.Addr
	}

	return s.httpServer.Addr
}

// Shutdown gracefully shuts down the server, waiting for in-flight requests
// to complete (up to the context deadline).
func (s *Server) Shutdown(ctx context.Context) error {
	s.serverMu.Lock()
	server := s.httpServer
	s.serverMu.Unlock()

	if server == nil {
		return nil
	}

	err := server.Shutdown(ctx)

	return fmt.Errorf("shutdown: %w", err)
}

// SignalComplete marks the workflow as finished. All connected SSE clients
// receive the final report (including full DAG structure from Snapshot).
//
// Typical usage:
//
//	w.Do(ctx)
//	auditor.Snapshot(w)
//	server.SignalComplete()
func (s *Server) SignalComplete() {
	s.hub.SignalComplete()
}

// OnEvent broadcasts an event to all connected SSE clients. This method is
// safe to pass directly as auditlog.Config.OnEvent.
func (s *Server) OnEvent(evt auditlog.Event) {
	s.hub.OnEvent(evt)
}

// ClientCount returns the number of currently connected SSE clients.
func (s *Server) ClientCount() int {
	return s.hub.ClientCount()
}

// ServeHTTP implements http.Handler, delegating to the internal mux.
// This allows the Server to be used with httptest.NewServer.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// --- HTTP Handlers ---

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	_, _ = w.Write([]byte(s.dashboardHTML))
}

func (s *Server) handleReport(w http.ResponseWriter, _ *http.Request) {
	report := s.auditor.Report()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")

	encoder := jsontext.NewEncoder(w)

	err := json.MarshalEncode(encoder, report)
	if err != nil {
		http.Error(w, "encode report", http.StatusInternalServerError)

		return
	}
}

// healthResponse is the JSON payload returned by the health endpoint.
type healthResponse struct {
	Status   string  `json:"status"`
	UptimeS  float64 `json:"uptime_s"`
	Clients  int     `json:"clients"`
	Events   int     `json:"events"`
	Complete bool    `json:"complete"`
	Dropped  int64   `json:"dropped"`
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	resp := healthResponse{
		Status:   "ok",
		UptimeS:  time.Since(s.startTime).Seconds(),
		Clients:  s.hub.ClientCount(),
		Events:   s.auditor.EventsCount(),
		Complete: s.hub.IsComplete(),
		Dropped:  s.auditor.DroppedEventCount(),
	}

	_ = json.MarshalEncode(jsontext.NewEncoder(w), resp)
}

// snapshotData is the payload sent as the initial SSE event.
type snapshotData struct {
	Report   auditlog.WorkflowReport `json:"report"`
	Events   []auditlog.Event        `json:"events"`
	Metadata viz.TypeMetadata        `json:"metadata"`
	DAG      daghtml.DAG             `json:"dag"`
	Complete bool                    `json:"complete"`
}

// completeData is the payload sent when the workflow finishes.
type completeData struct {
	Report auditlog.WorkflowReport `json:"report"`
	DAG    daghtml.DAG             `json:"dag"`
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sub := s.hub.Subscribe()
	defer s.hub.Unsubscribe(sub.id)

	err := s.sendSnapshot(w, flusher)
	if err != nil {
		return
	}

	heartbeat := time.NewTicker(s.config.HeartbeatInterval)
	defer heartbeat.Stop()

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return

		case <-sub.done:
			s.sendComplete(w, flusher)

			return

		case evt := <-sub.ch:
			err = writeSSE(w, "event", evt)
			if err != nil {
				return
			}

			flusher.Flush()

		case <-heartbeat.C:
			_, err = w.Write([]byte(": heartbeat\n\n"))
			if err != nil {
				return
			}

			flusher.Flush()
		}
	}
}

func (s *Server) sendSnapshot(w http.ResponseWriter, flusher http.Flusher) error {
	report := s.auditor.Report()
	events := s.auditor.Events()

	data := snapshotData{
		Report:   report,
		Events:   events,
		Metadata: viz.BuildTypeMetadata(),
		DAG:      viz.BuildDAGHTML(report),
		Complete: s.hub.IsComplete(),
	}

	err := writeSSE(w, "snapshot", data)
	if err != nil {
		return err
	}

	flusher.Flush()

	return nil
}

func (s *Server) sendComplete(w http.ResponseWriter, flusher http.Flusher) {
	report := s.auditor.Report()

	data := completeData{
		Report: report,
		DAG:    viz.BuildDAGHTML(report),
	}

	_ = writeSSE(w, "complete", data)

	flusher.Flush()
}

// writeSSE writes a named SSE event with JSON-encoded data.
func writeSSE(w http.ResponseWriter, eventName string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal SSE data: %w", err)
	}

	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, payload)
	if err != nil {
		return fmt.Errorf("write SSE: %w", err)
	}

	return nil
}
