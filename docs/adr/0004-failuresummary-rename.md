# ADR 0004: FailureSummary Rename

**Date:** 2026-08-06
**Status:** Accepted (shipped in v0.8.x)
**Deciders:** Lars Artmann

## Context

`WorkflowReport` originally had a field `FailureReason string` (JSON:
`"failure_reason"`) containing a human-readable summary like
`"3 step(s) failed: fetch, transform"`. When the structured `FailureReason`
enum (`timeout`/`canceled`/`user_error`) was added to `Event` (also JSON:
`"failure_reason"`), the two fields collided on the same JSON key name with
different semantic meanings:

- Report level: free-form sentence (`"3 step(s) failed: ..."`)
- Event level: typed enum (`"timeout"`)

Consumers parsing both events and reports from JSON could not distinguish
the two without checking the surrounding object context.

## Decision

Rename the report-level field from `FailureReason` to `FailureSummary`
(JSON: `"failure_summary"`). The event-level `FailureReason` enum keeps the
`"failure_reason"` JSON key.

## Rationale

1. **The names now match their semantics.** `FailureSummary` is a
   human-readable summary; `FailureReason` is a structured enum. No
   ambiguity.

2. **Each JSON key has exactly one meaning.** `"failure_summary"` always
   means "free-form sentence at report level"; `"failure_reason"` always
   means "typed enum value."

3. **The event-level key is the one consumers filter on** (e.g., "show me
   all timeout events"). Giving it the canonical `"failure_reason"` name
   makes this the primary axis, while `"failure_summary"` is the
   human-readable aggregate that appears once per report.

4. **Backward compatibility is manageable.** The field is `omitempty`, so
   successful reports don't carry either key. Consumers parsing failed
   reports need a one-line JSON tag rename.

## Consequences

- Breaking JSON change for consumers who parse `WorkflowReport.FailureReason`
  — they must rename to `FailureSummary` (documented in MIGRATION.md).
- The Go API field access changes from `report.FailureReason` to
  `report.FailureSummary`.
- No collision is possible going forward: each key name is owned by exactly
  one type with one semantic contract.
