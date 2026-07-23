package live

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/larsartmann/go-output/daghtml"
	"github.com/larsartmann/go-sse"
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
	Addr string
	// ReadHeaderTimeout is the maximum duration for reading the request
	// headers. Default 5 seconds. Set to 0 to disable.
	ReadHeaderTimeout time.Duration
	// HeartbeatInterval is how often to send SSE keepalive comments.
	// Default 15 seconds. Set to 0 to disable heartbeats.
	HeartbeatInterval time.Duration
}

// ReportProvider returns the current report as JSON bytes.
type ReportProvider func() ([]byte, error)

// SnapshotProvider returns the initial SSE snapshot payload as raw JSON.
type SnapshotProvider func(isComplete bool) (jsontext.Value, error)

// CompleteProvider returns the final SSE complete payload as raw JSON.
type CompleteProvider func() (jsontext.Value, error)

// DashboardProvider returns the full HTML string for the dashboard page.
type DashboardProvider func() string

// HealthInfo provides dynamic health check data beyond the built-in
// uptime, client count, and completion status.
type HealthInfo struct {
	Events  int   `json:"events"`
	Dropped int64 `json:"dropped"`
}

// HealthProvider returns additional health check information.
type HealthProvider func() HealthInfo

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

// Server serves the real-time workflow dashboard over HTTP.
type Server struct {
	hub    *Hub
	config Config

	serverMu   sync.Mutex
	httpServer *http.Server
	mux        *http.ServeMux

	reportProvider    ReportProvider
	snapshotProvider  SnapshotProvider
	completeProvider  CompleteProvider
	dashboardProvider DashboardProvider
	healthProvider    HealthProvider

	dashboardHTML string
	startTime     time.Time
}

// New is the convenience constructor. It creates a Hub, wires it as the
// auditlog OnEvent callback, creates the Auditor, and returns a ready-to-use
// Server.
func New(auditCfg auditlog.Config, serverCfg Config) (*Server, *auditlog.Auditor, error) {
	hub := NewHub()

	auditCfg.OnEvent = hub.OnEvent
	auditCfg.Enabled = true

	auditor, err := auditlog.New(auditCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("create auditor: %w", err)
	}

	server := NewServer(hub, auditor, serverCfg)

	return server, auditor, nil
}

// NewServer creates a Server from an existing Hub and Auditor.
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

	srv := &Server{ //nolint:exhaustruct
		hub:    hub,
		config: cfg,
		mux:    http.NewServeMux(),
	}

	srv.reportProvider = makeReportProvider(auditor)
	srv.snapshotProvider = makeSnapshotProvider(auditor)
	srv.completeProvider = makeCompleteProvider(auditor)
	srv.dashboardProvider = renderDashboardHTML
	srv.healthProvider = makeHealthProvider(auditor)
	srv.dashboardHTML = renderDashboardHTML()

	srv.setupRoutes()

	return srv
}

func (srv *Server) setupRoutes() {
	srv.mux.HandleFunc("/", srv.handleDashboard)
	srv.mux.HandleFunc("/api/report", srv.handleReport)
	srv.mux.HandleFunc("/api/events", srv.handleSSE)
	srv.mux.HandleFunc("/api/health", srv.handleHealth)
}

// ListenAndServe starts the HTTP server.
func (srv *Server) ListenAndServe() error {
	srv.serverMu.Lock()

	if srv.httpServer != nil {
		srv.serverMu.Unlock()

		return ErrServerAlreadyRunning
	}

	srv.startTime = time.Now()

	srv.httpServer = &http.Server{ //nolint:exhaustruct // minimal config
		Addr:              srv.config.Addr,
		Handler:           srv.mux,
		ReadHeaderTimeout: srv.config.ReadHeaderTimeout,
	}

	srv.serverMu.Unlock()

	return fmt.Errorf("listen and serve: %w", srv.httpServer.ListenAndServe())
}

// Addr returns the server's listen address.
func (srv *Server) Addr() string {
	srv.serverMu.Lock()
	defer srv.serverMu.Unlock()

	if srv.httpServer == nil {
		return srv.config.Addr
	}

	return srv.httpServer.Addr
}

// Shutdown gracefully shuts down the server.
func (srv *Server) Shutdown(ctx context.Context) error {
	srv.serverMu.Lock()
	server := srv.httpServer
	srv.serverMu.Unlock()

	if server == nil {
		return nil
	}

	return fmt.Errorf("shutdown: %w", server.Shutdown(ctx))
}

// SignalComplete marks the workflow as finished.
func (srv *Server) SignalComplete() {
	srv.hub.SignalComplete()
}

// OnEvent broadcasts an event to all connected SSE clients.
func (srv *Server) OnEvent(evt auditlog.Event) {
	srv.hub.OnEvent(evt)
}

// ClientCount returns the number of currently connected SSE clients.
func (srv *Server) ClientCount() int {
	return srv.hub.ClientCount()
}

// ServeHTTP implements http.Handler.
func (srv *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	srv.mux.ServeHTTP(w, r)
}

// --- HTTP Handlers ---

func (srv *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(srv.dashboardHTML))
}

func (srv *Server) handleReport(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")

	if srv.reportProvider == nil {
		http.Error(w, "report provider not configured", http.StatusInternalServerError)

		return
	}

	data, err := srv.reportProvider()
	if err != nil {
		http.Error(w, fmt.Sprintf("generate report: %v", err), http.StatusInternalServerError)

		return
	}

	_, _ = w.Write(data)
}

type healthResponse struct {
	Status   string  `json:"status"`
	UptimeS  float64 `json:"uptime_s"`
	Clients  int     `json:"clients"`
	Events   int     `json:"events"`
	Complete bool    `json:"complete"`
	Dropped  int64   `json:"dropped"`
}

func (srv *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	resp := healthResponse{
		Status:   "ok",
		UptimeS:  time.Since(srv.startTime).Seconds(),
		Clients:  srv.hub.ClientCount(),
		Complete: srv.hub.IsComplete(),
	}

	if srv.healthProvider != nil {
		info := srv.healthProvider()
		resp.Events = info.Events
		resp.Dropped = info.Dropped
	}

	payload, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "marshal health response", http.StatusInternalServerError)

		return
	}

	_, _ = w.Write([]byte(payload))
}

func (srv *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", sse.ContentType)
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sub := srv.hub.Subscribe()
	defer srv.hub.Unsubscribe(sub.id)

	if err := srv.sendSnapshot(w, flusher); err != nil {
		return
	}

	heartbeat := time.NewTicker(srv.config.HeartbeatInterval)
	defer heartbeat.Stop()

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return

		case <-sub.done:
			srv.sendComplete(w, flusher)

			return

		case evt := <-sub.ch:
			if err := sse.WriteEvent(w, sse.Event{Event: "event", Data: string(evt)}); err != nil {
				return
			}

			flusher.Flush()

		case <-heartbeat.C:
			if _, err := w.Write([]byte(": heartbeat\n\n")); err != nil {
				return
			}

			flusher.Flush()
		}
	}
}

func (srv *Server) sendSnapshot(w http.ResponseWriter, flusher http.Flusher) error {
	if srv.snapshotProvider == nil {
		return nil
	}

	data, err := srv.snapshotProvider(srv.hub.IsComplete())
	if err != nil {
		return fmt.Errorf("build snapshot: %w", err)
	}

	if err := sse.WriteEvent(w, sse.Event{Event: "snapshot", Data: string(data)}); err != nil {
		return err
	}

	flusher.Flush()

	return nil
}

func (srv *Server) sendComplete(w http.ResponseWriter, flusher http.Flusher) {
	if srv.completeProvider == nil {
		return
	}

	data, err := srv.completeProvider()
	if err != nil {
		return
	}

	_ = sse.WriteEvent(w, sse.Event{Event: "complete", Data: string(data)})

	flusher.Flush()
}

// --- Provider Factories ---

func makeReportProvider(auditor *auditlog.Auditor) ReportProvider {
	return func() ([]byte, error) {
		report := auditor.Report()

		var buf bytes.Buffer

		encoder := jsontext.NewEncoder(&buf, jsontext.WithIndent("  "))

		err := json.MarshalEncode(encoder, report)
		if err != nil {
			return nil, fmt.Errorf("encode report: %w", err)
		}

		return buf.Bytes(), nil
	}
}

func makeSnapshotProvider(auditor *auditlog.Auditor) SnapshotProvider {
	return func(isComplete bool) (jsontext.Value, error) {
		report := auditor.Report()
		events := auditor.Events()

		data := snapshotData{
			Report:   report,
			Events:   events,
			Metadata: viz.BuildTypeMetadata(),
			DAG:      viz.BuildDAGHTML(report),
			Complete: isComplete,
		}

		return json.Marshal(data)
	}
}

func makeCompleteProvider(auditor *auditlog.Auditor) CompleteProvider {
	return func() (jsontext.Value, error) {
		report := auditor.Report()

		data := completeData{
			Report: report,
			DAG:    viz.BuildDAGHTML(report),
		}

		return json.Marshal(data)
	}
}

func makeHealthProvider(auditor *auditlog.Auditor) HealthProvider {
	return func() HealthInfo {
		return HealthInfo{
			Events:  auditor.EventsCount(),
			Dropped: auditor.DroppedEventCount(),
		}
	}
}
