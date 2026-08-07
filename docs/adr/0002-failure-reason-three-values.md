# ADR 0002: FailureReason Enum with Three Values

**Date:** 2026-08-06
**Status:** Accepted (shipped in v0.8.x)
**Deciders:** Lars Artmann

## Context

The audit log captures errors on every `attempt_end` event, but the free-form
`Error` string is insufficient for programmatic filtering, routing, and
alerting. Consumers need a structured category to answer: "did this step time
out?" or "was it canceled?" without parsing error text.

The full set of go-workflow failure modes includes: timeout, cancellation,
user error, panic, and dependency-driven skip. A naive enum might include all
five.

## Decision

Define `FailureReason` with exactly three values: `timeout`, `canceled`, and
`user_error`. Do not include `panic` or `dependency_failed`.

## Rationale

1. **Panics are not observable at the `AfterStep` callback level.** A panicking
   step crashes the workflow goroutine; go-workflow does not recover it into
   an error that reaches `AfterStep`. The callback simply never fires. There
   is no error to classify — adding `panic` to the enum would be a value that
   can never be set through the normal capture path.

2. **Dependency-driven skips bypass callbacks entirely.** When a step is
   skipped because an upstream dependency failed, go-workflow settles its
   status inline via Conditions — the step never enters the `BeforeStep`/
   `AfterStep` chain. The `attempt_end` event is never emitted. The skip
   status IS captured (by `Snapshot` reading `flow.StateOf`), but there is
   no event to attach a `FailureReason` to.

3. **The three chosen values cover every error the recorder CAN observe:**
   - `timeout` — `errors.Is(err, context.DeadlineExceeded)`
   - `canceled` — `errors.Is(err, context.Canceled)`
   - `user_error` — any other non-nil error from `Do()`

4. **A zero value (empty string) means "no failure classified"** — typically
   because the attempt succeeded. This is cleaner than a `none` or `ok` enum
   value because `omitempty` naturally suppresses it in JSON for successful
   steps.

## Consequences

- `classifyFailure(err error) FailureReason` inspects the error chain with
  `errors.Is` and returns one of the three values (or empty for nil).
- The enum is extensible — new values can be added without breaking consumers
  (all `FailureReason` comparisons should use the typed constants, not string
  literals).
- Consumers who need to detect panics or dependency skips must check
  `StepInfo.Status` (which IS set for those cases via `Snapshot`), not
  `FailureReason`.
