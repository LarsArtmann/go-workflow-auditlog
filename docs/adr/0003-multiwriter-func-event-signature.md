# ADR 0003: MultiWriter func(Event) Signature

**Date:** 2026-08-06
**Status:** Accepted (shipped in v0.8.x)
**Deciders:** Lars Artmann

## Context

`Config.OnEvent` is `func(Event)` — a single callback per auditor. When a
workflow needs to drive multiple consumers (NDJSON file + live SSE hub +
metrics collector), the caller must compose them. Two approaches were
considered:

1. **Channel-based**: `MultiWriter` sends events to a channel; a consumer
   goroutine reads and fans out.
2. **Function-based**: `MultiWriter.OnEvent` matches `func(Event)` exactly
   and invokes each registered callback synchronously in registration order.

## Decision

Use the function-based approach: `MultiWriterCallback = func(Event)`, and
`MultiWriter.OnEvent` calls each callback synchronously.

## Rationale

1. **Direct wiring, zero adapter lambdas.** Because `MultiWriterCallback`
   matches `Config.OnEvent` and `NDJSONStreamer.OnEvent` exactly, sinks
   compose without wrapper functions:

   ```go
   mw := NewMultiWriter(streamer.OnEvent, hub.OnEvent)
   auditor, _ := New(Config{OnEvent: mw.OnEvent})
   ```

2. **Synchronous invocation preserves ordering guarantees.** Each callback
   sees events in the same order the recorder emitted them (within a single
   `OnEvent` call). A channel-based approach would introduce nondeterministic
   ordering unless the channel is unbuffered (which serializes at channel
   cost anyway).

3. **Mutex-serialized fan-out prevents concurrent interleaving.** The
   internal mutex ensures callbacks never see half-written events from
   another goroutine — matching `NDJSONStreamer.OnEvent`'s own guarantee.

4. **Callbacks that need async processing can spawn their own goroutine.**
   The synchronous contract is the safe default; async is opt-in per callback.

## Consequences

- No `Add`/`Remove` API — callbacks are fixed at construction time. Callers
  who need dynamic membership should swap the `MultiWriter` via an atomic
  pointer (out of scope for the common case).
- `NewMultiWriter()` with zero callbacks returns nil (no-op sink). Nil
  `MultiWriter` methods are safe to call.
- `CallbackCount()` is provided for debugging and tests.
