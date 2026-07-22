package viz

import (
	_ "embed"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	"github.com/larsartmann/go-output/daghtml"
)

//go:embed dashboard.css
var dashboardCSS string

//go:embed dashboard.js
var dashboardJS string

// htmlTemplate is the static HTML skeleton. The eight %s verbs receive:
// 1) dashboardCSS, 2) schema version (header), 3) report JSON,
// 4) type-metadata JSON, 5) DAG JSON data, 6) dashboardJS,
// 7) daghtml graph JS, 8) schema version (footer).
//
// Report data is injected via <script type="application/json"> tags (never
// parsed as HTML). Dynamic content in the JS is escaped via the esc()
// function defined in dashboard.js. A strict Content-Security-Policy meta
// tag blocks all external resources.
const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none';">
<title>workflow-auditlog Report</title>
<style>
%s</style>
</head>
<body>
<header>
  <div class="header-left">
    <h1><span class="logo-dot"></span>workflow-auditlog<span class="version">v%s</span> <span class="workflow-status" id="workflow-status"></span></h1>
    <p class="subtitle">Workflow <span class="mono" id="workflow-id"></span> &middot; Run <span class="mono" id="run-id"></span> &mdash; exported <span id="exported-at"></span></p>
  </div>
  <div class="legend" id="legend"></div>
</header>
<div class="failure-banner" id="failure-banner">
  <div class="failure-banner-header">
    <span class="failure-banner-icon">&#9888;</span>
    <span class="failure-banner-title">Workflow Failed</span>
  </div>
  <div class="failure-banner-reason" id="failure-reason"></div>
  <div class="failure-banner-list" id="failure-list"></div>
</div>
<div class="waveform-section">
  <span class="waveform-label">Event Timeline</span>
  <div class="waveform" id="waveform"></div>
  <div class="waveform-legend">
    <span class="wf-legend-item"><span class="wf-legend-dot" style="background:var(--success)"></span>attempt_start</span>
    <span class="wf-legend-item"><span class="wf-legend-dot" style="background:var(--warning)"></span>attempt_end</span>
    <span class="wf-legend-item"><span class="wf-legend-dot" style="background:var(--error)"></span>error</span>
  </div>
</div>
<div class="stats" id="stats"></div>
<div class="tab-bar" role="tablist" aria-label="Report sections">
  <button class="tab active" data-tab="steps" role="tab" aria-selected="true" aria-controls="tab-steps" id="tab-btn-steps">Steps</button>
  <button class="tab" data-tab="tree" role="tab" aria-selected="false" aria-controls="tab-tree" id="tab-btn-tree">DAG Tree</button>
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
      <tbody id="steps-empty" class="empty-state" style="display:none"><tr><td colspan="9">No steps recorded.</td></tr></tbody>
    </table>
  </div>
</div>
<div class="tab-content" id="tab-tree" role="tabpanel" aria-labelledby="tab-btn-tree">
  <div class="scope-tree" id="tree-container"></div>
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
  </div>
</div>
<div class="tab-content" id="tab-timeline" role="tabpanel" aria-labelledby="tab-btn-timeline">
  <div id="timeline-container"></div>
</div>
<div class="tab-content" id="tab-events" role="tabpanel" aria-labelledby="tab-btn-events">
  <div class="filter-bar" id="event-filters" role="group" aria-label="Filter events by type"></div>
  <div class="table-wrap">
    <table>
      <thead>
        <tr><th scope="col">#</th><th scope="col">Time</th><th scope="col">Type</th><th scope="col">Phase</th><th scope="col">Step</th><th scope="col">Attempt</th><th scope="col">Duration</th><th scope="col">Error</th></tr>
      </thead>
      <tbody id="events-tbody"></tbody>
      <tbody id="events-empty" class="empty-state" style="display:none"><tr><td colspan="8">No events recorded.</td></tr></tbody>
    </table>
  </div>
</div>
<div id="error-tooltip" class="tooltip"></div>
<script type="application/json" id="report-data">%s</script>
<script type="application/json" id="type-metadata">%s</script>
<script type="application/json" id="dag-data">%s</script>
<script>
%s</script>
<script>
%s</script>
<div class="footer">
  <span>Generated by <strong>workflow-auditlog</strong> &middot; <span id="footer-ts"></span></span>
  <span id="footer-stats">Schema v%s</span>
</div>
</body>
</html>`

// renderHTML builds a complete, self-contained HTML dashboard string from the
// workflow report. The output embeds all CSS (dashboard.css) and JavaScript
// (dashboard.js + daghtml graph JS) inline — no external dependencies.
func renderHTML(report WorkflowReport) (string, error) {
	reportJSON, err := json.Marshal(report,
		json.Deterministic(true),
		jsontext.EscapeForHTML(true),
		jsontext.EscapeForJS(true),
	)
	if err != nil {
		return "", fmt.Errorf("%w: marshal report: %w", ErrRenderFailed, err)
	}

	metadataJSON, err := json.Marshal(BuildTypeMetadata(),
		json.Deterministic(true),
		jsontext.EscapeForHTML(true),
		jsontext.EscapeForJS(true),
	)
	if err != nil {
		return "", fmt.Errorf("%w: marshal metadata: %w", ErrRenderFailed, err)
	}

	dag := buildDAGHTML(report)

	dagJSON, err := json.Marshal(dag,
		json.Deterministic(true),
		jsontext.EscapeForHTML(true),
		jsontext.EscapeForJS(true),
	)
	if err != nil {
		return "", fmt.Errorf("%w: marshal DAG: %w", ErrRenderFailed, err)
	}

	return fmt.Sprintf(
		htmlTemplate,
		dashboardCSS, report.Version, reportJSON, metadataJSON,
		dagJSON, dashboardJS, daghtml.Script(), report.Version,
	), nil
}
