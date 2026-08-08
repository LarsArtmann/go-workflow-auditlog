package viz

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// tokenPattern extracts --name: value; pairs from CSS.
var tokenPattern = regexp.MustCompile(`(--[a-z-]+)\s*:\s*([^;]+);`)

func parseCSSTokens(t *testing.T, css string) map[string]string {
	t.Helper()

	matches := tokenPattern.FindAllStringSubmatch(css, -1)
	result := make(map[string]string, len(matches))

	for _, m := range matches {
		name := strings.TrimSpace(m[1])
		value := strings.TrimSpace(m[2])
		result[name] = value
	}

	return result
}

// TestDesignTokensInSync verifies that the :root block in dashboard.css
// matches DesignTokensCSS exactly. This prevents visual drift between the
// static and live dashboards.
func TestDesignTokensInSync(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("dashboard.css")
	if err != nil {
		t.Fatalf("read dashboard.css: %v", err)
	}

	cssTokens := parseCSSTokens(t, string(source))
	canonicalTokens := parseCSSTokens(t, DesignTokensCSS)

	for name, expected := range canonicalTokens {
		actual, ok := cssTokens[name]
		if !ok {
			t.Errorf("dashboard.css is missing design token %s — add it or update DesignTokensCSS", name)

			continue
		}

		if actual != expected {
			t.Errorf("design token %s drifted:\n  DesignTokensCSS: %s\n  dashboard.css:    %s",
				name, expected, actual)
		}
	}

	var extra []string

	for name := range cssTokens {
		if _, ok := canonicalTokens[name]; !ok {
			extra = append(extra, name)
		}
	}

	if len(extra) > 0 {
		sort.Strings(extra)
		t.Logf("dashboard.css has tokens not in DesignTokensCSS (OK if non-palette): %v", extra)
	}
}
