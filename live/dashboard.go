package live

import (
	_ "embed"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	"github.com/larsartmann/go-output/daghtml"
	viz "github.com/larsartmann/go-workflow-auditlog/viz"
)

//go:embed dashboard.css
var liveCSS string

//go:embed dashboard.js
var liveJS string

// SchemaVersion mirrors the auditlog report version.
const SchemaVersion = "0.1.0"

// liveTemplate is the HTML skeleton for the live dashboard. The seven %s
// verbs receive:
// 1) viz base CSS, 2) live-specific CSS, 3) schema version (header),
// 4) type-metadata JSON, 5) dashboard JS, 6) daghtml graph JS,
// 7) schema version (footer).
//
// Unlike the static viz dashboard, no report data is embedded — all data
// arrives via SSE at runtime.
const liveTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none';">
<title>workflow-auditlog Live</title>
<style>
%s
%s</style>
</head>
<body>
<header>
  <div class="header-left">
    <h1><span class="logo-dot live-dot"></span>workflow-auditlog<span class="version">v%s</span> <span class="live-badge" id="live-badge"><span class="live-pulse"></span>LIVE</span></h1>
    <p class="subtitle">Workflow <span class="mono" id="workflow-id">&mdash;</span> &middot; Run <span class="mono" id="run-id">&mdash;</span> &mdash; <span id="connection-status" class="conn-status connecting">connecting...</span></p>
  </div>
  <div class="legend" id="legend"></div>
</header>
<div class="failure-banner" id="failure-banner" style="display:none">
  <div class="failure-banner-header">
    <span class="failure-banner-icon">&#9888;</span>
    <span class="failure-banner-title">Workflow Failed</span>
  </div>
  <div class="failure-banner-reason" id="failure-reason"></div>
  <div class="failure-banner-list" id="failure-list"></div>
</div>
<div class="waveform-section">
  <span class="waveform-label">Event Timeline</span>
  <div class="waveform" id="waveform">
    <span class="waveform-placeholder">Waiting for events...</span>
  </div>
  <div class="waveform-legend">
    <span class="wf-legend-item"><span class="wf-legend-dot" style="background:var(--success)"></span>attempt_start</span>
    <span class="wf-legend-item"><span class="wf-legend-dot" style="background:var(--warning)"></span>attempt_end</span>
    <span class="wf-legend-item"><span class="wf-legend-dot" style="background:var(--error)"></span>error</span>
  </div>
</div>
<div class="stats" id="stats">
  <div class="stat-placeholder">Connect to see live stats...</div>
</div>
<div class="tab-bar" role="tablist" aria-label="Report sections">
  <button class="tab active" data-tab="steps" role="tab" aria-selected="true" aria-controls="tab-steps" id="tab-btn-steps">Steps</button>
  <button class="tab" data-tab="graph" role="tab" aria-selected="false" aria-controls="tab-graph" id="tab-btn-graph">DAG Graph</button>
  <button class="tab" data-tab="timeline" role="tab" aria-selected="false" aria-controls="tab-timeline" id="tab-btn-timeline">Timeline</button>
  <button class="tab" data-tab="events" role="tab" aria-selected="false" aria-controls="tab-events" id="tab-btn-events">Events</button>
</div>
<div class="tab-content active" id="tab-steps" role="tabpanel" aria-labelledby="tab-btn-steps">
  <div class="filter-bar">
    <label for="step-search" class="sr-only">Filter steps by name</label>
    <input type="text" id="step-search" placeholder="Filter steps..." aria-label="Filter steps by name">
    <button class="chip" id="step-errors-only" aria-pressed="false" title="Show only steps with errors">Errors only</button>
    <span id="step-result-count" style="font-size:0.75rem;color:var(--text-dim);font-family:var(--font-mono)"></span>
  </div>
  <div class="table-wrap">
    <table>
      <thead>
        <tr>
          <th class="sortable" data-sort="name">Step</th>
          <th class="sortable" data-sort="type">Type</th>
          <th class="sortable" data-sort="status">Status</th>
          <th class="sortable" data-sort="attempts">Attempts</th>
          <th class="sortable" data-sort="duration">Duration</th>
          <th>Dependencies</th>
          <th>Dependents</th>
          <th>Config</th>
          <th>Error</th>
        </tr>
      </thead>
      <tbody id="steps-tbody"></tbody>
      <tbody id="steps-empty" class="empty-state" style="display:none"><tr><td colspan="9">No steps recorded yet.</td></tr></tbody>
    </table>
  </div>
</div>
<div class="tab-content" id="tab-graph" role="tabpanel" aria-labelledby="tab-btn-graph">
  <div class="filter-bar">
    <label for="graph-search" class="sr-only">Filter graph nodes</label>
    <input type="text" id="graph-search" placeholder="Highlight nodes..." aria-label="Filter graph nodes by name">
    <button class="chip" id="graph-critical-path" aria-pressed="false" title="Highlight the longest dependency chain (bottleneck)">Critical Path</button>
    <span id="graph-info-text" style="font-size:0.75rem;color:var(--text-dim);font-family:var(--font-mono)"></span>
  </div>
  <div id="graph-container">
    <div class="graph-controls">
      <button class="graph-zoom-in" title="Zoom in" aria-label="Zoom in">+</button>
      <button class="graph-zoom-out" title="Zoom out" aria-label="Zoom out">&minus;</button>
      <button class="graph-fit" title="Fit to view" aria-label="Fit to view">&#8982;</button>
    </div>
    <div class="graph-info">Scroll/pinch to zoom &middot; Drag to pan &middot; Click node to highlight</div>
    <div class="graph-placeholder" id="graph-placeholder">DAG will appear here as steps execute...</div>
  </div>
</div>
<div class="tab-content" id="tab-timeline" role="tabpanel" aria-labelledby="tab-btn-timeline">
  <div id="timeline-container">
    <div class="graph-placeholder">Timeline will appear here as events arrive...</div>
  </div>
</div>
<div class="tab-content" id="tab-events" role="tabpanel" aria-labelledby="tab-btn-events">
  <div class="filter-bar" id="event-filters" role="group" aria-label="Filter events by type"></div>
  <div class="table-wrap">
    <table>
      <thead>
        <tr><th scope="col">#</th><th scope="col">Time</th><th scope="col">Type</th><th scope="col">Phase</th><th scope="col">Step</th><th scope="col">Attempt</th><th scope="col">Duration</th><th scope="col">Error</th></tr>
      </thead>
      <tbody id="events-tbody"></tbody>
      <tbody id="events-empty" class="empty-state" style="display:none"><tr><td colspan="8">No events recorded yet.</td></tr></tbody>
    </table>
  </div>
</div>
<div id="error-tooltip" class="tooltip"></div>
<script type="application/json" id="type-metadata">%s</script>
<script>
%s</script>
<script>
%s</script>
<div class="footer">
  <span>Generated by <strong>workflow-auditlog live</strong> &middot; <span id="footer-ts"></span></span>
  <span id="footer-stats">Schema v%s</span>
</div>
</body>
</html>`

// renderDashboardHTML builds the static HTML dashboard string. This is called
// once at server startup (not per-request) since all dynamic data flows via SSE.
func renderDashboardHTML() string {
	metadata := viz.BuildTypeMetadata()

	metadataJSON, err := json.Marshal(
		metadata,
		json.Deterministic(true),
		jsontext.EscapeForHTML(true),
		jsontext.EscapeForJS(true),
	)
	if err != nil {
		return "<html><body>failed to render dashboard</body></html>"
	}

	return fmt.Sprintf(
		liveTemplate,
		viz.DashboardCSS(),
		liveCSS,
		SchemaVersion,
		metadataJSON,
		liveJS,
		daghtml.Script(),
		SchemaVersion,
	)
}
