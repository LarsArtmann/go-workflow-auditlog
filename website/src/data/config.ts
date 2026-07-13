export const siteConfig = {
  name: "go-workflow-auditlog",
  title: "go-workflow-auditlog — Audit Logging for Azure/go-workflow",
  description:
    "Records every step execution event — attempts, retries, durations, errors, dependencies — with timestamped events and export to JSON, NDJSON, diagrams, and an interactive HTML dashboard.",
  siteUrl: "https://auditlog.lars.software",
  github: "https://github.com/larsartmann/go-workflow-auditlog",
  author: {
    name: "LarsArtmann",
    url: "https://larsartmann.com/",
  },
  pkgGoDev: "https://pkg.go.dev/github.com/larsartmann/go-workflow-auditlog",
} as const;
