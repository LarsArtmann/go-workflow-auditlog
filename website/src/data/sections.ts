import type { StepCard, ComparisonItem, UseCase, ComparisonMatrix } from "./types";

export const steps: StepCard[] = [
  {
    step: "1",
    stepColor: "accent",
    title: "Attach",
    desc: "Inject BeforeStep / AfterStep callbacks into every step via State.MergeConfig. Call before Do.",
    code: "audit.Attach(w)",
  },
  {
    step: "2",
    stepColor: "accent",
    title: "Execute",
    desc: "During w.Do(ctx), callbacks fire per-attempt, recording timestamped events with duration and error.",
    code: "w.Do(ctx)",
  },
  {
    step: "3",
    stepColor: "amber",
    title: "Snapshot",
    desc: "Read post-execution state to capture DAG structure, retry config, and skipped/canceled statuses.",
    code: "audit.Snapshot(w)",
  },
];

export const comparisons: ComparisonItem[] = [
  {
    variant: "No audit",
    accent: false,
    pros: ["Zero overhead", "No dependencies"],
    cons: [
      "No per-attempt visibility",
      "No DAG structure",
      "No retry tracking",
      "No exportable reports",
      "No timeline or dashboard",
    ],
  },
  {
    variant: "Manual logging",
    accent: false,
    pros: ["Full control"],
    cons: [
      "High boilerplate per step",
      "No DAG structure",
      "No retry/timeout config",
      "No diagram export",
      "No interactive dashboard",
      "Easy to forget on new steps",
    ],
  },
  {
    variant: "go-workflow-auditlog",
    accent: true,
    pros: [
      "Automatic per-attempt capture",
      "Full DAG with dependencies",
      "Skipped & canceled detection",
      "12+ export formats",
      "Interactive HTML dashboard",
      "Report diffing & replay",
      "128-bit RunID correlation",
      "3-line integration",
    ],
    cons: [],
  },
];

export const comparisonMatrix: ComparisonMatrix = {
  columns: ["No audit", "Manual logging", "go-workflow-auditlog"],
  rows: [
    { feature: "Per-attempt events", values: ["no", "partial", "yes"] },
    { feature: "DAG structure", values: ["no", "no", "yes"] },
    { feature: "Skipped/canceled detection", values: ["no", "no", "yes"] },
    { feature: "Retry & timeout config", values: ["no", "no", "yes"] },
    { feature: "HTML dashboard", values: ["no", "no", "yes"] },
    { feature: "Diagram export", values: ["no", "no", "yes"] },
    { feature: "Report diffing", values: ["no", "no", "yes"] },
    { feature: "RunID correlation", values: ["no", "no", "yes"] },
    { feature: "Lines of boilerplate", values: ["0", "~50", "3"] },
  ],
};

export const useCases: UseCase[] = [
  {
    title: "CI/CD Pipelines",
    desc: "Audit every step execution, retry, and failure for post-mortem analysis",
    icon: "refresh",
  },
  {
    title: "Data Pipelines",
    desc: "Track transform chain timing for bottleneck detection and regression alerts",
    icon: "chart",
  },
  {
    title: "Microservice Orchestration",
    desc: "Correlate workflow runs with distributed traces via 128-bit RunID",
    icon: "bolt",
  },
  {
    title: "Compliance & Audit Trails",
    desc: "Immutable event stream for regulated workflows and SOC2/HIPAA evidence",
    icon: "shield",
  },
];
