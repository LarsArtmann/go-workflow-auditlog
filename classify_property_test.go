package auditlog_test

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	errorfamily "github.com/larsartmann/go-error-family"
	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

// =============================================================================
// P2-15: Property — wrapping preserves Family through arbitrary depth
// =============================================================================

func TestClassifyProperty_WrappingPreservesFamily(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(42, 42))
	classifications := auditlog.ErrorClassifications()

	for range 200 {
		sentinel := allPublicSentinels()[rng.IntN(len(allPublicSentinels()))]
		wantFamily := classifications[sentinel]
		depth := 1 + rng.IntN(10) // 1–10 layers of wrapping

		err := sentinel
		for range depth {
			err = fmt.Errorf("layer %d: %w", depth, err)
		}

		gotFamily := errorfamily.Classify(err)
		if gotFamily != wantFamily {
			t.Errorf("Classify(%d-deep wrapped %v) = %v, want %v", depth, sentinel, gotFamily, wantFamily)
		}
	}
}

// =============================================================================
// P2-17: Property — Classify identity: every registered sentinel matches its map entry
// Verifies the registration mechanism is consistent with the declared mapping.
// =============================================================================

func TestClassifyProperty_IdentityMatchesErrorClassificationsMap(t *testing.T) {
	t.Parallel()

	classifications := auditlog.ErrorClassifications()

	for sentinel, wantFamily := range classifications {
		gotFamily := errorfamily.Classify(sentinel)
		if gotFamily != wantFamily {
			t.Errorf("Classify(%v) = %v, want %v (from ErrorClassifications map)", sentinel, gotFamily, wantFamily)
		}
	}
}

// =============================================================================
// P2-16: Fuzz — Classify on adversarial wrapped error chains
// Ensures Classify never panics and always returns a valid Family for any input.
// =============================================================================

func FuzzClassifyAdversarialChains(f *testing.F) {
	// Seed with known sentinel-wrapping patterns
	seeds := []string{
		"wrapped error",
		"nil",
		"",
		"very long error message " + strings.Repeat("x", 200),
		"special: %s %d %v %w",
		"nested: outer: inner: core",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, payload string) {
		t.Parallel()

		// Direct unregistered error — should classify as Transient (fail-open default)
		bareErr := fmt.Errorf("fuzz: %s", payload)

		family := errorfamily.Classify(bareErr)
		if family != errorfamily.Transient {
			t.Errorf("Classify(unregistered) = %v, want Transient (fail-open default)", family)
		}

		// Wrapped sentinel → family should match sentinel's classification
		sentinels := allPublicSentinels()
		expected := auditlog.ErrorClassifications()

		for _, sentinel := range sentinels {
			wrapped := fmt.Errorf("%s: %w: %s", payload, sentinel, payload)

			gotFamily := errorfamily.Classify(wrapped)
			if gotFamily != expected[sentinel] {
				t.Errorf("Classify(wrapped %v payload=%q) = %v, want %v",
					sentinel, payload, gotFamily, expected[sentinel])
			}
		}
	})
}
