package live

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/larsartmann/go-output/daghtml"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	viz "github.com/larsartmann/go-workflow-auditlog/viz"
	corelive "github.com/larsartmann/auditlog-core/live"
)

// ErrServerAlreadyRunning is returned when ListenAndServe is called on a
// server that is already serving.
var ErrServerAlreadyRunning = corelive.ErrServerAlreadyRunning

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
	core    *corelive.Server
	hub     *Hub
	auditor *auditlog.Auditor
}

// New is the convenience constructor. It creates a Hub, wires it as the
// auditlog OnEvent callback, creates the Auditor, and returns a ready-to-use
// Server.
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

// NewServer creates a Server from an existing Hub and Auditor.
func NewServer(hub *Hub, auditor *auditlog.Auditor, cfg Config) *Server {
	s := &Server{
		hub:     hub,
		auditor: auditor,
	}

	reportProvider := func() ([]byte, error) {
		report := auditor.Report()

		var buf bytes.Buffer

		encoder := json.NewEncoder(&buf)
		encoder.SetIndent("", "  ")

		if err := encoder.Encode(report); err != nil {
			return nil, fmt.Errorf("encode report: %w", err)
		}

		return buf.Bytes(), nil
	}

	snapshotProvider := func(isComplete bool) (json.RawMessage, error) {
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

	completeProvider := func() (json.RawMessage, error) {
		report := auditor.Report()

		data := completeData{
			Report: report,
			DAG:    viz.BuildDAGHTML(report),
		}

		return json.Marshal(data)
	}

	healthProvider := func() corelive.HealthInfo {
		return corelive.HealthInfo{
			Events:  auditor.EventsCount(),
			Dropped: auditor.DroppedEventCount(),
		}
	}

	coreCfg := corelive.Config{
		Addr:              cfg.Addr,
		Prefix:            "/",
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		HeartbeatInterval: cfg.HeartbeatInterval,
	}

	s.core = corelive.New(hub.core, coreCfg,
		corelive.WithReportProvider(reportProvider),
		corelive.WithSnapshotProvider(snapshotProvider),
		corelive.WithCompleteProvider(completeProvider),
		corelive.WithDashboardProvider(renderDashboardHTML),
		corelive.WithHealthProvider(healthProvider),
	)

	return s
}

// SignalComplete marks the workflow as finished.
func (s *Server) SignalComplete() {
	s.core.SignalComplete()
}

// OnEvent broadcasts an event to all connected SSE clients.
func (s *Server) OnEvent(evt auditlog.Event) {
	s.hub.OnEvent(evt)
}

// ClientCount returns the number of currently connected SSE clients.
func (s *Server) ClientCount() int {
	return s.core.ClientCount()
}

// ServeHTTP implements http.Handler, delegating to the core server.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.core.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	return s.core.ListenAndServe()
}

// Addr returns the server's listen address.
func (s *Server) Addr() string {
	return s.core.Addr()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.core.Shutdown(ctx)
}
