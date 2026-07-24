# Benchmarks

Baseline benchmark results for `go-workflow-auditlog`, captured post-v0.7.0.

These serve as a regression detection baseline. Re-run with:

> **Requires `GOEXPERIMENT=jsonv2`** — set in the Nix devShell automatically, or `export GOEXPERIMENT=jsonv2` manually.

```bash
# Core module
GOEXPERIMENT=jsonv2 go test -bench=. -benchmem -count=3 -run=^$ ./...

# Viz module
cd viz && GOEXPERIMENT=jsonv2 go test -bench=. -benchmem -count=3 -run=^$ ./...
```

Compare against this file with `benchstat`:

```bash
go test -bench=. -benchmem -count=5 -run=^$ ./... > /tmp/new.txt
benchstat /tmp/new.txt  # compare manually against the table below
```

---

## Environment

| Property           | Value                                      |
| ------------------ | ------------------------------------------ |
| Date               | 2026-07-24                                 |
| Go                 | 1.26.5                                     |
| OS                 | Linux (NixOS)                              |
| CPU                | AMD Ryzen AI MAX+ 395 (32 threads)         |
| Runs per benchmark | 1 (median of 3 recommended for comparison) |

---

## Results

### Core Module — Streaming

| Benchmark                             | Time/op      | Bytes/op    | Allocs/op | Notes                              |
| ------------------------------------- | ------------ | ----------- | --------- | ---------------------------------- |
| `BenchmarkNDJSONStreamer_100Events`   | 70,066 ns    | 106,315 B   | 218       | Stream 100 events to io.Discard    |
| `BenchmarkNDJSONStreamer_1000Events`  | 713,548 ns   | 394,320 B   | 2,018     | Stream 1,000 events to io.Discard  |
| `BenchmarkNDJSONStreamer_10000Events` | 4,479,693 ns | 3,274,326 B | 20,018    | Stream 10,000 events to io.Discard |

### Viz Module — Hot Path

| Benchmark                      | Time/op    | Bytes/op  | Allocs/op | Notes                                               |
| ------------------------------ | ---------- | --------- | --------- | --------------------------------------------------- |
| `BenchmarkInvocation/disabled` | 3,082 ns   | 1,040 B   | 16        | Zero-cost disabled path (go-workflow overhead only) |
| `BenchmarkInvocation/enabled`  | 7,202 ns   | 3,246 B   | 28        | Single step invoke with before+after hooks          |
| `BenchmarkAttach/10-steps`     | 3,707 ns   | 1,376 B   | 70        | Attach callbacks to 10-step workflow                |
| `BenchmarkAttach/50-steps`     | 40,826 ns  | 22,570 B  | 1,332     | Attach callbacks to 50-step workflow                |
| `BenchmarkAttach/100-steps`    | 160,527 ns | 85,274 B  | 5,158     | Attach callbacks to 100-step workflow               |
| `BenchmarkBuildReport/50`      | 35,776 ns  | 62,344 B  | 130       | BuildReport with 50 steps                           |
| `BenchmarkBuildReport/100`     | 71,789 ns  | 126,248 B | 236       | BuildReport with 100 steps                          |
| `BenchmarkBuildReport/500`     | 446,028 ns | 763,466 B | 1,054     | BuildReport with 500 steps                          |
| `BenchmarkEventsCopy`          | 6,285 ns   | 32,768 B  | 1         | Defensive copy of all events (single allocation)    |
| `BenchmarkOnEventCallback`     | 6,240 ns   | 2,980 B   | 23        | OnEvent callback overhead per event                 |
| `BenchmarkRetryWithAudit`      | 1.3 s      | 3,560 B   | 73        | Full retry cycle with backoff (dominated by sleep)  |

### Viz Module — Export Rendering (100-step reports)

| Benchmark                         | Time/op      | Bytes/op    | Allocs/op | Notes                           |
| --------------------------------- | ------------ | ----------- | --------- | ------------------------------- |
| `BenchmarkWriteJSON_LargeReport`  | 86,122 ns    | 9,738 B     | 25        | JSON export (100 steps)         |
| `BenchmarkWriteMermaid_Large`     | 113,296 ns   | 135,080 B   | 2,138     | Mermaid diagram (100 steps)     |
| `BenchmarkWriteD2_LargeReport`    | 155,376 ns   | 322,699 B   | 1,561     | D2 diagram (100 steps)          |
| `BenchmarkWriteTree_LargeReport`  | 119,621 ns   | 183,836 B   | 827       | ASCII tree (100 steps)          |
| `BenchmarkWriteTable_LargeReport` | 36,204 ns    | 63,149 B    | 239       | Table export (100 steps)        |
| `BenchmarkRenderHTML_LargeReport` | 2,173,328 ns | 4,617,301 B | 11,083    | Full HTML dashboard (100 steps) |

### Viz Module — Export Rendering (3-step reports)

| Benchmark                         | Time/op    | Bytes/op  | Allocs/op | Notes                         |
| --------------------------------- | ---------- | --------- | --------- | ----------------------------- |
| `BenchmarkWriteJSON_SmallReport`  | 5,177 ns   | 3,592 B   | 23        | JSON export (3 steps)         |
| `BenchmarkRenderHTML_SmallReport` | 108,210 ns | 420,178 B | 81        | Full HTML dashboard (3 steps) |

### Viz Module — Mermaid Export Scaling

| Benchmark                    | Time/op    | Bytes/op  | Allocs/op | Notes               |
| ---------------------------- | ---------- | --------- | --------- | ------------------- |
| `BenchmarkMermaidExport/10`  | 10,842 ns  | 13,456 B  | 230       | Mermaid (10 steps)  |
| `BenchmarkMermaidExport/50`  | 51,822 ns  | 67,005 B  | 1,082     | Mermaid (50 steps)  |
| `BenchmarkMermaidExport/100` | 106,599 ns | 135,083 B | 2,138     | Mermaid (100 steps) |

---

## Key Observations

- **Disabled path is low-overhead**: ~3 us / 16 allocs — entirely go-workflow's own overhead. The auditlog plugin adds nothing when `Enabled: false`.
- **Invocation hot path is lean**: ~7 us / 28 allocs for a full before+after invocation hook pair.
- **BuildReport scales linearly**: 50→500 steps is ~12x time, confirming O(n) complexity.
- **EventsCopy is a single allocation**: the `append([]Event(nil), r.events...)` pattern allocates exactly once for the backing array.
- **HTML rendering is the heaviest export**: ~2 ms for 100 steps due to daghtml SVG generation + CSS/JS embedding. Still well under interactive latency.
- **RetryWithAudit is dominated by backoff sleep**: 1.3 s wall time with only 73 allocs — the audit overhead is negligible compared to the retry delay.
- **NDJSON streaming scales linearly**: 100→10,000 events is ~64x time and ~31x memory, confirming O(n) with constant per-event cost (~440 ns/event).
