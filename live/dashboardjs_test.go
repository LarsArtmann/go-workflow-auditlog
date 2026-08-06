package live_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDashboardJS_StructuralIntegrity validates that the embedded dashboard.js
// is well-formed and contains all expected function definitions. This catches
// accidental deletion, broken embedding, and missing features.
func TestDashboardJS_StructuralIntegrity(t *testing.T) {
	t.Parallel()

	jsBytes, err := os.ReadFile("dashboard.js")
	if err != nil {
		t.Fatalf("read dashboard.js: %v", err)
	}

	js := string(jsBytes)

	// Verify expected function definitions exist
	expectedFunctions := []string{
		"function esc(",
		"function humanizeDuration(",
		"function humanizeMs(",
		"function connect()",
		"function connectSSE()",
		"function connectWebSocket()",
		"function handleSnapshot(",
		"function handleEvent(",
		"function handleComplete(",
		"function processEventIntoSteps(",
		"function scheduleRender()",
		"function scheduleFullRender()",
		"function renderAll()",
		"function renderHeader()",
		"function renderStats()",
		"function renderWaveform()",
		"function renderLegend()",
		"function renderFailureBanner()",
		"function getSortedSteps()",
		"function buildStepCellsHTML(",
		"function stepStateKey(",
		"function updateStepRow(",
		"function renderStepsTable()",
		"function renderEventsTable()",
		"function renderGraph()",
		"function enhanceGraph()",
		"function computeCriticalPathSteps()",
		"function buildNodeNameMap()",
		"function toggleCriticalPathHighlight(",
		"function applyGraphSearch(",
		"function renderMinimap()",
		"function updateGraphLive()",
		"function renderTimeline()",
		"function renderFooter()",
		"function switchTab(",
		"function handleKeyboardShortcut(",
		"function openHelp()",
		"function closeHelp()",
		"function helpIsOpen()",
		"function focusHelpTrap(",
		"function isInputTarget(",
		"function buildGraphAdjacency()",
		"function handleGraphNodeKeydown(",
		"function selectGraphNode(",
		"function handleStepRowKeydown(",
		"function refreshStepRowTabIndexes()",
		"function updateSortableHeaders()",
		"function activateSortHeader(",
		"function focusStepRow(",
		"function showTooltipForRow(",
		"function focusGraphNodeLabel(",
		"function navigateGraphNode(",
	}

	for _, fn := range expectedFunctions {
		if !strings.Contains(js, fn) {
			t.Errorf("dashboard.js missing expected function: %s", fn)
		}
	}
}

// TestDashboardJS_NoDeadCode verifies that previously-removed dead code
// functions are not present. These were identified during self-audit and
// removed: focusTabPanel, getGraphSvgNodes.
func TestDashboardJS_NoDeadCode(t *testing.T) {
	t.Parallel()

	jsBytes, err := os.ReadFile("dashboard.js")
	if err != nil {
		t.Fatalf("read dashboard.js: %v", err)
	}

	js := string(jsBytes)

	deadFunctions := []string{
		"function focusTabPanel(",
		"function getGraphSvgNodes(",
		// Non-existent zoom functions that were guarded by typeof checks
		"typeof fitDAGGraph",
		"typeof zoomInDAGGraph",
		"typeof zoomOutDAGGraph",
		// Duplicate Tab condition in focusHelpTrap
		`e.key === "Tab" || e.key === "Tab"`,
	}

	for _, dead := range deadFunctions {
		if strings.Contains(js, dead) {
			t.Errorf("dashboard.js should not contain dead/buggy code: %s", dead)
		}
	}
}

// TestDashboardJS_GraphZoomDelegation verifies that zoom/fit keyboard
// shortcuts delegate to daghtml's own buttons via .click() rather than
// calling non-existent global functions.
func TestDashboardJS_GraphZoomDelegation(t *testing.T) {
	t.Parallel()

	jsBytes, err := os.ReadFile("dashboard.js")
	if err != nil {
		t.Fatalf("read dashboard.js: %v", err)
	}

	js := string(jsBytes)

	// The keyboard handler must delegate to the existing buttons
	requiredPatterns := []string{
		"els.graphFit.click()",
		"els.graphZoomIn.click()",
		"els.graphZoomOut.click()",
	}

	for _, pat := range requiredPatterns {
		if !strings.Contains(js, pat) {
			t.Errorf("dashboard.js missing zoom delegation: %s", pat)
		}
	}
}

// TestDashboardJS_BalancedBraces verifies basic syntactic validity by checking
// that the file has a reasonable ratio of opening to closing braces.
// A perfect count is impossible due to regex literals containing quote chars,
// but a large imbalance indicates a real problem.
func TestDashboardJS_BalancedBraces(t *testing.T) {
	t.Parallel()

	jsBytes, err := os.ReadFile("dashboard.js")
	if err != nil {
		t.Fatalf("read dashboard.js: %v", err)
	}

	js := string(jsBytes)

	openBraces := strings.Count(js, "{")
	closeBraces := strings.Count(js, "}")

	if openBraces < closeBraces {
		t.Errorf("more closing braces (%d) than opening (%d)", closeBraces, openBraces)
	}

	// Allow small imbalance from regex literals; flag large gaps
	diff := openBraces - closeBraces
	if diff > 10 {
		t.Errorf("brace imbalance too large: %d open vs %d close (diff=%d)", openBraces, closeBraces, diff)
	}
}

// TestDashboardJS_NoUndefinedGlobals verifies that commonly-used DOM elements
// are referenced via the `els` object (no bare global access that would fail
// if the element is missing).
func TestDashboardJS_UsesEventSource(t *testing.T) {
	t.Parallel()

	jsBytes, err := os.ReadFile("dashboard.js")
	if err != nil {
		t.Fatalf("read dashboard.js: %v", err)
	}

	js := string(jsBytes)

	// SSE connection must use EventSource
	if !strings.Contains(js, "new EventSource") {
		t.Error("dashboard.js should create EventSource for SSE connection")
	}

	// Must handle all three SSE event types
	for _, evt := range []string{"snapshot", "event", "complete"} {
		searchStr := `addEventListener("` + evt + `"`
		if !strings.Contains(js, searchStr) {
			t.Errorf("dashboard.js missing SSE event listener for %q", evt)
		}
	}

	// Must have reconnection logic
	if !strings.Contains(js, "reconnect") {
		t.Error("dashboard.js missing reconnection logic")
	}

	// Must show a visible "reconnecting" status indicator on SSE error
	if !strings.Contains(js, `"reconnecting"`) {
		t.Error("dashboard.js missing 'reconnecting' status indicator")
	}
}

// TestDashboardCSS_StructuralIntegrity validates key CSS classes exist.
func TestDashboardCSS_StructuralIntegrity(t *testing.T) {
	t.Parallel()

	cssBytes, err := os.ReadFile("dashboard.css")
	if err != nil {
		t.Fatalf("read dashboard.css: %v", err)
	}

	css := string(cssBytes)

	expectedClasses := []string{
		".live-badge",
		".conn-status",
		".step-row-running",
		".step-row-new",
		".graph-minimap",
		".node-flash",
		".critical-path",
		".search-dimmed",
		".retry-badge",
		".skip-link",
		".help-modal",
		".help-hint",
		".sort-asc",
		".sort-desc",
		"#tab-steps th.sortable:focus-visible",
		"#tab-steps tr:focus-visible",
		".graph-node:focus",
	}

	for _, cls := range expectedClasses {
		if !strings.Contains(css, cls) {
			t.Errorf("dashboard.css missing expected class: %s", cls)
		}
	}
}

// TestDashboardJS_WebSocketFallback validates the SSE→WebSocket fallback
// logic exists and the WebSocket message handling is wired correctly.
func TestDashboardJS_WebSocketFallback(t *testing.T) {
	t.Parallel()

	jsBytes, err := os.ReadFile("dashboard.js")
	if err != nil {
		t.Fatalf("read dashboard.js: %v", err)
	}

	js := string(jsBytes)

	// Must create WebSocket connection in fallback
	if !strings.Contains(js, "new WebSocket") {
		t.Error("dashboard.js should create WebSocket for fallback transport")
	}

	// Must have SSE fail counter for fallback trigger
	if !strings.Contains(js, "sseFailCount") {
		t.Error("dashboard.js missing sseFailCount for fallback detection")
	}

	// Must handle all three WebSocket message types
	for _, msgType := range []string{`case "snapshot"`, `case "event"`, `case "complete"`} {
		if !strings.Contains(js, msgType) {
			t.Errorf("dashboard.js missing WebSocket message handler for %s", msgType)
		}
	}

	// Must build ws:// or wss:// URL
	if !strings.Contains(js, "ws:") || !strings.Contains(js, "wss:") {
		t.Error("dashboard.js missing WebSocket URL scheme construction")
	}
}

// TestDashboardJS_DiffBasedRendering validates the incremental DOM update
// infrastructure for the steps table (no full innerHTML rebuild).
func TestDashboardJS_DiffBasedRendering(t *testing.T) {
	t.Parallel()

	jsBytes, err := os.ReadFile("dashboard.js")
	if err != nil {
		t.Fatalf("read dashboard.js: %v", err)
	}

	js := string(jsBytes)

	// Must track rendered rows by step name
	if !strings.Contains(js, "stepRows") {
		t.Error("dashboard.js missing stepRows tracking for diff-based rendering")
	}

	// Must have state key for change detection
	if !strings.Contains(js, "stepStateKey") {
		t.Error("dashboard.js missing stepStateKey for change detection")
	}

	// Must use DOM positioning (insertBefore/after/prepend) not innerHTML rebuild
	if !strings.Contains(js, "prevTr.after") {
		t.Error("dashboard.js should use DOM positioning for incremental updates")
	}

	// Must NOT use full innerHTML rebuild on steps table
	if strings.Contains(js, "stepsTbody.innerHTML = visible.join") {
		t.Error("dashboard.js should not rebuild steps table via innerHTML (use diff-based updates)")
	}
}

// TestDashboardJS_GraphEnhancements validates graph interaction features.
func TestDashboardJS_GraphEnhancements(t *testing.T) {
	t.Parallel()

	jsBytes, err := os.ReadFile("dashboard.js")
	if err != nil {
		t.Fatalf("read dashboard.js: %v", err)
	}

	js := string(jsBytes)

	// Must NOT have broken direction toggle (daghtml has no direction API)
	if strings.Contains(js, "graphDirection") {
		t.Error("dashboard.js should not have graphDirection (daghtml has no direction support)")
	}

	// Must NOT have broken zoomGraph/fitGraphToView (daghtml handles natively)
	if strings.Contains(js, "function zoomGraph") {
		t.Error("dashboard.js should not have zoomGraph (daghtml handles zoom natively)")
	}

	if strings.Contains(js, "function fitGraphToView") {
		t.Error("dashboard.js should not have fitGraphToView (daghtml handles fit natively)")
	}

	// Must have MutationObserver for minimap viewport tracking
	if !strings.Contains(js, "MutationObserver") {
		t.Error("dashboard.js missing MutationObserver for minimap viewport tracking")
	}

	// Must have duration label updates in live graph
	if !strings.Contains(js, "humanizeMs") {
		t.Error("dashboard.js missing humanizeMs for live duration labels")
	}
}

// TestVizDashboardJS_StructuralIntegrity validates the viz dashboard.js.
func TestVizDashboardJS_StructuralIntegrity(t *testing.T) {
	t.Parallel()

	vizPath := filepath.Join("..", "viz", "dashboard.js")

	jsBytes, err := os.ReadFile(vizPath)
	if err != nil {
		t.Fatalf("read viz dashboard.js: %v", err)
	}

	js := string(jsBytes)

	expectedFunctions := []string{
		"function esc(",
		"function humanizeDuration(",
		"function switchTab(",
		"function applyStepView()",
		"function toggleErrorsOnly()",
		"function renderGraph()",
		"function enhanceGraph()",
		"function computeCriticalPathSteps()",
		"function buildNodeNameMap()",
		"function toggleCriticalPathHighlight(",
		"function applyGraphSearch(",
		"function configBadges(",
		"function showTooltip(",
	}

	for _, fn := range expectedFunctions {
		if !strings.Contains(js, fn) {
			t.Errorf("viz dashboard.js missing expected function: %s", fn)
		}
	}
}
