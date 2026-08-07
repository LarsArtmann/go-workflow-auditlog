# ADR 0001: SSE-Only Transport for Live Dashboard

**Date:** 2026-08-06
**Status:** Accepted (shipped in v0.8.3)
**Deciders:** Lars Artmann

## Context

The live dashboard module (`live/`) originally supported two real-time transports:
Server-Sent Events (SSE) via `/api/events` and WebSocket via `/api/ws`. Both
carried the same `snapshot`/`event`/`complete` message protocol.

The WebSocket transport was added as a fallback for environments where SSE
connections are unreliable (corporate proxies, aggressive load balancers).
However, it introduced significant complexity and a heavyweight dependency
(`gorilla/websocket` v1.5.3) for a path that was rarely used in practice.

## Decision

Remove the WebSocket transport entirely. SSE is the sole real-time transport.

## Rationale

1. **Every browser supports SSE natively** via `EventSource` — no client-side
   library, no framing protocol, no `ws://` vs `http://` URL scheme.
2. **SSE has native reconnection** — `EventSource` automatically reconnects
   and sends `Last-Event-ID`, enabling replay of missed events.
3. **The parallel transport duplicated envelope logic** — `snapshot`/`event`/
   `complete` message handling was written twice, doubling the maintenance
   surface and the test matrix.
4. **WebSocket lacked replay semantics** — it had no equivalent of
   `Last-Event-ID`, so reconnects lost events silently.
5. **The sibling project** (`samber-do-auditlog/live`) has always been SSE-only;
   aligning the two reduces cognitive load across the family.
6. **No concrete customer benefited** from the WebSocket path in practice.

## Consequences

- `gorilla/websocket` dependency removed from the `live` module.
- Single transport code path, single set of message envelopes.
- Consumers who wrote custom WebSocket clients (not the dashboard JS) must
  switch to `EventSource` against `/api/events`.
