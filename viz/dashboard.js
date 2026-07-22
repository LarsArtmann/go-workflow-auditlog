const report = JSON.parse(document.getElementById("report-data").textContent);
const meta = JSON.parse(document.getElementById("type-metadata").textContent);

function esc(s) {
  if (!s) return "";
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

const statusIcons = Object.fromEntries(Object.entries(meta.statuses).map(([k, v]) => [k, v.icon]));
const statusLabels = Object.fromEntries(
  Object.entries(meta.statuses).map(([k, v]) => [k, v.label]),
);
const eventLabels = Object.fromEntries(Object.entries(meta.events).map(([k, v]) => [k, v.label]));
const eventColors = Object.fromEntries(Object.entries(meta.events).map(([k, v]) => [k, v.color]));

document.getElementById("workflow-id").textContent = report.workflow_id || "unknown";
if (report.run_id) {
  document.getElementById("run-id").textContent = report.run_id;
} else {
  document.getElementById("run-id").textContent = "\u2014";
}
document.getElementById("exported-at").textContent = new Date(report.exported_at).toLocaleString();

// Workflow status hero badge
(function renderWorkflowStatus() {
  var el = document.getElementById("workflow-status");
  if (report.workflow_succeeded) {
    el.className = "workflow-status passed";
    el.innerHTML = "&#10003; Passed";
  } else {
    el.className = "workflow-status failed";
    el.innerHTML = "&#10007; Failed";
  }
})();

// Failure summary banner
(function renderFailureBanner() {
  if (report.workflow_succeeded) return;
  var banner = document.getElementById("failure-banner");
  var reasonEl = document.getElementById("failure-reason");
  var listEl = document.getElementById("failure-list");
  if (report.failure_reason) {
    reasonEl.textContent = report.failure_reason;
  } else if (report.failed_count > 0) {
    reasonEl.textContent = report.failed_count + " step(s) failed.";
  } else {
    return;
  }
  var failedSteps = (report.steps || []).filter(function (s) {
    return s.status === "failed" || s.status === "canceled";
  });
  listEl.innerHTML = failedSteps
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
  banner.classList.add("visible");
})();

// Compute failure-impact: steps skipped/canceled because a dependency failed
var impactedSteps = {};
(function computeImpact() {
  var failedNames = {};
  (report.steps || []).forEach(function (s) {
    if (s.status === "failed" || s.status === "canceled") {
      failedNames[s.step_name] = true;
    }
  });
  (report.steps || []).forEach(function (s) {
    if (s.status === "skipped" || s.status === "canceled" || s.status === "pending") {
      var deps = s.dependencies || [];
      var blockedBy = deps.filter(function (d) {
        return failedNames[d.step_name];
      });
      if (blockedBy.length) {
        impactedSteps[s.step_name] = blockedBy
          .map(function (d) {
            return d.step_name;
          })
          .join(", ");
      }
    }
  });
})();

// Status legend with counts
const statusCounts = {};
report.steps.forEach((s) => {
  if (s.status) statusCounts[s.status] = (statusCounts[s.status] || 0) + 1;
});
const statusOrder = ["succeeded", "failed", "running", "pending", "canceled", "skipped"];
document.getElementById("legend").innerHTML = statusOrder
  .map((k) => {
    const count = statusCounts[k] || 0;
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

// Lifecycle waveform
(function renderWaveform() {
  const events = report.events;
  if (!events || !events.length) return;
  const container = document.getElementById("waveform");
  const colors = eventColors;
  const ts = events.map((e) => new Date(e.timestamp).getTime());
  const minT = Math.min.apply(null, ts),
    maxT = Math.max.apply(null, ts),
    range = maxT - minT || 1;
  const maxDur = Math.max.apply(
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
  container.innerHTML = events
    .map(function (e) {
      const t = new Date(e.timestamp).getTime();
      const pct = ((t - minT) / range) * 100;
      const color = e.error ? "var(--error)" : colors[e.event_type] || "var(--text-muted)";
      const dur = e.duration_ms || 0;
      const h = dur > 0 ? Math.max(4, (dur / maxDur) * 28) : 4;
      const tip =
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
})();

// Stats
const errorCount = report.failed_count + report.canceled_count;
document.getElementById("stats").innerHTML = [
  { label: "Steps", value: report.step_count },
  { label: "Events", value: report.event_count },
  { label: "Succeeded", value: report.succeeded_count, cls: "success" },
  { label: "Failed", value: errorCount, cls: errorCount > 0 ? "error" : "success" },
  { label: "Wall Clock", value: humanizeDuration(report.wall_clock_duration_ms) },
  report.peak_concurrency ? { label: "Peak Concurrency", value: report.peak_concurrency } : null,
  report.critical_path_duration_ms
    ? { label: "Critical Path", value: humanizeDuration(report.critical_path_duration_ms) }
    : null,
]
  .filter(Boolean)
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

// Tabs + keyboard shortcuts
var tabMap = { 1: "steps", 2: "tree", 3: "graph", 4: "timeline", 5: "events" };
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
var tabList = Array.prototype.slice.call(document.querySelectorAll(".tab"));
document.addEventListener("keydown", function (e) {
  var tag = e.target.tagName;
  var onTab = e.target.classList && e.target.classList.contains("tab");
  if (onTab) {
    var cur = tabList.indexOf(e.target);
    if (e.key === "ArrowRight") {
      e.preventDefault();
      switchTab(tabList[(cur + 1) % tabList.length]);
      return;
    }
    if (e.key === "ArrowLeft") {
      e.preventDefault();
      switchTab(tabList[(cur - 1 + tabList.length) % tabList.length]);
      return;
    }
    if (e.key === "Home") {
      e.preventDefault();
      switchTab(tabList[0]);
      return;
    }
    if (e.key === "End") {
      e.preventDefault();
      switchTab(tabList[tabList.length - 1]);
      return;
    }
  }
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || tag === "BUTTON") return;
  if (e.key === "e" || e.key === "E") {
    var stepsActive = document.getElementById("tab-steps").classList.contains("active");
    if (stepsActive) {
      e.preventDefault();
      toggleErrorsOnly();
      return;
    }
  }
  var tab = tabMap[e.key];
  if (tab) {
    var btn = document.querySelector('.tab[data-tab="' + tab + '"]');
    if (btn) switchTab(btn);
  }
});

// Error tooltip
var tooltip = document.getElementById("error-tooltip");
function showTooltip(el) {
  var msg = el.getAttribute("data-error");
  if (!msg) return;
  var r = el.getBoundingClientRect();
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
}
document.addEventListener("click", function (e) {
  var badge = e.target.closest(".status-badge.failed, .status-badge.canceled");
  if (badge) {
    e.stopPropagation();
    showTooltip(badge);
  } else {
    tooltip.classList.remove("visible");
  }
});
document.addEventListener("keydown", function (e) {
  if (e.key === "Escape" && tooltip.classList.contains("visible")) {
    tooltip.classList.remove("visible");
  }
});

function configBadges(s) {
  var badges = "";
  if (s.has_retry)
    badges +=
      '<span class="config-badge retry" title="Max attempts: ' +
      (s.max_attempts || 0) +
      '">\u{1F501} retry</span> ';
  if (s.has_timeout) badges += '<span class="config-badge timeout">\u23F2 timeout</span>';
  return badges;
}

// Steps table
var allSteps = report.steps.map(function (s) {
  var deps = (s.dependencies || [])
    .map(function (d) {
      return "<span>" + esc(d.step_name) + "</span>";
    })
    .join(", ");
  var depsR = (s.dependents || [])
    .map(function (d) {
      return "<span>" + esc(d.step_name) + "</span>";
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
  var hasError = s.status === "failed" || s.status === "canceled" ? "1" : "0";
  var rowCls =
    s.status === "failed" ? " row-failed" : s.status === "canceled" ? " row-canceled" : "";
  return (
    '<tr data-search="' +
    esc(
      (
        s.step_name +
        " " +
        (s.step_type || "") +
        " " +
        s.status +
        " " +
        (s.error || "")
      ).toLowerCase(),
    ) +
    '"' +
    ' class="' +
    rowCls.trim() +
    '"' +
    ' data-sort-name="' +
    esc((s.step_name || "").toLowerCase()) +
    '"' +
    ' data-sort-type="' +
    esc(s.step_type || "") +
    '"' +
    ' data-sort-status="' +
    esc(s.status) +
    '"' +
    ' data-sort-attempts="' +
    s.attempt_count +
    '"' +
    ' data-sort-duration="' +
    (s.duration_ms || 0) +
    '"' +
    ' data-has-error="' +
    hasError +
    '">' +
    '<td class="mono" title="' +
    (s.started_at ? "Started: " + esc(s.started_at) : "") +
    (s.finished_at ? "\nFinished: " + esc(s.finished_at) : "") +
    '">' +
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
    configBadges(s) +
    "</td>" +
    '<td class="error-cell' +
    (errMsg ? "" : " empty") +
    '"' +
    (errMsg ? ' title="' + errMsg + '"' : "") +
    ">" +
    (errMsg ||
      (impactedSteps[s.step_name]
        ? '<span class="impact-badge" title="Blocked by: ' +
          esc(impactedSteps[s.step_name]) +
          '">&#9888; blocked</span>'
        : "\u2014")) +
    "</td>" +
    "</tr>"
  );
});
document.getElementById("steps-tbody").innerHTML = allSteps.join("");
document.getElementById("steps-empty").style.display = allSteps.length ? "none" : "";

// Steps: sort + filter + pagination
var STEP_PAGE_SIZE = 50;
var stepTbody = document.getElementById("steps-tbody");
var stepRows = Array.prototype.slice.call(stepTbody.querySelectorAll("tr"));
var stepExpanded = false;
var stepSortKey = "name";
var stepSortDir = 1;
var numericSortKeys = ["attempts", "duration"];

function syncStepMoreBar(visibleCount) {
  var existing = stepTbody.parentElement.querySelector("#step-more-bar");
  var totalShown = stepRows.length;
  var needBar = !stepExpanded && visibleCount < totalShown;
  if (existing && !needBar) existing.remove();
  if (!existing && needBar) {
    var moreBar = document.createElement("div");
    moreBar.id = "step-more-bar";
    moreBar.style.cssText = "text-align:center;padding:0.75rem";
    moreBar.innerHTML =
      '<button class="chip" id="show-more-step">Showing ' +
      visibleCount +
      " of " +
      totalShown +
      " \u2014 Show all</button>";
    stepTbody.parentElement.appendChild(moreBar);
    document.getElementById("show-more-step").addEventListener("click", function () {
      stepExpanded = true;
      var eb = stepTbody.parentElement.querySelector("#step-more-bar");
      if (eb) eb.remove();
      applyStepView();
    });
  } else if (existing && needBar) {
    existing.querySelector("button").textContent =
      "Showing " + visibleCount + " of " + totalShown + " \u2014 Show all";
  }
}

function applyStepView() {
  stepRows.sort(function (a, b) {
    var av = a.dataset["sort-" + stepSortKey],
      bv = b.dataset["sort-" + stepSortKey];
    var cmp;
    if (numericSortKeys.indexOf(stepSortKey) >= 0) {
      cmp = parseFloat(av) - parseFloat(bv);
    } else {
      cmp = av < bv ? -1 : av > bv ? 1 : 0;
    }
    return cmp * stepSortDir;
  });
  stepRows.forEach(function (tr) {
    stepTbody.appendChild(tr);
  });
  document.querySelectorAll("#tab-steps th.sortable").forEach(function (th) {
    th.classList.remove("sort-asc", "sort-desc");
    if (th.dataset.sort === stepSortKey)
      th.classList.add(stepSortDir === 1 ? "sort-asc" : "sort-desc");
  });
  var q = document.getElementById("step-search").value.toLowerCase();
  var errorsOnly =
    document.getElementById("step-errors-only").getAttribute("aria-pressed") === "true";
  var searching = q.length > 0;
  var visible = 0;
  stepRows.forEach(function (tr) {
    var show = true;
    if (searching && tr.dataset.search.indexOf(q) < 0) show = false;
    if (errorsOnly && tr.dataset.hasError !== "1") show = false;
    if (show && !stepExpanded && visible >= STEP_PAGE_SIZE) show = false;
    tr.style.display = show ? "" : "none";
    if (show) visible++;
  });
  var total = stepRows.length;
  var filtered = stepRows.filter(function (tr) {
    var match = true;
    if (searching && tr.dataset.search.indexOf(q) < 0) match = false;
    if (errorsOnly && tr.dataset.hasError !== "1") match = false;
    return match;
  }).length;
  var countEl = document.getElementById("step-result-count");
  countEl.textContent = searching || errorsOnly ? filtered + " / " + total + " steps" : "";
  syncStepMoreBar(visible);
}

document.querySelectorAll("#tab-steps th.sortable").forEach(function (th) {
  th.addEventListener("click", function () {
    var key = th.dataset.sort;
    if (stepSortKey === key) {
      stepSortDir *= -1;
    } else {
      stepSortKey = key;
      stepSortDir = 1;
    }
    applyStepView();
  });
});

document.getElementById("step-errors-only").addEventListener("click", toggleErrorsOnly);

(function setupErrorsOnlyBadge() {
  var errorSteps = report.steps.filter(function (s) {
    return s.status === "failed" || s.status === "canceled";
  }).length;
  if (errorSteps > 0) {
    var btn = document.getElementById("step-errors-only");
    btn.innerHTML = "Errors only <strong>(" + errorSteps + ")</strong>";
    btn.style.borderColor = "var(--error)";
    btn.style.color = "var(--error)";
  }
})();

function toggleErrorsOnly() {
  var btn = document.getElementById("step-errors-only");
  var pressed = btn.getAttribute("aria-pressed") === "true";
  btn.setAttribute("aria-pressed", !pressed);
  btn.classList.toggle("active", !pressed);
  applyStepView();
}

var searchTimer;
document.getElementById("step-search").addEventListener("input", function () {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(applyStepView, 120);
});

applyStepView();

// Events table
var allEvents = report.events.map(function (e) {
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
    e.attempt +
    "</td>" +
    '<td class="mono">' +
    dur +
    "</td>" +
    "<td" +
    errTip +
    ">" +
    (e.error
      ? '<span class="inline-error" data-error="' + esc(e.error) + '">' + esc(e.error) + "</span>"
      : "") +
    "</td>" +
    "</tr>"
  );
});
document.getElementById("events-tbody").innerHTML = allEvents.join("");
document.getElementById("events-empty").style.display = allEvents.length ? "none" : "";

// Events pagination
var EVT_PAGE_SIZE = 100;
var evtRows = Array.prototype.slice.call(document.querySelectorAll("#events-tbody tr"));
var evtExpanded = false;

if (evtRows.length > EVT_PAGE_SIZE) {
  evtRows.forEach(function (tr, i) {
    if (i >= EVT_PAGE_SIZE) tr.style.display = "none";
  });
  var evtWrap = document.querySelector("#tab-events .table-wrap");
  var evtMoreBar = document.createElement("div");
  evtMoreBar.id = "evt-more-bar";
  evtMoreBar.style.cssText = "text-align:center;padding:0.75rem";
  evtMoreBar.innerHTML =
    '<button class="chip" id="show-more-evt">Showing ' +
    EVT_PAGE_SIZE +
    " of " +
    evtRows.length +
    " \u2014 Show all</button>";
  evtWrap.appendChild(evtMoreBar);
  document.getElementById("show-more-evt").addEventListener("click", function () {
    evtExpanded = true;
    evtRows.forEach(function (tr) {
      tr.style.display = "";
    });
    evtMoreBar.remove();
  });
}

// Event filters
var eventTypes = ["all", "attempt_start", "attempt_end"];
document.getElementById("event-filters").innerHTML = eventTypes
  .map(function (t) {
    return (
      '<button class="chip' +
      (t === "all" ? " active" : "") +
      '" aria-pressed="' +
      (t === "all" ? "true" : "false") +
      '" aria-label="Filter by ' +
      t +
      '" data-filter="' +
      t +
      '">' +
      (t === "all" ? "all" : eventLabels[t] || t) +
      "</button>"
    );
  })
  .join("");
document.querySelectorAll("#event-filters .chip").forEach(function (btn) {
  btn.addEventListener("click", function () {
    document.querySelectorAll("#event-filters .chip").forEach(function (b) {
      b.classList.remove("active");
      b.setAttribute("aria-pressed", "false");
    });
    btn.classList.add("active");
    btn.setAttribute("aria-pressed", "true");
    var f = btn.dataset.filter;
    evtRows.forEach(function (tr, i) {
      var matches = f === "all" || tr.dataset.type === f;
      if (f !== "all") {
        tr.style.display = matches ? "" : "none";
      } else {
        tr.style.display = matches && (evtExpanded || i < EVT_PAGE_SIZE) ? "" : "none";
      }
    });
  });
});

// Step DAG Tree
(function renderTree() {
  var container = document.getElementById("tree-container");
  if (!report.steps || !report.steps.length) {
    container.innerHTML =
      '<div class="panel-empty"><div class="empty-icon">\u{1F5C2}</div><div class="empty-text">No steps recorded</div></div>';
    return;
  }
  var byName = {};
  report.steps.forEach(function (s) {
    byName[s.step_name] = s;
  });
  var roots = report.steps.filter(function (s) {
    return !s.dependencies || !s.dependencies.length;
  });
  if (!roots.length && report.steps.length) roots.push(report.steps[0]);
  var visited = {};

  function renderNode(step, parent) {
    if (visited[step.step_name]) return;
    visited[step.step_name] = true;
    var div = document.createElement("div");
    div.className = "scope-node";
    if (step.status === "failed" || step.status === "canceled") {
      div.className += " has-failure";
    }
    var hdr = document.createElement("div");
    hdr.className = "scope-node-header";
    var icon = statusIcons[step.status] || "";
    var depCount = (step.dependents || []).length;
    hdr.innerHTML =
      '<button class="toggle" aria-expanded="true" aria-label="Collapse">\u25BE</button>' +
      '<span class="scope-name">' +
      icon +
      " " +
      esc(step.step_name) +
      "</span>" +
      '<span class="scope-count">' +
      depCount +
      " dependent" +
      (depCount !== 1 ? "s" : "") +
      " \u00B7 " +
      esc(step.status) +
      (step.duration_ms ? " \u00B7 " + humanizeDuration(step.duration_ms) : "") +
      "</span>";
    hdr.setAttribute("role", "button");
    hdr.setAttribute("tabindex", "0");
    div.appendChild(hdr);

    if (step.error) {
      var errDiv = document.createElement("div");
      errDiv.className = "scope-node-error";
      errDiv.textContent = step.error;
      div.appendChild(errDiv);
    }

    var deps = (step.dependents || [])
      .map(function (d) {
        return byName[d.step_name];
      })
      .filter(function (s) {
        return s && !visited[s.step_name];
      });
    if (deps.length) {
      var childDiv = document.createElement("div");
      childDiv.className = "scope-children";
      deps.forEach(function (c) {
        renderNode(c, childDiv);
      });
      div.appendChild(childDiv);
    }

    function toggleNode() {
      var collapsed = div.classList.toggle("collapsed");
      var btn = hdr.querySelector(".toggle");
      btn.innerHTML = collapsed ? "\u25B8" : "\u25BE";
      btn.setAttribute("aria-expanded", !collapsed);
    }
    hdr.addEventListener("click", toggleNode);
    hdr.addEventListener("keydown", function (e) {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        toggleNode();
      }
    });
    parent.appendChild(div);
  }

  roots.forEach(function (r) {
    renderNode(r, container);
  });
})();

// Gantt-style timeline (time-positioned bars showing actual parallelism)
(function renderGantt() {
  var tlSteps = report.steps.filter(function (s) {
    return s.started_at;
  });
  if (!tlSteps.length) {
    document.getElementById("timeline-container").innerHTML =
      '<div class="panel-empty"><div class="empty-icon">\u23F1</div><div class="empty-text">No timing data recorded</div><div class="empty-hint">Step durations appear here after workflow execution</div></div>';
    return;
  }
  tlSteps.sort(function (a, b) {
    return new Date(a.started_at) - new Date(b.started_at);
  });
  var minT = new Date(tlSteps[0].started_at).getTime();
  var maxT = Math.max.apply(
    null,
    tlSteps.map(function (s) {
      return new Date(s.finished_at || s.started_at).getTime();
    }),
  );
  var range = maxT - minT || 1;

  function pct(ts) {
    return ((ts - minT) / range) * 100;
  }
  function pctStr(ts) {
    return pct(ts).toFixed(2);
  }

  var axisStart = new Date(minT).toLocaleTimeString();
  var axisMid = new Date(minT + range / 2).toLocaleTimeString();
  var axisEnd = new Date(maxT).toLocaleTimeString();

  var rowsHtml = tlSteps
    .map(function (s) {
      var st = new Date(s.started_at).getTime();
      var ft = s.finished_at ? new Date(s.finished_at).getTime() : st;
      var left = pctStr(st);
      var width = Math.max(0.3, pct(ft) - pct(st)).toFixed(2);
      var icon = statusIcons[s.status] || "";
      var tip =
        esc(s.step_name) +
        " | " +
        s.status +
        (s.duration_ms ? " | " + humanizeDuration(s.duration_ms) : "") +
        (s.attempt_count > 1 ? " | " + s.attempt_count + " attempts" : "") +
        (s.error ? " | " + esc(s.error) : "");
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
        "</div>" +
        "</div>"
      );
    })
    .join("");

  document.getElementById("timeline-container").innerHTML =
    '<div class="gantt-axis"><span>' +
    axisStart +
    "</span><span>" +
    axisMid +
    "</span><span>" +
    axisEnd +
    "</span></div>" +
    '<div class="gantt-grid">' +
    rowsHtml +
    "</div>";
})();

// Dependency Graph - rendered by daghtml SDK
function renderGraph() {
  initDAGGraph("graph-container", "dag-data");
}

document.getElementById("footer-ts").textContent = new Date().toLocaleString();
document.getElementById("footer-stats").textContent =
  "Schema v" +
  report.version +
  " \u00b7 " +
  report.event_count +
  " events \u00b7 " +
  report.step_count +
  " steps";
