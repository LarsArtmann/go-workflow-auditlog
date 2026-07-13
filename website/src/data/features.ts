import type { Feature } from "./types";

export const features: Feature[] = [
  {
    icon: "chart",
    title: "Per-Attempt Events",
    desc: "Every retry, every duration, every error — captured as timestamped events with attempt numbers and phases.",
  },
  {
    icon: "graph",
    title: "Full DAG Structure",
    desc: "Dependency graph, retry/timeout config, step types, skipped and canceled statuses — all captured automatically.",
  },
  {
    icon: "monitor",
    title: "Interactive Dashboard",
    desc: "Self-contained HTML report with DAG graph engine, sortable tables, Gantt timeline, and collapsible tree — zero dependencies.",
  },
  {
    icon: "download",
    title: "12+ Export Formats",
    desc: "JSON, NDJSON, Mermaid, PlantUML, Graphviz DOT, D2, tables (16 sub-formats), ASCII tree, HTML tree, and interactive HTML.",
  },
  {
    icon: "git-compare",
    title: "Report Diffing",
    desc: "Compare two runs for regression detection — added, removed, and changed steps with duration deltas and status changes.",
  },
  {
    icon: "link",
    title: "Cross-System Correlation",
    desc: "128-bit crypto-random RunID stamped on every event for trace/log correlation across distributed systems.",
  },
];
