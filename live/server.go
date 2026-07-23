package live

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	corelive "github.com/larsartmann/auditlog-core/live"
	"github.com/larsartmann/go-output/daghtml"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
	viz "github.com/larsartmann/go-workflow-auditlog/viz"
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
	srv := &Server{
		hub:     hub,
		auditor: auditor,
	}

	coreCfg := corelive.Config{
		Addr:              cfg.Addr,
		Prefix:            "/",
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		HeartbeatInterval: cfg.HeartbeatInterval,
	}

	srv.core = corelive.New(hub.core, coreCfg,
		corelive.WithReportProvider(makeReportProvider(auditor)),
		corelive.WithSnapshotProvider(makeSnapshotProvider(auditor)),
		corelive.WithCompleteProvider(makeCompleteProvider(auditor)),
		corelive.WithDashboardProvider(renderDashboardHTML),
		corelive.WithHealthProvider(makeHealthProvider(auditor)),
	)

	return srv
}

// SignalComplete marks the workflow as finished.
func (srv *Server) SignalComplete() {
	srv.core.SignalComplete()
}

// OnEvent broadcasts an event to all connected SSE clients.
func (srv *Server) OnEvent(evt auditlog.Event) {
	srv.hub.OnEvent(evt)
}

// ClientCount returns the number of currently connected SSE clients.
func (srv *Server) ClientCount() int {
	return srv.core.ClientCount()
}

// ServeHTTP implements http.Handler, delegating to the core server.
func (srv *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	srv.core.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server.
func (srv *Server) ListenAndServe() error {
	return srv.core.ListenAndServe()
}

// Addr returns the server's listen address.
func (srv *Server) Addr() string {
	return srv.core.Addr()
}

// Shutdown gracefully shuts down the server.
func (srv *Server) Shutdown(ctx context.Context) error {
	return srv.core.Shutdown(ctx)
}

func makeReportProvider(auditor *auditlog.Auditor) corelive.ReportProvider {
	return func() ([]byte, error) {
		report := auditor.Report()

		var buf bytes.Buffer

		encoder := json.NewEncoder(&buf)
		encoder.SetIndent("", "  ")

		err := encoder.Encode(report)
		if err != nil {
			return nil, fmt.Errorf("encode report: %w", err)
		}

		return buf.Bytes(), nil
	}
}

func makeSnapshotProvider(auditor *auditlog.Auditor) corelive.SnapshotProvider {
	return func(isComplete bool) (json.RawMessage, error) {
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

func makeCompleteProvider(auditor *auditlog.Auditor) corelive.CompleteProvider {
	return func() (json.RawMessage, error) {
		report := auditor.Report()

		data := completeData{
			Report: report,
			DAG:    viz.BuildDAGHTML(report),
		}

		return json.Marshal(data)
	}
}

func makeHealthProvider(auditor *auditlog.Auditor) corelive.HealthProvider {
	return func() corelive.HealthInfo {
		return corelive.HealthInfo{
			Events:  auditor.EventsCount(),
			Dropped: auditor.DroppedEventCount(),
		}
	}
}
