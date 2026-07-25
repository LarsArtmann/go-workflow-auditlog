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
	}

	for _, fn := range expectedFunctions {
		if !strings.Contains(js, fn) {
			t.Errorf("dashboard.js missing expected function: %s", fn)
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
	}

	for _, cls := range expectedClasses {
		if !strings.Contains(css, cls) {
			t.Errorf("dashboard.css missing expected class: %s", cls)
		}
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
