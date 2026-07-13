import { siteConfig } from "./config";

const importPath = siteConfig.github.replace("https://github.com/", "github.com/");

export const heroCode = `package main

import (
    "context"

    flow "github.com/Azure/go-workflow"

    auditlog "${importPath}"
)

func main() {
    audit, _ := auditlog.New(auditlog.Config{
        Enabled:    true,
        WorkflowID: "data-pipeline",
    })

    w := &flow.Workflow{}
    // add your steps...

    audit.Attach(w)                      // 1. Inject callbacks
    _ = w.Do(context.Background())       // 2. Run workflow
    audit.Snapshot(w)                    // 3. Capture final state

    _ = audit.ExportHTML("report.html")  // Interactive dashboard
}`;
