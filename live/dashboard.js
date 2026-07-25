/* live/dashboard.js — SSE client + incremental rendering engine */

(function () {
  "use strict";

  // === Utilities ===

  function esc(s) {
    if (s == null) return "";
    return String(s).replace(/[&<>"']/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
    });
  }

  function humanizeDuration(ms) {
    if (ms == null || ms < 0) return "\u2014";
    if (ms < 1) return ms.toFixed(3) + "ms";
    if (ms < 1000) return ms.toFixed(1) + "ms";
    var s = ms / 1000;
    if (s < 60) return s.toFixed(1) + "s";
    var m = Math.floor(s / 60);
    var rem = s - m * 60;
    if (m < 60) return m + "m " + Math.round(rem) + "s";
    var h = Math.floor(m / 60);
    var remM = m - h * 60;
    return h + "h " + remM + "m";
  }

  // humanizeMs formats durations for compact graph node labels.
  // Mirrors the Go humanizeMs() in viz/daghtml_adapter.go.
  function humanizeMs(ms) {
    if (ms == null || ms < 0) return "";
    if (ms < 1) return "<1ms";
    if (ms < 1000) return Math.round(ms) + "ms";
    return (ms / 1000).toFixed(1) + "s";
  }

  // === Type metadata ===

  var meta = {};
  try {
    meta = JSON.parse(document.getElementById("type-metadata").textContent);
  } catch (e) {
    meta = { statuses: {}, events: {} };
  }

  var statusIcons = {};
  var statusLabels = {};
  var statusColors = {};
  if (meta.statuses) {
    Object.keys(meta.statuses).forEach(function (k) {
      statusIcons[k] = meta.statuses[k].icon;
      statusLabels[k] = meta.statuses[k].label;
    });
  }
  var eventLabels = {};
  var eventColors = {};
  if (meta.events) {
    Object.keys(meta.events).forEach(function (k) {
      eventLabels[k] = meta.events[k].label;
      eventColors[k] = meta.events[k].color;
    });
  }

  // === State ===

  var state = {
    report: null,
    events: [],
    steps: {},
    dag: null,
    complete: false,
    maxEventSeq: 0,
    newStepNames: {},
    changedSteps: {},
  };

  // === DOM refs ===

  var els = {
    workflowId: document.getElementById("workflow-id"),
    runId: document.getElementById("run-id"),
    connStatus: document.getElementById("connection-status"),
    liveBadge: document.getElementById("live-badge"),
    legend: document.getElementById("legend"),
    stats: document.getElementById("stats"),
    waveform: document.getElementById("waveform"),
    failureBanner: document.getElementById("failure-banner"),
    failureReason: document.getElementById("failure-reason"),
    failureList: document.getElementById("failure-list"),
    stepsTbody: document.getElementById("steps-tbody"),
    stepsEmpty: document.getElementById("steps-empty"),
    stepSearch: document.getElementById("step-search"),
    stepErrorsOnly: document.getElementById("step-errors-only"),
    stepResultCount: document.getElementById("step-result-count"),
    eventsTbody: document.getElementById("events-tbody"),
    eventsEmpty: document.getElementById("events-empty"),
    eventFilters: document.getElementById("event-filters"),
    graphContainer: document.getElementById("graph-container"),
    graphPlaceholder: document.getElementById("graph-placeholder"),
    timelineContainer: document.getElementById("timeline-container"),
    footerTs: document.getElementById("footer-ts"),
    footerStats: document.getElementById("footer-stats"),
  };

  // === Connection status ===

  function setConnStatus(cls, text) {
    els.connStatus.className = "conn-status " + cls;
    els.connStatus.textContent = text;
  }

  // === Connection (SSE with WebSocket fallback) ===

  var eventSource = null;
  var wsConnection = null;
  var reconnectDelay = 1000;
  var maxReconnectDelay = 10000;
  var sseFailCount = 0;

  function connect() {
    setConnStatus("connecting", "connecting...");

    // After 2 SSE failures, fall back to WebSocket (corporate proxies, etc.)
    if (sseFailCount >= 2) {
      connectWebSocket();
      return;
    }

    connectSSE();
  }

  function connectSSE() {
    eventSource = new EventSource((window.ROUTE_PREFIX || "") + "/api/events");

    eventSource.addEventListener("snapshot", function (e) {
      setConnStatus("connected", "connected");
      reconnectDelay = 1000;
      sseFailCount = 0;
      handleSnapshot(JSON.parse(e.data));
    });

    eventSource.addEventListener("event", function (e) {
      handleEvent(JSON.parse(e.data));
    });

    eventSource.addEventListener("complete", function (e) {
      handleComplete(JSON.parse(e.data));
    });

    eventSource.onopen = function () {
      setConnStatus("connected", "connected");
      reconnectDelay = 1000;
      sseFailCount = 0;
    };

    eventSource.onerror = function () {
      if (state.complete) {
        setConnStatus("disconnected", "disconnected");
        if (eventSource) eventSource.close();
        return;
      }

      sseFailCount++;

      if (sseFailCount >= 2) {
        if (eventSource) eventSource.close();
        connectWebSocket();
        return;
      }

      setConnStatus("reconnecting", "reconnecting...");
      if (eventSource) eventSource.close();

      setTimeout(function () {
        reconnectDelay = Math.min(reconnectDelay * 1.5, maxReconnectDelay);
        connectSSE();
      }, reconnectDelay);
    };
  }

  function connectWebSocket() {
    setConnStatus("connecting", "connecting (ws)...");

    var pfx = window.ROUTE_PREFIX || "";
    var wsScheme = location.protocol === "https:" ? "wss:" : "ws:";
    var wsUrl = wsScheme + "//" + location.host + pfx + "/api/ws";

    try {
      wsConnection = new WebSocket(wsUrl);
    } catch (e) {
      setConnStatus("disconnected", "ws unavailable");
      return;
    }

    wsConnection.onopen = function () {
      setConnStatus("connected", "connected (ws)");
      reconnectDelay = 1000;
    };

    wsConnection.onmessage = function (e) {
      var msg = JSON.parse(e.data);
      switch (msg.type) {
        case "snapshot":
          handleSnapshot(msg.data);
          break;
        case "event":
          handleEvent(msg.data);
          break;
        case "complete":
          handleComplete(msg.data);
          break;
      }
    };

    wsConnection.onerror = function () {
      setConnStatus("reconnecting", "reconnecting (ws)...");
    };

    wsConnection.onclose = function () {
      if (state.complete) {
        setConnStatus("disconnected", "disconnected");
        return;
      }

      setConnStatus("reconnecting", "reconnecting (ws)...");

      setTimeout(function () {
        reconnectDelay = Math.min(reconnectDelay * 1.5, maxReconnectDelay);
        connectWebSocket();
      }, reconnectDelay);
    };
  }

  // === Event Handlers ===

  function handleSnapshot(data) {
    state.report = data.report;
    state.events = data.events || [];
    state.dag = data.dag;
    state.complete = data.complete || false;
    state.maxEventSeq = 0;
    state.newStepNames = {};
    state.changedSteps = {};

    state.events.forEach(function (evt) {
      if (evt.sequence > state.maxEventSeq) {
        state.maxEventSeq = evt.sequence;
      }
      processEventIntoSteps(evt);
    });

    if (state.complete) {
      markComplete();
    }

    scheduleFullRender();
  }

  function handleEvent(evt) {
    if (evt.sequence <= state.maxEventSeq) return;
    state.maxEventSeq = evt.sequence;

    state.events.push(evt);
    processEventIntoSteps(evt);

    scheduleRender();
  }

  function handleComplete(data) {
    state.report = data.report;
    state.dag = data.dag;
    state.complete = true;
    markComplete();
    scheduleFullRender();
  }

  function markComplete() {
    document.body.classList.add("workflow-complete");
    els.liveBadge.classList.add("completed");
    els.liveBadge.innerHTML = '<span class="live-pulse"></span>DONE';
    setConnStatus("connected", "complete");
  }

  // === Process events into step state ===

  function processEventIntoSteps(evt) {
    var name = evt.step_name;
    if (!name) return;

    if (!state.steps[name]) {
      state.steps[name] = {
        step_name: name,
        step_type: evt.step_type || "",
        status: "pending",
        attempt_count: 0,
        max_attempts: 0,
        started_at: null,
        finished_at: null,
        duration_ms: null,
        dependencies: [],
        dependents: [],
        error: null,
        has_retry: false,
        has_timeout: false,
      };
      state.newStepNames[name] = true;
    }

    var step = state.steps[name];

    if (evt.event_type === "attempt_start") {
      if (!step.started_at) step.started_at = evt.timestamp;
      step.status = "running";
      step.attempt_count = Math.max(step.attempt_count, evt.attempt || 1);
    } else if (evt.event_type === "attempt_end") {
      step.finished_at = evt.timestamp;
      if (evt.duration_ms != null) step.duration_ms = evt.duration_ms;
      step.attempt_count = Math.max(step.attempt_count, evt.attempt || 1);

      if (evt.status) {
        var oldStatus = step.status;
        step.status = evt.status;
        if (oldStatus && oldStatus !== evt.status) {
          state.changedSteps[name] = evt.status;
        }
      }

      if (evt.status === "succeeded") {
        step.error = null;
      } else if (evt.error) {
        step.error = evt.error;
      }
    }
  }

  // === Render scheduling (debounced via requestAnimationFrame) ===

  var renderQueued = false;

  function scheduleRender() {
    if (renderQueued) return;
    renderQueued = true;
    requestAnimationFrame(function () {
      renderQueued = false;
      renderAll();
      updateGraphLive();
    });
  }

  var fullRenderQueued = false;

  function scheduleFullRender() {
    if (fullRenderQueued) return;
    fullRenderQueued = true;
    requestAnimationFrame(function () {
      fullRenderQueued = false;
      renderAll();
      renderGraph();
      renderTimeline();
    });
  }

  function renderAll() {
    renderHeader();
    renderStats();
    renderWaveform();
    renderLegend();
    renderFailureBanner();
    renderStepsTable();
    renderEventsTable();
    renderFooter();
  }

  // === Header ===

  function renderHeader() {
    if (!state.report) return;
    var r = state.report;
    els.workflowId.textContent = r.workflow_id || "unknown";
    els.runId.textContent = r.run_id || "\u2014";
  }

  // === Stats ===

  function renderStats() {
    if (!state.report) return;
    var r = state.report;

    var errorCount = r.failed_count + r.canceled_count;
    var stats = [
      { label: "Steps", value: r.step_count },
      { label: "Events", value: r.event_count },
      { label: "Succeeded", value: r.succeeded_count, cls: "success" },
      { label: "Failed", value: errorCount, cls: errorCount > 0 ? "error" : "success" },
      { label: "Wall Clock", value: humanizeDuration(r.wall_clock_duration_ms) },
    ];

    if (r.peak_concurrency) {
      stats.push({ label: "Peak Concurrency", value: r.peak_concurrency });
    }

    if (r.critical_path_duration_ms) {
      stats.push({ label: "Critical Path", value: humanizeDuration(r.critical_path_duration_ms) });
    }

    els.stats.innerHTML = stats
      .map(function (s) {
        return (
          '<div class="stat-card' +
          (s.cls ? " " + s.cls : "") +
          '"><div class="label">' +
          s.label +
          '</div><div class="value">' +
          s.value +
          "</div></div>"
        );
      })
      .join("");
  }

  // === Waveform ===

  function renderWaveform() {
    var events = state.events;
    if (!events.length) return;

    var waveform = els.waveform;
    var placeholder = waveform.querySelector(".waveform-placeholder");
    if (placeholder) placeholder.remove();

    var ts = events.map(function (e) {
      return new Date(e.timestamp).getTime();
    });
    var minT = Math.min.apply(null, ts);
    var maxT = Math.max.apply(null, ts);
    var range = maxT - minT || 1;
    var maxDur = Math.max.apply(
      null,
      events
        .filter(function (e) {
          return e.duration_ms;
        })
        .map(function (e) {
          return e.duration_ms;
        })
        .concat([1]),
    );

    waveform.innerHTML = events
      .map(function (e) {
        var t = new Date(e.timestamp).getTime();
        var pct = ((t - minT) / range) * 100;
        var color = e.error ? "var(--error)" : eventColors[e.event_type] || "var(--text-muted)";
        var dur = e.duration_ms || 0;
        var h = dur > 0 ? Math.max(4, (dur / maxDur) * 28) : 4;
        var tip =
          e.event_type +
          (e.step_name ? " " + e.step_name : "") +
          (e.phase ? " " + e.phase : "") +
          (e.attempt ? " attempt " + e.attempt : "") +
          (dur > 0 ? " " + humanizeDuration(dur) : "");
        return (
          '<div class="wf-event" style="left:' +
          pct.toFixed(2) +
          "%;height:" +
          h.toFixed(0) +
          "px;background:" +
          color +
          '" title="' +
          esc(tip) +
          '"></div>'
        );
      })
      .join("");
  }

  // === Legend ===

  function renderLegend() {
    var counts = {};
    Object.keys(state.steps).forEach(function (name) {
      var s = state.steps[name];
      if (s.status) counts[s.status] = (counts[s.status] || 0) + 1;
    });

    var order = ["succeeded", "failed", "running", "pending", "canceled", "skipped"];
    els.legend.innerHTML = order
      .map(function (k) {
        var count = counts[k] || 0;
        return count > 0
          ? '<div class="legend-item"><span class="icon">' +
              (statusIcons[k] || "") +
              "</span>" +
              esc(statusLabels[k] || k) +
              ' <span style="opacity:0.5">(' +
              count +
              ")</span></div>"
          : "";
      })
      .join("");
  }

  // === Failure Banner ===

  function renderFailureBanner() {
    if (!state.report) return;
    var r = state.report;
    if (r.workflow_succeeded) {
      els.failureBanner.style.display = "none";
      return;
    }
    if (r.failed_count === 0 && !r.failure_reason) {
      els.failureBanner.style.display = "none";
      return;
    }

    els.failureBanner.style.display = "";

    if (r.failure_reason) {
      els.failureReason.textContent = r.failure_reason;
    } else if (r.failed_count > 0) {
      els.failureReason.textContent = r.failed_count + " step(s) failed.";
    }

    var failedSteps = Object.keys(state.steps)
      .map(function (n) {
        return state.steps[n];
      })
      .filter(function (s) {
        return s.status === "failed" || s.status === "canceled";
      });

    els.failureList.innerHTML = failedSteps
      .map(function (s) {
        return (
          '<div class="failure-item"><span class="failure-item-name">' +
          esc(s.step_name) +
          '</span><span class="failure-item-error">' +
          esc(s.error || "(no error message)") +
          "</span></div>"
        );
      })
      .join("");
  }

  // === Steps Table ===

  var stepSortKey = "name";
  var stepSortDir = 1;
  var numericSortKeys = ["attempts", "duration"];

  function getSortedSteps() {
    var steps = Object.keys(state.steps).map(function (n) {
      return state.steps[n];
    });

    steps.sort(function (a, b) {
      var av, bv;
      if (stepSortKey === "name") {
        av = (a.step_name || "").toLowerCase();
        bv = (b.step_name || "").toLowerCase();
        return (av < bv ? -1 : av > bv ? 1 : 0) * stepSortDir;
      } else if (stepSortKey === "type") {
        av = a.step_type || "";
        bv = b.step_type || "";
        return (av < bv ? -1 : av > bv ? 1 : 0) * stepSortDir;
      } else if (stepSortKey === "status") {
        av = a.status || "";
        bv = b.status || "";
        return (av < bv ? -1 : av > bv ? 1 : 0) * stepSortDir;
      } else if (stepSortKey === "attempts") {
        return ((a.attempt_count || 0) - (b.attempt_count || 0)) * stepSortDir;
      } else if (stepSortKey === "duration") {
        return ((a.duration_ms || 0) - (b.duration_ms || 0)) * stepSortDir;
      }
      return 0;
    });

    return steps;
  }

  // Build the HTML for a single step row's <td> cells (not the <tr> wrapper).
  function buildStepCellsHTML(s) {
    var deps = (s.dependencies || [])
      .map(function (d) {
        return esc(d.step_name || d);
      })
      .join(", ");
    var depsR = (s.dependents || [])
      .map(function (d) {
        return esc(d.step_name || d);
      })
      .join(", ");

    var errMsg = s.error ? esc(s.error) : "";
    var statusBadge =
      '<span class="status-badge ' +
      esc(s.status) +
      '"' +
      (errMsg ? ' data-error="' + errMsg + '"' : "") +
      ">" +
      (statusIcons[s.status] || "") +
      " " +
      esc(s.status) +
      "</span>";

    var cfgBadges = "";
    if (s.has_retry) {
      cfgBadges +=
        '<span class="config-badge retry" title="Max attempts: ' +
        (s.max_attempts || 0) +
        '">\u{1F501} retry</span> ';
    }
    if (s.has_timeout) cfgBadges += '<span class="config-badge timeout">\u23F2 timeout</span>';

    return (
      '<td class="mono">' +
      esc(s.step_name) +
      "</td>" +
      '<td class="mono" style="color:var(--text-dim)">' +
      esc(s.step_type || "\u2014") +
      "</td>" +
      "<td>" +
      statusBadge +
      "</td>" +
      "<td>" +
      s.attempt_count +
      (s.max_attempts > 1 ? "/" + s.max_attempts : "") +
      "</td>" +
      "<td>" +
      (s.duration_ms ? humanizeDuration(s.duration_ms) : "\u2014") +
      "</td>" +
      '<td class="deps-list">' +
      (deps || "\u2014") +
      "</td>" +
      '<td class="deps-list">' +
      (depsR || "\u2014") +
      "</td>" +
      "<td>" +
      cfgBadges +
      "</td>" +
      '<td class="error-cell' +
      (errMsg ? "" : " empty") +
      '"' +
      (errMsg ? ' title="' + errMsg + '"' : "") +
      ">" +
      (errMsg || "\u2014") +
      "</td>"
    );
  }

  // Compact state key for change detection — only volatile fields.
  function stepStateKey(s) {
    return [
      s.status,
      s.attempt_count,
      s.duration_ms || 0,
      s.error || "",
      s.has_retry ? 1 : 0,
      s.max_attempts || 0,
    ].join("|");
  }

  // Update only the volatile cells of an existing row (status, attempts,
  // duration, error). Stable cells (name, type, deps, config) are left alone.
  function updateStepRow(tr, s) {
    tr.classList.toggle("step-row-running", s.status === "running");
    tr.classList.toggle("row-failed", s.status === "failed");
    tr.classList.toggle("row-canceled", s.status === "canceled");
    tr.setAttribute("data-has-error", s.status === "failed" || s.status === "canceled" ? "1" : "0");

    // Cell 2: status badge
    var errMsg = s.error ? esc(s.error) : "";
    tr.children[2].innerHTML =
      '<span class="status-badge ' +
      esc(s.status) +
      '"' +
      (errMsg ? ' data-error="' + errMsg + '"' : "") +
      ">" +
      (statusIcons[s.status] || "") +
      " " +
      esc(s.status) +
      "</span>";

    // Cell 3: attempts
    tr.children[3].textContent = s.attempt_count + (s.max_attempts > 1 ? "/" + s.max_attempts : "");

    // Cell 4: duration
    tr.children[4].textContent = s.duration_ms ? humanizeDuration(s.duration_ms) : "\u2014";

    // Cell 8: error
    tr.children[8].className = "error-cell" + (errMsg ? "" : " empty");
    if (errMsg) {
      tr.children[8].setAttribute("title", errMsg);
    } else {
      tr.children[8].removeAttribute("title");
    }
    tr.children[8].textContent = errMsg || "\u2014";
  }

  var STEP_PAGE_SIZE = 50;
  var stepExpanded = false;

  // stepRows tracks rendered rows for diff-based incremental updates.
  // Map: stepName → { tr: HTMLTableRowElement, key: string }
  var stepRows = {};

  function renderStepsTable() {
    var sortedSteps = getSortedSteps();

    var q = (els.stepSearch.value || "").toLowerCase();
    var errorsOnly = els.stepErrorsOnly.getAttribute("aria-pressed") === "true";
    var filtering = q.length > 0 || errorsOnly;

    var visibleSteps = sortedSteps.filter(function (s) {
      if (errorsOnly && s.status !== "failed" && s.status !== "canceled") return false;
      if (q.length > 0) {
        var searchText = (
          s.step_name +
          " " +
          (s.step_type || "") +
          " " +
          s.status +
          " " +
          (s.error || "")
        ).toLowerCase();
        if (searchText.indexOf(q) < 0) return false;
      }
      return true;
    });

    var pageSteps =
      !stepExpanded && visibleSteps.length > STEP_PAGE_SIZE
        ? visibleSteps.slice(0, STEP_PAGE_SIZE)
        : visibleSteps;

    var visibleNames = {};
    pageSteps.forEach(function (s) {
      visibleNames[s.step_name] = true;
    });

    // Remove rows that are no longer visible (filtered out or beyond page)
    Object.keys(stepRows).forEach(function (name) {
      if (!visibleNames[name]) {
        stepRows[name].tr.remove();
        delete stepRows[name];
      }
    });

    // Create / update / position rows in sorted order
    var prevTr = null;
    pageSteps.forEach(function (step) {
      var entry = stepRows[step.step_name];

      if (!entry) {
        // Create new row — one-time innerHTML cost per step
        var tr = document.createElement("tr");
        tr.innerHTML = buildStepCellsHTML(step);
        if (state.newStepNames[step.step_name]) tr.classList.add("step-row-new");
        if (step.status === "running") tr.classList.add("step-row-running");
        if (step.status === "failed") tr.classList.add("row-failed");
        if (step.status === "canceled") tr.classList.add("row-canceled");
        tr.setAttribute(
          "data-has-error",
          step.status === "failed" || step.status === "canceled" ? "1" : "0",
        );

        stepRows[step.step_name] = { tr: tr, key: stepStateKey(step) };
      } else {
        // Update existing row only if volatile state changed
        var newKey = stepStateKey(step);
        if (entry.key !== newKey) {
          updateStepRow(entry.tr, step);
          entry.key = newKey;
        }
      }

      // Position the row correctly in the DOM (cheap pointer move)
      var tr = stepRows[step.step_name].tr;
      if (prevTr) {
        if (prevTr.nextSibling !== tr) prevTr.after(tr);
      } else {
        if (els.stepsTbody.firstChild !== tr) els.stepsTbody.prepend(tr);
      }
      prevTr = tr;
    });

    els.stepsEmpty.style.display = pageSteps.length ? "none" : "";

    els.stepResultCount.textContent = filtering
      ? visibleSteps.length + " / " + sortedSteps.length + " steps"
      : "";

    // Clear new-step markers after render
    state.newStepNames = {};
    state.changedSteps = {};
  }

  // === Events Table ===

  var EVT_DISPLAY_CAP = 500;
  var evtFilter = "all";

  function renderEventsTable() {
    var events = state.events;

    // Build filter chips if not done
    if (!els.eventFilters.children.length) {
      var types = ["all", "attempt_start", "attempt_end"];
      els.eventFilters.innerHTML = types
        .map(function (t) {
          return (
            '<button class="chip' +
            (t === "all" ? " active" : "") +
            '" aria-pressed="' +
            (t === "all" ? "true" : "false") +
            '" data-filter="' +
            t +
            '">' +
            (t === "all" ? "all" : eventLabels[t] || t) +
            "</button>"
          );
        })
        .join("");

      els.eventFilters.querySelectorAll(".chip").forEach(function (btn) {
        btn.addEventListener("click", function () {
          els.eventFilters.querySelectorAll(".chip").forEach(function (b) {
            b.classList.remove("active");
            b.setAttribute("aria-pressed", "false");
          });
          btn.classList.add("active");
          btn.setAttribute("aria-pressed", "true");
          evtFilter = btn.dataset.filter;
          renderEventsTable();
        });
      });
    }

    var filtered = events.filter(function (e) {
      return evtFilter === "all" || e.event_type === evtFilter;
    });

    // Show most recent first for live view, cap display
    var display = filtered.slice(-EVT_DISPLAY_CAP);

    els.eventsTbody.innerHTML = display
      .map(function (e) {
        var phase = e.phase === "before" ? "\u25B4" : "\u25BE";
        var evtBadge =
          '<span class="event-badge ' +
          esc(e.event_type) +
          '">' +
          esc(eventLabels[e.event_type] || e.event_type) +
          "</span>";
        var dur = e.duration_ms != null ? humanizeDuration(e.duration_ms) : "";
        var errTip = e.error ? ' data-error="' + esc(e.error) + '"' : "";
        return (
          '<tr data-type="' +
          esc(e.event_type) +
          '" class="' +
          (e.error ? "has-error" : "") +
          '">' +
          '<td class="mono">' +
          e.sequence +
          "</td>" +
          '<td class="mono" title="' +
          esc(e.timestamp) +
          '">' +
          new Date(e.timestamp).toLocaleTimeString() +
          "</td>" +
          "<td>" +
          evtBadge +
          "</td>" +
          '<td class="mono">' +
          phase +
          "</td>" +
          '<td class="mono">' +
          esc(e.step_name) +
          "</td>" +
          "<td>" +
          (e.attempt || "") +
          "</td>" +
          '<td class="mono">' +
          dur +
          "</td>" +
          "<td" +
          errTip +
          ">" +
          (e.error ? '<span class="inline-error">' + esc(e.error) + "</span>" : "") +
          "</td>" +
          "</tr>"
        );
      })
      .join("");

    els.eventsEmpty.style.display = display.length ? "none" : "";
  }

  // === Graph ===

  var graphRendered = false;
  var statusColorMap = {
    succeeded: "var(--success)",
    failed: "var(--error)",
    running: "var(--warning)",
    pending: "var(--text-muted)",
    canceled: "var(--transient)",
    skipped: "var(--text-dim)",
  };

  // updateGraphLive incrementally updates node colors in the existing SVG
  // without rebuilding the entire graph. Called on every incremental render.
  function updateGraphLive() {
    if (!graphRendered) return;

    var svg = els.graphContainer.querySelector("svg");
    if (!svg) return;

    // daghtml nodes have data-id matching the step name
    var nodes = svg.querySelectorAll("[data-id]");
    nodes.forEach(function (node) {
      var nodeId = node.getAttribute("data-id");
      if (!nodeId) return;

      var step = state.steps[nodeId];
      if (!step) return;

      var color = statusColorMap[step.status] || "var(--accent)";

      // Update fill on the primary shape (rect, circle, or ellipse)
      var shape = node.querySelector("rect, circle, ellipse");
      if (shape) {
        shape.style.fill = color;
        shape.style.transition = "fill 0.3s ease";
      }

      // Update label with status icon + duration (matches Go buildDAGHTML format)
      var textEl = node.querySelector("text");
      if (textEl) {
        var label = step.step_name;
        var icon = statusIcons[step.status];
        if (icon) label = icon + " " + label;
        if (step.duration_ms && step.duration_ms > 0) {
          label += " \u00b7 " + humanizeMs(step.duration_ms);
        }
        textEl.textContent = label;
      }

      // Flash recently changed nodes
      if (state.changedSteps[nodeId]) {
        node.classList.add("node-flash");
        setTimeout(function () {
          node.classList.remove("node-flash");
        }, 600);
      }
    });
  }

  // Compute the critical path (longest duration chain) from report data.
  function computeCriticalPathSteps() {
    if (!state.report) return [];

    if (state.report.critical_path_steps && state.report.critical_path_steps.length > 0) {
      return state.report.critical_path_steps;
    }

    var steps = state.report.steps || [];
    var byName = {};
    steps.forEach(function (s) {
      byName[s.step_name] = s;
    });

    var memo = {};
    var bestChain = [];

    function dfs(name) {
      if (memo[name] !== undefined) return memo[name];

      var step = byName[name];
      if (!step) {
        memo[name] = { duration: 0, chain: [] };
        return memo[name];
      }

      var duration = step.duration_ms || 0;
      var deps = step.dependencies || [];
      var bestChild = { duration: 0, chain: [] };

      deps.forEach(function (d) {
        var child = dfs(d.step_name);
        if (child.duration > bestChild.duration) {
          bestChild = child;
        }
      });

      var result = {
        duration: duration + bestChild.duration,
        chain: bestChild.chain.concat([name]),
      };
      memo[name] = result;
      return result;
    }

    steps.forEach(function (s) {
      var r = dfs(s.step_name);
      if (r.duration > 0 && r.chain.length > bestChain.length) {
        bestChain = r.chain;
      }
    });

    return bestChain;
  }

  // Build a map from dag-data node index to step name.
  function buildNodeNameMap() {
    var dataEl = document.getElementById("dag-data");
    if (!dataEl) return {};

    var data = JSON.parse(dataEl.textContent);
    var map = {};
    (data.nodes || []).forEach(function (n, i) {
      map[i] = n.id;
    });
    return map;
  }

  function renderGraph() {
    if (!state.dag) {
      if (els.graphPlaceholder) els.graphPlaceholder.style.display = "";
      return;
    }

    if (els.graphPlaceholder) els.graphPlaceholder.style.display = "none";

    // Update or create the data element
    var dataEl = document.getElementById("dag-data");
    if (!dataEl) {
      dataEl = document.createElement("script");
      dataEl.type = "application/json";
      dataEl.id = "dag-data";
      document.body.appendChild(dataEl);
    }
    dataEl.textContent = JSON.stringify(state.dag);

    // Clear previous graph and re-init
    var container = els.graphContainer;
    var existingSvg = container.querySelector("svg");
    if (existingSvg) existingSvg.remove();

    container.dataset.enhanced = "";

    if (typeof initDAGGraph === "function") {
      initDAGGraph("graph-container", "dag-data");
      enhanceGraph();
      renderMinimap();
    }

    graphRendered = true;
  }

  function enhanceGraph() {
    var container = els.graphContainer;
    var svg = container.querySelector("svg");
    if (!svg) return;

    // Build a local steps array for graph enhancements — never mutate state.report
    var stepsForGraph;
    if (state.report && state.report.steps && state.report.steps.length) {
      stepsForGraph = state.report.steps;
    } else {
      stepsForGraph = Object.keys(state.steps).map(function (n) {
        return state.steps[n];
      });
      if (!stepsForGraph.length) return;
    }

    var nameMap = buildNodeNameMap();
    var stepByName = {};
    stepsForGraph.forEach(function (s) {
      stepByName[s.step_name] = s;
    });

    // Retry badges on nodes with attempt_count > 1
    var nodeEls = container.querySelectorAll(".graph-node");
    var ns = "http://www.w3.org/2000/svg";
    nodeEls.forEach(function (g) {
      var idx = parseInt(g.getAttribute("data-id"));
      var stepName = nameMap[idx];
      if (!stepName) return;

      var step = stepByName[stepName] || state.steps[stepName];
      if (!step || !step.attempt_count || step.attempt_count <= 1) return;

      var rect = g.querySelector("rect");
      if (!rect) return;
      var w = parseFloat(rect.getAttribute("width"));

      var badge = document.createElementNS(ns, "g");
      badge.classList.add("retry-badge");
      badge.setAttribute("transform", "translate(" + (w - 24) + ", -6)");

      var circle = document.createElementNS(ns, "circle");
      circle.setAttribute("cx", 8);
      circle.setAttribute("cy", 8);
      circle.setAttribute("r", 8);
      circle.setAttribute("fill", "var(--warning)");
      circle.setAttribute("stroke", "var(--bg-elevated)");
      circle.setAttribute("stroke-width", 1.5);
      badge.appendChild(circle);

      var text = document.createElementNS(ns, "text");
      text.setAttribute("x", 8);
      text.setAttribute("y", 8);
      text.setAttribute("text-anchor", "middle");
      text.setAttribute("dominant-baseline", "central");
      text.setAttribute("fill", "var(--bg)");
      text.setAttribute("font-size", "10");
      text.setAttribute("font-weight", "bold");
      text.setAttribute("font-family", "var(--font-mono)");
      text.textContent = "\u21bb" + step.attempt_count;
      badge.appendChild(text);

      g.appendChild(badge);
    });

    // Node click → navigate to step in Steps tab
    nodeEls.forEach(function (g) {
      g.style.cursor = "pointer";
      g.addEventListener("click", function () {
        var idx = parseInt(g.getAttribute("data-id"));
        var stepName = nameMap[idx];
        if (!stepName) return;

        var stepsTab = document.querySelector('.tab[data-tab="steps"]');
        if (stepsTab) switchTab(stepsTab);

        setTimeout(function () {
          var rows = document.querySelectorAll("#steps-tbody tr");
          rows.forEach(function (row) {
            if (row.textContent.indexOf(stepName) !== -1) {
              row.classList.add("row-highlight");
              row.scrollIntoView({ behavior: "smooth", block: "center" });
              setTimeout(function () {
                row.classList.remove("row-highlight");
              }, 2000);
            }
          });
        }, 50);
      });
    });

    // Edge color coding by target status
    var edgeEls = container.querySelectorAll(".graph-edge");
    edgeEls.forEach(function (e) {
      var targetIdx = parseInt(e.getAttribute("data-target"));
      var targetName = nameMap[targetIdx];
      if (!targetName) return;
      var step = stepByName[targetName] || state.steps[targetName];
      if (!step) return;
      if (step.status === "failed" || step.status === "canceled") {
        e.classList.add("edge-failed");
      } else if (step.status === "succeeded") {
        e.classList.add("edge-succeeded");
      }
    });

    // Critical path
    var criticalPathSteps = computeCriticalPathSteps();
    var criticalPathSet = {};
    criticalPathSteps.forEach(function (name) {
      criticalPathSet[name] = true;
    });

    // Update info text
    var infoEl = document.getElementById("graph-info-text");
    if (infoEl && criticalPathSteps.length > 1) {
      infoEl.textContent =
        "Critical path: " +
        criticalPathSteps.length +
        " steps" +
        (state.report.critical_path_duration_ms
          ? " \u00b7 " + humanizeDuration(state.report.critical_path_duration_ms)
          : "");
    }

    // Critical path button
    var cpBtn = document.getElementById("graph-critical-path");
    if (cpBtn) {
      if (criticalPathSteps.length <= 1) {
        cpBtn.style.display = "none";
      } else {
        cpBtn.style.display = "";

        // Auto-highlight critical path by default
        if (cpBtn.getAttribute("aria-pressed") !== "true") {
          cpBtn.setAttribute("aria-pressed", "true");
          cpBtn.classList.add("active");
          toggleCriticalPathHighlight(container, nameMap, criticalPathSet, true);
        }

        // Remove old listeners by cloning
        var newCpBtn = cpBtn.cloneNode(true);
        cpBtn.parentNode.replaceChild(newCpBtn, cpBtn);
        newCpBtn.addEventListener("click", function () {
          var active = newCpBtn.getAttribute("aria-pressed") === "true";
          newCpBtn.setAttribute("aria-pressed", !active);
          newCpBtn.classList.toggle("active", !active);
          toggleCriticalPathHighlight(container, nameMap, criticalPathSet, !active);
        });
      }
    }

    // Graph search
    var searchInput = document.getElementById("graph-search");
    if (searchInput) {
      var newSearch = searchInput.cloneNode(true);
      searchInput.parentNode.replaceChild(newSearch, searchInput);
      var searchTimer2;
      newSearch.addEventListener("input", function () {
        clearTimeout(searchTimer2);
        searchTimer2 = setTimeout(function () {
          applyGraphSearch(container, nameMap, newSearch.value.toLowerCase());
        }, 120);
      });
    }

    container.dataset.enhanced = "true";
  }

  function toggleCriticalPathHighlight(container, nameMap, criticalPathSet, active) {
    var nodeEls = container.querySelectorAll(".graph-node");
    nodeEls.forEach(function (g) {
      var idx = parseInt(g.getAttribute("data-id"));
      var stepName = nameMap[idx];
      if (active && criticalPathSet[stepName]) {
        g.classList.add("critical-path");
      } else {
        g.classList.remove("critical-path");
      }
    });

    var edgeEls = container.querySelectorAll(".graph-edge");
    edgeEls.forEach(function (eg) {
      var sIdx = parseInt(eg.getAttribute("data-source"));
      var tIdx = parseInt(eg.getAttribute("data-target"));
      var sName = nameMap[sIdx];
      var tName = nameMap[tIdx];
      if (active && criticalPathSet[sName] && criticalPathSet[tName]) {
        eg.classList.add("critical-path");
      } else {
        eg.classList.remove("critical-path");
      }
    });
  }

  function applyGraphSearch(container, nameMap, query) {
    var nodeEls = container.querySelectorAll(".graph-node");
    if (!query) {
      nodeEls.forEach(function (g) {
        g.classList.remove("search-match", "search-dimmed");
      });
      return;
    }
    nodeEls.forEach(function (g) {
      var idx = parseInt(g.getAttribute("data-id"));
      var stepName = nameMap[idx] || "";
      if (stepName.toLowerCase().indexOf(query) >= 0) {
        g.classList.add("search-match");
        g.classList.remove("search-dimmed");
      } else {
        g.classList.add("search-dimmed");
        g.classList.remove("search-match");
      }
    });
  }

  function renderMinimap() {
    var minimapEl = document.getElementById("graph-minimap");
    if (!minimapEl) return;

    var dataEl = document.getElementById("dag-data");
    if (!dataEl) return;

    var data = JSON.parse(dataEl.textContent);
    var nodeCount = (data.nodes || []).length;

    // Only show minimap for large graphs
    if (nodeCount <= 20) {
      minimapEl.style.display = "none";
      return;
    }

    minimapEl.style.display = "";
    minimapEl.innerHTML = "";

    // Create a scaled-down clone of the SVG
    var originalSvg = els.graphContainer.querySelector("svg");
    if (!originalSvg) return;

    // Capture full graph bounds from the main SVG's initial viewBox.
    // daghtml sets the viewBox once at render; pan/zoom mutate it afterwards.
    var fullVB = originalSvg.viewBox.baseVal;
    var fullX = fullVB.x,
      fullY = fullVB.y,
      fullW = fullVB.width,
      fullH = fullVB.height;

    var clone = originalSvg.cloneNode(true);
    clone.removeAttribute("width");
    clone.removeAttribute("height");
    clone.style.width = "100%";
    clone.style.height = "100%";
    clone.style.pointerEvents = "none";
    // Lock the clone's viewBox to the full graph so it always shows everything
    clone.setAttribute("viewBox", fullX + " " + fullY + " " + fullW + " " + fullH);

    minimapEl.appendChild(clone);

    // Add viewport indicator rectangle
    var ns = "http://www.w3.org/2000/svg";
    var viewport = document.createElementNS(ns, "rect");
    viewport.classList.add("minimap-viewport");
    viewport.setAttribute("fill", "rgba(100,160,255,0.15)");
    viewport.setAttribute("stroke", "var(--accent)");
    viewport.setAttribute("stroke-width", "2");
    viewport.setAttribute("pointer-events", "none");

    // Append viewport rect to clone's SVG
    var cloneSvg = minimapEl.querySelector("svg");
    if (cloneSvg) {
      cloneSvg.appendChild(viewport);
    }

    // Update the viewport indicator to match the main SVG's current viewBox.
    function syncViewport() {
      var vb = originalSvg.viewBox.baseVal;
      if (!vb || !vb.width) return;
      viewport.setAttribute("x", vb.x);
      viewport.setAttribute("y", vb.y);
      viewport.setAttribute("width", vb.width);
      viewport.setAttribute("height", vb.height);
    }

    syncViewport();

    // Watch main SVG viewBox attribute changes (daghtml updates via setAttribute)
    var minimapObserver = new MutationObserver(function () {
      syncViewport();
    });
    minimapObserver.observe(originalSvg, {
      attributes: true,
      attributeFilter: ["viewBox"],
    });

    // Click-to-navigate: center the main graph on the clicked point
    minimapEl.style.pointerEvents = "auto";
    minimapEl.addEventListener("click", function (e) {
      var rect = minimapEl.getBoundingClientRect();
      var pctX = (e.clientX - rect.left) / rect.width;
      var pctY = (e.clientY - rect.top) / rect.height;

      var mainSvg = els.graphContainer.querySelector("svg");
      if (!mainSvg) return;

      var vb = mainSvg.viewBox.baseVal;
      if (!vb || !vb.width) return;

      // Map click percentage to graph coordinates, then center
      var targetX = fullX + pctX * fullW - vb.width / 2;
      var targetY = fullY + pctY * fullH - vb.height / 2;
      mainSvg.setAttribute("viewBox", targetX + " " + targetY + " " + vb.width + " " + vb.height);
      // MutationObserver will sync the minimap indicator
    });
  }

  // === Timeline ===

  function renderTimeline() {
    if (!state.report || !state.report.steps) return;

    var steps = state.report.steps.filter(function (s) {
      return s.started_at;
    });
    if (!steps.length) {
      els.timelineContainer.innerHTML =
        '<div class="graph-placeholder">No timing data yet...</div>';
      return;
    }

    steps.sort(function (a, b) {
      return new Date(a.started_at) - new Date(b.started_at);
    });

    var minT = new Date(steps[0].started_at).getTime();
    var maxT = Math.max.apply(
      null,
      steps.map(function (s) {
        return new Date(s.finished_at || s.started_at).getTime();
      }),
    );
    var range = maxT - minT || 1;

    function pct(ts) {
      return ((ts - minT) / range) * 100;
    }

    var rowsHtml = steps
      .map(function (s) {
        var st = new Date(s.started_at).getTime();
        var ft = s.finished_at ? new Date(s.finished_at).getTime() : st;
        var left = pct(st).toFixed(2);
        var width = Math.max(0.3, pct(ft) - pct(st)).toFixed(2);
        var icon = statusIcons[s.status] || "";
        var tip =
          esc(s.step_name) +
          " | " +
          s.status +
          (s.duration_ms ? " | " + humanizeDuration(s.duration_ms) : "") +
          (s.attempt_count > 1 ? " | " + s.attempt_count + " attempts" : "");
        return (
          '<div class="gantt-row">' +
          '<div class="gantt-label">' +
          (icon ? icon + " " : "") +
          esc(s.step_name) +
          "</div>" +
          '<div class="gantt-track">' +
          '<div class="gantt-bar ' +
          esc(s.status) +
          '" style="left:' +
          left +
          "%;width:" +
          width +
          '%" title="' +
          tip +
          '"></div>' +
          "</div></div>"
        );
      })
      .join("");

    els.timelineContainer.innerHTML = '<div class="gantt-grid">' + rowsHtml + "</div>";
  }

  // === Footer ===

  function renderFooter() {
    els.footerTs.textContent = new Date().toLocaleString();
    var evtCount = state.events.length;
    var stepCount = Object.keys(state.steps).length;
    els.footerStats.textContent = evtCount + " events \u00b7 " + stepCount + " steps";
  }

  // === Tab Management ===

  document.querySelectorAll(".tab").forEach(function (tab) {
    tab.addEventListener("click", function () {
      switchTab(tab);
    });
    tab.setAttribute("tabindex", tab.classList.contains("active") ? "0" : "-1");
  });

  function switchTab(tab) {
    document.querySelectorAll(".tab").forEach(function (t) {
      t.classList.remove("active");
      t.setAttribute("aria-selected", "false");
      t.setAttribute("tabindex", "-1");
    });
    document.querySelectorAll(".tab-content").forEach(function (t) {
      t.classList.remove("active");
    });
    tab.classList.add("active");
    tab.setAttribute("aria-selected", "true");
    tab.setAttribute("tabindex", "0");
    tab.focus();
    document.getElementById("tab-" + tab.dataset.tab).classList.add("active");

    if (tab.dataset.tab === "graph") renderGraph();
  }

  // Keyboard shortcuts
  var tabList = Array.prototype.slice.call(document.querySelectorAll(".tab"));
  document.addEventListener("keydown", function (e) {
    var tag = e.target.tagName;
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;

    if (e.key === "ArrowRight" || e.key === "ArrowLeft") {
      var focused = document.querySelector(".tab:focus");
      if (focused) {
        e.preventDefault();
        var cur = tabList.indexOf(focused);
        var next =
          e.key === "ArrowRight"
            ? (cur + 1) % tabList.length
            : (cur - 1 + tabList.length) % tabList.length;
        switchTab(tabList[next]);
      }
    }
  });

  // === Sortable headers ===

  document.querySelectorAll("#tab-steps th.sortable").forEach(function (th) {
    th.addEventListener("click", function () {
      var key = th.dataset.sort;
      if (stepSortKey === key) {
        stepSortDir *= -1;
      } else {
        stepSortKey = key;
        stepSortDir = 1;
      }
      document.querySelectorAll("#tab-steps th.sortable").forEach(function (t) {
        t.classList.remove("sort-asc", "sort-desc");
      });
      th.classList.add(stepSortDir === 1 ? "sort-asc" : "sort-desc");
      renderStepsTable();
    });
  });

  // === Search + filter ===

  var searchTimer;
  els.stepSearch.addEventListener("input", function () {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(renderStepsTable, 120);
  });

  els.stepErrorsOnly.addEventListener("click", function () {
    var pressed = els.stepErrorsOnly.getAttribute("aria-pressed") === "true";
    els.stepErrorsOnly.setAttribute("aria-pressed", !pressed);
    els.stepErrorsOnly.classList.toggle("active", !pressed);
    renderStepsTable();
  });

  // === Error tooltip ===

  var tooltip = document.getElementById("error-tooltip");
  document.addEventListener("click", function (e) {
    var badge = e.target.closest(".status-badge.failed, .status-badge.canceled");
    if (badge) {
      e.stopPropagation();
      var msg = badge.getAttribute("data-error");
      if (!msg) return;
      var r = badge.getBoundingClientRect();
      tooltip.textContent = msg;
      tooltip.style.left = "-9999px";
      tooltip.classList.add("visible");
      var tw = tooltip.offsetWidth,
        th = tooltip.offsetHeight;
      var left = r.left;
      var top = r.bottom + 6;
      if (left + tw > window.innerWidth - 12) left = Math.max(12, window.innerWidth - tw - 12);
      if (top + th > window.innerHeight - 12) top = Math.max(12, r.top - th - 6);
      tooltip.style.left = left + "px";
      tooltip.style.top = top + "px";
    } else {
      tooltip.classList.remove("visible");
    }
  });

  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape" && tooltip.classList.contains("visible")) {
      tooltip.classList.remove("visible");
    }
  });

  // === Start ===

  var pfx = window.ROUTE_PREFIX || "";
  var exportJson = document.getElementById("export-json");
  var exportNdjson = document.getElementById("export-ndjson");
  var exportHtml = document.getElementById("export-html");
  if (exportJson) exportJson.href = pfx + "/api/report";
  if (exportNdjson) exportNdjson.href = pfx + "/api/export/ndjson";
  if (exportHtml) exportHtml.href = pfx + "/api/export/html";

  connect();
})();
